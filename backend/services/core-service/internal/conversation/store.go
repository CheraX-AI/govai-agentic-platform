package conversation

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore keeps conversations in core.conversations
// (internal/database/migrations/001_conversations.sql).
type PostgresStore struct {
	pool *pgxpool.Pool
}

var _ Store = (*PostgresStore)(nil)

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, errors.New("conversation store needs a database pool")
	}

	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Create(ctx context.Context, id, owner string) error {
	if _, err := s.pool.Exec(
		ctx,
		"INSERT INTO core.conversations (id, citizen_subject) VALUES ($1, $2)",
		id,
		owner,
	); err != nil {
		return ErrUnavailable
	}

	return nil
}

// Owns matches the ID and the owner in one query, so another citizen's
// conversation looks exactly like an unknown one.
func (s *PostgresStore) Owns(ctx context.Context, id, owner string) (bool, error) {
	var owned bool
	if err := s.pool.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT 1 FROM core.conversations WHERE id = $1 AND citizen_subject = $2)",
		id,
		owner,
	).Scan(&owned); err != nil {
		return false, ErrUnavailable
	}

	return owned, nil
}
