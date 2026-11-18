package conversation

import (
	"context"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
)

// Roles and statuses as stored in core.conversation_messages.
const (
	RoleCitizen   = "citizen"
	RoleAssistant = "assistant"

	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// replayWindow is how long a repeated question gets the stored answer.
const replayWindow = 10 * time.Minute

// MaxRecentTurns caps one Recent read.
const MaxRecentTurns = 50

// ErrInProgress means the same question is still being answered.
var ErrInProgress = errors.New("message is still being answered")

// LastTurn is the newest citizen message and its answer, still sealed.
type LastTurn struct {
	QuestionID string
	Question   []byte
	Status     string
	Age        time.Duration // since the last status change
	AnswerID   string        // "" when not answered
	Answer     []byte
}

// SealedTurn is one answered question as stored.
type SealedTurn struct {
	QuestionID string
	Question   []byte
	AnswerID   string
	Answer     []byte
}

// Message is one decrypted message of the recent window.
type Message struct {
	Role string // RoleCitizen or RoleAssistant
	Text string
}

// Start tells StartTurn to add a new question, reopen the last one, or (zero value) do nothing.
type Start struct {
	NewQuestionID string
	NewQuestion   []byte
	Reopen        bool
}

// MessageStore keeps sealed messages; every method checks the owner (ErrNotFound).
type MessageStore interface {
	// StartTurn locks the conversation, fails stale pending questions and applies decide in one transaction.
	StartTurn(ctx context.Context, owner, conversationID string, stale time.Duration,
		decide func(last *LastTurn) (Start, error)) error

	// CompleteTurn saves the answer and completes the question in one transaction.
	CompleteTurn(ctx context.Context, owner, conversationID, questionID, answerID string, answer []byte) error

	FailTurn(ctx context.Context, owner, conversationID, questionID string) error

	// RecentTurns returns up to limit completed turns, newest first.
	RecentTurns(ctx context.Context, owner, conversationID string, limit int) ([]SealedTurn, error)
}

// Turn is one citizen question being answered.
type Turn struct {
	owner, conversationID, questionID string
	answer                            string // set when a retry gets the stored answer
}

// Answered returns the stored answer when the turn is a retry of an answered question.
func (t Turn) Answered() (string, bool) { return t.answer, t.answer != "" }

// Messages saves each exchange encrypted: question pending, then completed or failed.
type Messages struct {
	store    MessageStore
	sealer   sealer
	newID    func() (string, error)
	inFlight time.Duration
}

// NewMessages takes the 32-byte key and how long a question may stay in progress.
func NewMessages(store MessageStore, key []byte, inFlight time.Duration) (*Messages, error) {
	if store == nil || inFlight <= 0 {
		return nil, ErrUnavailable
	}

	s, err := newSealer(key)
	if err != nil {
		return nil, err
	}

	return &Messages{store: store, sealer: s, newID: newUUID, inFlight: inFlight}, nil
}

// Start saves the question as pending; repeating the last question is a retry, never a new turn.
func (m *Messages) Start(ctx context.Context, owner, conversationID, text string) (Turn, error) {
	if owner == "" || len(owner) > maxOwnerLength || !chatcontract.ValidConversationID(conversationID) || text == "" {
		return Turn{}, ErrUnavailable
	}

	id, err := m.newID()
	if err != nil || !chatcontract.ValidConversationID(id) {
		return Turn{}, ErrUnavailable
	}

	sealed, err := m.sealer.seal(text, conversationID, id, RoleCitizen)
	if err != nil {
		return Turn{}, ErrUnavailable
	}

	var turn Turn
	err = m.store.StartTurn(ctx, owner, conversationID, m.inFlight, func(last *LastTurn) (Start, error) {
		turn = Turn{owner: owner, conversationID: conversationID, questionID: id}
		if last == nil || !m.sameQuestion(last, conversationID, text) {
			return Start{NewQuestionID: id, NewQuestion: sealed}, nil
		}

		switch last.Status {
		case StatusPending:
			return Start{}, ErrInProgress
		case StatusFailed:
			turn.questionID = last.QuestionID
			return Start{Reopen: true}, nil
		}

		// A quick repeat of an answered question missed the reply; a later one is a new question
		if last.AnswerID != "" && last.Age <= replayWindow {
			answer, err := m.sealer.open(last.Answer, conversationID, last.AnswerID, RoleAssistant)
			if err == nil && answer != "" {
				turn.questionID, turn.answer = last.QuestionID, answer
				return Start{}, nil
			}
		}

		return Start{NewQuestionID: id, NewQuestion: sealed}, nil
	})

	switch {
	case err == nil:
		return turn, nil
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrInProgress):
		return Turn{}, err
	default:
		return Turn{}, ErrUnavailable
	}
}

// Complete saves the answer and marks the question completed.
func (m *Messages) Complete(ctx context.Context, turn Turn, answer string) error {
	if _, replayed := turn.Answered(); replayed {
		return nil
	}

	if turn.questionID == "" || answer == "" {
		return ErrUnavailable
	}

	id, err := m.newID()
	if err != nil || !chatcontract.ValidConversationID(id) {
		return ErrUnavailable
	}

	sealed, err := m.sealer.seal(answer, turn.conversationID, id, RoleAssistant)
	if err != nil {
		return ErrUnavailable
	}

	if m.store.CompleteTurn(ctx, turn.owner, turn.conversationID, turn.questionID, id, sealed) != nil {
		return ErrUnavailable
	}

	return nil
}

// Fail marks the question failed so it can be retried.
func (m *Messages) Fail(ctx context.Context, turn Turn) error {
	if _, replayed := turn.Answered(); replayed {
		return nil
	}

	if turn.questionID == "" || m.store.FailTurn(ctx, turn.owner, turn.conversationID, turn.questionID) != nil {
		return ErrUnavailable
	}

	return nil
}

// Recent returns the last limit completed turns, oldest first; failed and pending questions are left out.
func (m *Messages) Recent(ctx context.Context, owner, conversationID string, limit int) ([]Message, error) {
	if owner == "" || len(owner) > maxOwnerLength || !chatcontract.ValidConversationID(conversationID) ||
		limit < 1 || limit > MaxRecentTurns {
		return nil, ErrUnavailable
	}

	turns, err := m.store.RecentTurns(ctx, owner, conversationID, limit)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}

	if err != nil || len(turns) > limit {
		return nil, ErrUnavailable
	}

	messages := make([]Message, 0, 2*len(turns))
	for i := len(turns) - 1; i >= 0; i-- {
		question, qErr := m.sealer.open(turns[i].Question, conversationID, turns[i].QuestionID, RoleCitizen)
		answer, aErr := m.sealer.open(turns[i].Answer, conversationID, turns[i].AnswerID, RoleAssistant)

		// A turn sealed with another key cannot be read; it is skipped, not fatal
		if qErr != nil || aErr != nil {
			continue
		}

		messages = append(messages, Message{RoleCitizen, question}, Message{RoleAssistant, answer})
	}

	return messages, nil
}

// sameQuestion is false for a value that no longer decrypts (for example after a key change).
func (m *Messages) sameQuestion(last *LastTurn, conversationID, text string) bool {
	stored, err := m.sealer.open(last.Question, conversationID, last.QuestionID, RoleCitizen)
	return err == nil && stored == text
}
