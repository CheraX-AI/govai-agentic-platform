// Package pgstore keeps conversations and their sealed messages in PostgreSQL.
package pgstore

import (
	"context"
	"errors"
	"hash/fnv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store uses core.conversations and core.conversation_messages (internal/database/migrations).
type Store struct {
	pool *pgxpool.Pool
}

var (
	_ conversation.Store        = (*Store)(nil)
	_ conversation.MessageStore = (*Store)(nil)

	errNotPending = errors.New("question is not pending")
)

// messageLock namespaces the per-conversation advisory locks.
const messageLock int32 = 71480262

const ownsSQL = "SELECT EXISTS (SELECT 1 FROM core.conversations WHERE id = $1 AND citizen_subject = $2)"

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("conversation store needs a database pool")
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Create(ctx context.Context, id, owner string) error {
	if _, err := s.pool.Exec(ctx, "INSERT INTO core.conversations (id, citizen_subject) VALUES ($1, $2)", id, owner); err != nil {
		return conversation.ErrUnavailable
	}

	return nil
}

// Owns matches ID and owner together, so another citizen's conversation looks unknown.
func (s *Store) Owns(ctx context.Context, id, owner string) (bool, error) {
	var owned bool
	if err := s.pool.QueryRow(ctx, ownsSQL, id, owner).Scan(&owned); err != nil {
		return false, conversation.ErrUnavailable
	}

	return owned, nil
}

func (s *Store) StartTurn(
	ctx context.Context,
	owner, conversationID string,
	stale time.Duration,
	decide func(last *conversation.LastTurn) (conversation.Start, error),
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// One request at a time per conversation decides between a new turn and a retry
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1, $2)", messageLock, conversationLock(conversationID)); err != nil {
			return err
		}

		var owned bool
		if err := tx.QueryRow(ctx, ownsSQL, conversationID, owner).Scan(&owned); err != nil {
			return err
		}

		if !owned {
			return conversation.ErrNotFound
		}

		// A question pending for too long belongs to a lost request
		if _, err := tx.Exec(ctx, `UPDATE core.conversation_messages SET status = 'failed'
 WHERE conversation_id = $1 AND role = 'citizen' AND status = 'pending'
   AND updated_at < clock_timestamp() - make_interval(secs => $2)`, conversationID, stale.Seconds()); err != nil {
			return err
		}

		last, err := lastTurn(ctx, tx, conversationID)
		if err != nil {
			return err
		}

		start, err := decide(last)
		if err != nil {
			return err
		}

		switch {
		case start.Reopen:
			if last == nil {
				return errNotPending
			}

			tag, err := tx.Exec(ctx, `UPDATE core.conversation_messages SET status = 'pending'
 WHERE message_id = $1 AND conversation_id = $2 AND status = 'failed'`, last.QuestionID, conversationID)
			if err != nil {
				return err
			}

			if tag.RowsAffected() != 1 {
				return errNotPending
			}

		case start.NewQuestionID != "":
			_, err := tx.Exec(ctx, `INSERT INTO core.conversation_messages
 (message_id, conversation_id, role, content_ciphertext, status) VALUES ($1, $2, 'citizen', $3, 'pending')`,
				start.NewQuestionID, conversationID, start.NewQuestion)
			return err
		}

		return nil
	})
}

func (s *Store) CompleteTurn(ctx context.Context, owner, conversationID, questionID, answerID string, answer []byte) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := settle(ctx, tx, owner, conversationID, questionID, "completed"); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `INSERT INTO core.conversation_messages
 (message_id, conversation_id, role, content_ciphertext, status, reply_to)
 VALUES ($1, $2, 'assistant', $3, 'completed', $4)`, answerID, conversationID, answer, questionID)
		return err
	})
}

func (s *Store) FailTurn(ctx context.Context, owner, conversationID, questionID string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return settle(ctx, tx, owner, conversationID, questionID, "failed")
	})
}

func (s *Store) RecentTurns(ctx context.Context, owner, conversationID string, limit int) ([]conversation.SealedTurn, error) {
	var turns []conversation.SealedTurn
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var owned bool
		if err := tx.QueryRow(ctx, ownsSQL, conversationID, owner).Scan(&owned); err != nil {
			return err
		}

		if !owned {
			return conversation.ErrNotFound
		}

		rows, err := tx.Query(ctx, `SELECT q.message_id::text, q.content_ciphertext, a.message_id::text, a.content_ciphertext
 FROM core.conversation_messages q
 JOIN core.conversation_messages a ON a.reply_to = q.message_id AND a.conversation_id = q.conversation_id
 WHERE q.conversation_id = $1 AND q.role = 'citizen' AND q.status = 'completed'
 ORDER BY q.ordinal DESC LIMIT $2`, conversationID, limit)
		if err != nil {
			return err
		}

		turns, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (conversation.SealedTurn, error) {
			var t conversation.SealedTurn
			return t, row.Scan(&t.QuestionID, &t.Question, &t.AnswerID, &t.Answer)
		})
		return err
	})

	return turns, err
}

// lastTurn reads the newest citizen message and its answer; nil when there is none.
func lastTurn(ctx context.Context, tx pgx.Tx, conversationID string) (*conversation.LastTurn, error) {
	var last conversation.LastTurn
	var age float64
	var answerID *string
	err := tx.QueryRow(ctx, `SELECT q.message_id::text, q.status, q.content_ciphertext,
       extract(epoch FROM clock_timestamp() - q.updated_at)::float8,
       a.message_id::text, a.content_ciphertext
 FROM core.conversation_messages q
 LEFT JOIN core.conversation_messages a ON a.reply_to = q.message_id
 WHERE q.conversation_id = $1 AND q.role = 'citizen'
 ORDER BY q.ordinal DESC LIMIT 1`, conversationID).
		Scan(&last.QuestionID, &last.Status, &last.Question, &age, &answerID, &last.Answer)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	last.Age = time.Duration(age * float64(time.Second))
	if answerID != nil {
		last.AnswerID = *answerID
	}

	return &last, nil
}

// settle moves the owner's pending question to status, or fails if it is not pending.
func settle(ctx context.Context, tx pgx.Tx, owner, conversationID, questionID, status string) error {
	tag, err := tx.Exec(ctx, `UPDATE core.conversation_messages SET status = $4
 WHERE message_id = $1 AND conversation_id = $2 AND role = 'citizen' AND status = 'pending'
   AND EXISTS (SELECT 1 FROM core.conversations WHERE id = $2 AND citizen_subject = $3)`,
		questionID, conversationID, owner, status)
	if err != nil {
		return err
	}

	if tag.RowsAffected() != 1 {
		return errNotPending
	}

	return nil
}

// conversationLock maps a conversation ID to a lock key; a collision only adds a short wait.
func conversationLock(conversationID string) int32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(conversationID))
	return int32(h.Sum32())
}
