package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRecoveryUnavailable = errors.New("recovery unavailable or expired")

// RecoveryStore persists hashes only. Claim irreversibly spends the token BEFORE
// any Keycloak mutation. An ambiguous provider result must enter attention, never
// reactivate the token or blindly retry the password mutation.
type RecoveryStore struct {
	webPrivate
	pool *pgxpool.Pool
}

type RecoveryIdentity struct {
	webPrivate
	Issuer, Subject string
}

// NewRecoveryStore prepares persistent reset-challenge access.
func NewRecoveryStore(p *pgxpool.Pool) (*RecoveryStore, error) {
	if p == nil {
		return nil, ErrStore
	}

	return &RecoveryStore{pool: p}, nil
}

// MigrateRecovery requires baseline 001; no existing optional migration changes.
func MigrateRecovery(ctx context.Context, p *pgxpool.Pool) error {
	if p == nil {
		return ErrStore
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return ErrStore
	}

	defer func() {
		c, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(c)
	}()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71480261031)"); err != nil {
		return ErrStore
	}

	baseline, _ := migrations.ReadFile("migrations/001_registration.sql")
	sum := sha256.Sum256(baseline)
	var old string
	if tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=1").Scan(&old) != nil ||

		old != hex.EncodeToString(sum[:]) {
		return ErrStore
	}

	sql, err := migrations.ReadFile("migrations/004_password_recovery.sql")
	if err != nil {
		return ErrStore
	}

	sum = sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=4").Scan(&old)
	if err == nil {
		if old != checksum {
			return ErrStore
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return ErrStore
		}

		if _, err = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(4,$1)", checksum); err != nil {
			return ErrStore
		}
	} else {
		return ErrStore
	}

	if err = applyRecoveryBindings(ctx, tx); err != nil {
		return err
	}

	if tx.Commit(ctx) != nil {
		return ErrStore
	}

	return nil
}

func (s *RecoveryStore) Issue(ctx context.Context, identity RecoveryIdentity, ttl time.Duration) (recovery.Token, error) {
	if identity.Issuer == "" ||
		len(identity.Issuer) > 2048 ||
		identity.Subject == "" ||
		len(identity.Subject) > 128 ||
		ttl < time.Second ||
		ttl > 30*time.Minute {
		return recovery.Token{}, ErrStore
	}

	token, err := recovery.NewToken()
	if err != nil {
		return recovery.Token{}, ErrStore
	}

	h, err := token.Digest()
	if err != nil {
		return recovery.Token{}, ErrStore
	}
	// Both timestamps use the database clock. Token identity is trusted server input.
	_, err = s.pool.Exec(ctx, `INSERT INTO auth.password_recovery(token_hash,issuer,subject,expires_at)
 SELECT $1,$2,$3,statement_timestamp()+($4*interval '1 second')`,
		h[:],
		identity.Issuer,
		identity.Subject,
		ttl.Seconds())
	// created_at is default-owned: omit it from INSERT under runtime privileges.
	if err != nil {
		return recovery.Token{}, ErrStore
	}

	return token, nil
}

func (s *RecoveryStore) Claim(ctx context.Context, token recovery.Token) (RecoveryIdentity, error) {
	h, err := token.Digest()
	if err != nil {
		return RecoveryIdentity{}, ErrRecoveryUnavailable
	}

	var identity RecoveryIdentity
	err = s.pool.QueryRow(ctx, `UPDATE auth.password_recovery SET state='claimed'
 WHERE token_hash=$1 AND state='pending' AND expires_at>clock_timestamp()
 RETURNING issuer,subject`, h[:]).Scan(&identity.Issuer, &identity.Subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return RecoveryIdentity{}, ErrRecoveryUnavailable
	}

	if err != nil {
		return RecoveryIdentity{}, ErrStore
	}

	return identity, nil
}

func (s *RecoveryStore) Finish(ctx context.Context, token recovery.Token, success bool) error {
	h, err := token.Digest()
	if err != nil {
		return ErrRecoveryUnavailable
	}

	state := "attention"
	if success {
		state = "completed"
	}

	result, err := s.pool.Exec(
		ctx,
		`UPDATE auth.password_recovery SET state=$2 WHERE token_hash=$1 AND state='claimed'`,
		h[:],
		state,
	)
	if err != nil {
		return ErrStore
	}

	if result.RowsAffected() != 1 {
		return ErrRecoveryUnavailable
	}

	return nil
}
