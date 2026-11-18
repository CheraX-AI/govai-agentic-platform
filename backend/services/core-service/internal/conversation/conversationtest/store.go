// Package conversationtest is an in-memory store for tests that follows the PostgreSQL rules.
package conversationtest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation"
)

// Message is one stored row; Sealed is the encrypted content.
type Message struct {
	ID, ConversationID, Role, Status, ReplyTo string
	Sealed                                    []byte
	updated                                   time.Time
}

// Store fakes both stores; set Err (every call) or one step's error before use.
type Store struct {
	Err, StartErr, CompleteErr, FailErr error

	mu       sync.Mutex
	owners   map[string]string
	messages []Message // insert order is the ordinal
	creates  int
	offset   time.Duration
}

var (
	_ conversation.Store        = (*Store)(nil)
	_ conversation.MessageStore = (*Store)(nil)

	errState = errors.New("synthetic store: invalid state")
)

func New() *Store { return &Store{owners: map[string]string{}} }

func (s *Store) Create(_ context.Context, id, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}

	if _, exists := s.owners[id]; exists {
		return errState
	}

	s.creates++
	s.owners[id] = owner
	return nil
}

func (s *Store) Owns(_ context.Context, id, owner string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return false, s.Err
	}

	return s.owners[id] == owner, nil
}

func (s *Store) StartTurn(
	_ context.Context,
	owner, conversationID string,
	stale time.Duration,
	decide func(last *conversation.LastTurn) (conversation.Start, error),
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(s.StartErr); err != nil {
		return err
	}

	if s.owners[conversationID] != owner {
		return conversation.ErrNotFound
	}

	// Work on a copy and keep it only on success, like a transaction
	rows := append([]Message(nil), s.messages...)
	now := s.now()
	last := -1
	for i := range rows {
		m := &rows[i]
		if m.ConversationID != conversationID || m.Role != conversation.RoleCitizen {
			continue
		}

		if m.Status == conversation.StatusPending && now.Sub(m.updated) > stale {
			m.Status, m.updated = conversation.StatusFailed, now
		}

		last = i
	}

	var found *conversation.LastTurn
	if last >= 0 {
		q := rows[last]
		found = &conversation.LastTurn{QuestionID: q.ID, Question: q.Sealed, Status: q.Status, Age: now.Sub(q.updated)}
		for _, a := range rows {
			if a.ReplyTo == q.ID {
				found.AnswerID, found.Answer = a.ID, a.Sealed
			}
		}
	}

	start, err := decide(found)
	if err != nil {
		return err
	}

	switch {
	case start.Reopen:
		if found == nil || rows[last].Status != conversation.StatusFailed {
			return errState
		}

		rows[last].Status, rows[last].updated = conversation.StatusPending, now
	case start.NewQuestionID != "":
		for _, m := range rows {
			if m.ID == start.NewQuestionID {
				return errState
			}
		}

		rows = append(rows, Message{
			ID: start.NewQuestionID, ConversationID: conversationID, Role: conversation.RoleCitizen,
			Status: conversation.StatusPending, Sealed: start.NewQuestion, updated: now,
		})
	}

	s.messages = rows
	return nil
}

func (s *Store) CompleteTurn(_ context.Context, owner, conversationID, questionID, answerID string, answer []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(s.CompleteErr); err != nil {
		return err
	}

	q := s.pending(owner, conversationID, questionID)
	if q == nil {
		return errState
	}

	for _, m := range s.messages {
		if m.ID == answerID || m.ReplyTo == questionID {
			return errState
		}
	}

	q.Status, q.updated = conversation.StatusCompleted, s.now()
	s.messages = append(s.messages, Message{
		ID: answerID, ConversationID: conversationID, Role: conversation.RoleAssistant,
		Status: conversation.StatusCompleted, ReplyTo: questionID, Sealed: answer, updated: s.now(),
	})
	return nil
}

func (s *Store) FailTurn(_ context.Context, owner, conversationID, questionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(s.FailErr); err != nil {
		return err
	}

	q := s.pending(owner, conversationID, questionID)
	if q == nil {
		return errState
	}

	q.Status, q.updated = conversation.StatusFailed, s.now()
	return nil
}

func (s *Store) RecentTurns(_ context.Context, owner, conversationID string, limit int) ([]conversation.SealedTurn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}

	if s.owners[conversationID] != owner {
		return nil, conversation.ErrNotFound
	}

	var turns []conversation.SealedTurn
	for i := len(s.messages) - 1; i >= 0 && len(turns) < limit; i-- {
		q := s.messages[i]
		if q.ConversationID != conversationID || q.Role != conversation.RoleCitizen || q.Status != conversation.StatusCompleted {
			continue
		}

		for _, a := range s.messages {
			if a.ReplyTo == q.ID {
				turns = append(turns, conversation.SealedTurn{QuestionID: q.ID, Question: q.Sealed, AnswerID: a.ID, Answer: a.Sealed})
			}
		}
	}

	return turns, nil
}

// Messages returns a copy of one conversation's rows, oldest first.
func (s *Store) Messages(conversationID string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	var rows []Message
	for _, m := range s.messages {
		if m.ConversationID == conversationID {
			rows = append(rows, m)
		}
	}

	return rows
}

// Owner returns the conversation's owner, or "".
func (s *Store) Owner(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owners[id]
}

// Creates counts the conversations created.
func (s *Store) Creates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creates
}

// Advance moves the store's clock forward.
func (s *Store) Advance(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offset += d
}

func (s *Store) now() time.Time { return time.Now().Add(s.offset) }

func (s *Store) fail(step error) error {
	if s.Err != nil {
		return s.Err
	}

	return step
}

func (s *Store) pending(owner, conversationID, questionID string) *Message {
	if s.owners[conversationID] != owner {
		return nil
	}

	for i := range s.messages {
		m := &s.messages[i]
		if m.ID == questionID && m.ConversationID == conversationID &&
			m.Role == conversation.RoleCitizen && m.Status == conversation.StatusPending {
			return m
		}
	}

	return nil
}
