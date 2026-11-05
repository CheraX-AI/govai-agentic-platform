package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RecoveryWebStore is opt-in. Unbound legacy sessions fail closed. Every web
// instance must use it before password recovery can be enabled. Cookie login
// must call AuthenticationMark BEFORE provider login, then CreateFenced.
type RecoveryWebStore struct {
	*BoundWebSessionStore
	fenceKey []byte
}

var _ recovery.LocalSessions = (*RecoveryWebStore)(nil)

func NewRecoveryWebStore(b *BoundWebSessionStore, key []byte) (*RecoveryWebStore, error) {
	if b == nil || len(key) != 32 {
		return nil, ErrStore
	}

	return &RecoveryWebStore{BoundWebSessionStore: b, fenceKey: append([]byte(nil), key...)}, nil
}

func MigrateRecoverySessions(ctx context.Context, p *pgxpool.Pool) error {
	if p == nil || MigrateWebSessions(ctx, p) != nil || MigrateRecovery(ctx, p) != nil {
		return ErrStore
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return ErrStore
	}

	defer rollbackRecovery(tx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71480261031)"); err != nil {
		return ErrStore
	}

	sql, err := migrations.ReadFile("migrations/006_recovery_session_fences.sql")
	if err != nil {
		return ErrStore
	}

	sum := sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	var old string
	err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=6").Scan(&old)
	if err == nil {
		if old != checksum {
			return ErrStore
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return ErrStore
		}

		if _, err = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(6,$1)", checksum); err != nil {
			return ErrStore
		}
	} else {
		return ErrStore
	}

	if tx.Commit(ctx) != nil {
		return ErrStore
	}

	return nil
}

func rollbackRecovery(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func (s *RecoveryWebStore) identity(issuer, subject string) ([]byte, error) {
	if issuer != s.issuer || subject == "" || len(subject) > 128 {
		return nil, ErrStore
	}

	mac := hmac.New(sha256.New, s.fenceKey)
	body, _ := json.Marshal([]string{"govai:web:recovery-fence:v1", issuer, subject})
	_, _ = mac.Write(body)
	return mac.Sum(nil), nil
}

func (s *RecoveryWebStore) lock(ctx context.Context, identity []byte) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, ErrStore
	}

	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(binary.BigEndian.Uint64(identity[:8]))); err != nil {
		rollbackRecovery(tx)
		return nil, ErrStore
	}

	return tx, nil
}

func (s *RecoveryWebStore) AuthenticationMark(ctx context.Context) (int64, error) {
	var mark int64
	if s.pool.QueryRow(ctx, "SELECT nextval('auth.recovery_fence_order')").Scan(&mark) != nil || mark <= 0 {
		return 0, ErrStore
	}

	return mark, nil
}

// Create without a pre-login marker is prohibited on guarded stores.
func (s *RecoveryWebStore) Create(context.Context, WebCredentials, time.Time) (string, error) {
	return "", ErrStore
}

func (s *RecoveryWebStore) CreateFenced(ctx context.Context, c WebCredentials, expires time.Time, mark int64) (string, error) {
	identity, err := s.identity(c.Issuer, c.Subject)
	if err != nil || mark <= 0 {
		return "", ErrStore
	}

	raw, err := s.WebSessionStore.Create(ctx, c, expires)
	if err != nil {
		return "", err
	}

	committed := false
	defer func() {
		if !committed {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = s.WebSessionStore.Terminate(cleanup, raw)
		}
	}()
	tx, err := s.lock(ctx, identity)
	if err != nil {
		return "", err
	}

	defer rollbackRecovery(tx)
	var cutoff int64
	if tx.QueryRow(
		ctx,
		"SELECT COALESCE((SELECT revoked_through FROM auth.web_recovery_fences WHERE identity_hash=$1),0)",
		identity,
	).Scan(&cutoff) != nil {
		return "", ErrStore
	}

	if mark <= cutoff {
		return "", ErrWebSessionMissing
	}

	hash, _ := webHash(raw)
	if _, err = tx.Exec(
		ctx,
		"INSERT INTO auth.web_recovery_sessions(session_hash,identity_hash,login_order) VALUES($1,$2,$3)",
		hash,
		identity,
		mark,
	); err != nil {
		return "", ErrStore
	}

	if tx.Commit(ctx) != nil {
		return "", ErrStore
	}

	committed = true
	return raw, nil
}

func (s *RecoveryWebStore) valid(ctx context.Context, raw string, c WebCredentials) error {
	identity, err := s.identity(c.Issuer, c.Subject)
	if err != nil {
		return ErrStore
	}

	hash, err := webHash(raw)
	if err != nil {
		return ErrWebSessionMissing
	}

	var valid bool
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth.web_recovery_sessions b
 LEFT JOIN auth.web_recovery_fences f ON b.identity_hash=f.identity_hash
 WHERE b.session_hash=$1 AND b.identity_hash=$2 AND b.login_order>COALESCE(f.revoked_through,0))`,
		hash,
		identity).Scan(&valid)
	if err != nil {
		return ErrStore
	}

	if !valid {
		return ErrWebSessionMissing
	}

	return nil
}

func (s *RecoveryWebStore) Load(ctx context.Context, raw string) (WebSnapshot, error) {
	snap, err := s.WebSessionStore.Load(ctx, raw)
	if err != nil {
		return WebSnapshot{}, err
	}

	if err = s.valid(ctx, raw, snap.Credentials); err != nil {
		return WebSnapshot{}, err
	}

	return snap, nil
}

func (s *RecoveryWebStore) LoadForTermination(ctx context.Context, raw string) (WebSnapshot, error) {
	snap, err := s.WebSessionStore.LoadForTermination(ctx, raw)
	if err != nil {
		return WebSnapshot{}, err
	}

	if err = s.valid(ctx, raw, snap.Credentials); err != nil {
		return WebSnapshot{}, err
	}

	return snap, nil
}

func (s *RecoveryWebStore) BeginRefresh(ctx context.Context, raw string) (WebSnapshot, error) {
	snap, err := s.WebSessionStore.BeginRefresh(ctx, raw)
	if err != nil {
		return WebSnapshot{}, err
	}

	if err = s.valid(ctx, raw, snap.Credentials); err != nil {
		return WebSnapshot{}, err
	}

	return snap, nil
}

func (s *RecoveryWebStore) FinishRefresh(ctx context.Context, raw string, expected WebSnapshot, next WebCredentials) error {
	identity, err := s.identity(next.Issuer, next.Subject)
	if err != nil {
		return ErrStore
	}

	if next.Issuer != expected.Credentials.Issuer ||
		next.Subject != expected.Credentials.Subject ||
		next.CSRF != expected.Credentials.CSRF {
		return ErrStore
	}

	hash, err := webHash(raw)
	if err != nil {
		return ErrStore
	}

	sealed, err := s.seal(next, hash)
	if err != nil {
		return ErrStore
	}

	tx, err := s.lock(ctx, identity)
	if err != nil {
		return err
	}

	defer rollbackRecovery(tx)
	var previous []byte
	err = tx.QueryRow(ctx, `SELECT p.ciphertext FROM auth.web_sessions p
 JOIN auth.web_recovery_sessions b ON p.session_hash=b.session_hash
 LEFT JOIN auth.web_recovery_fences f ON b.identity_hash=f.identity_hash
 WHERE p.session_hash=$1 AND p.revision=$2 AND p.state='refreshing' AND p.expires_at>clock_timestamp()
 AND b.identity_hash=$3 AND b.login_order>COALESCE(f.revoked_through,0) FOR UPDATE OF p`,
		hash,
		expected.Revision,
		identity).Scan(&previous)
	if err != nil {
		return ErrWebSessionMissing
	}

	owner, err := s.open(previous, hash)
	if err != nil || owner.Issuer != next.Issuer || owner.Subject != next.Subject || owner.CSRF != next.CSRF {
		return ErrStore
	}

	result, err := tx.Exec(
		ctx,
		"UPDATE auth.web_sessions SET ciphertext=$3,state='active',revision=revision+1 WHERE session_hash=$1 AND revision=$2 AND state='refreshing'",
		hash,
		expected.Revision,
		sealed,
	)
	if err != nil || result.RowsAffected() != 1 {
		return ErrStore
	}

	if tx.Commit(ctx) != nil {
		return ErrStore
	}

	return nil
}

// Revoke advances a durable identity cutoff. Guarded loaders reject all sessions
// whose login began at/before it, including refreshes finishing concurrently.
// Physical ciphertext cleanup and provider revocation are separate operations.
func (s *RecoveryWebStore) Revoke(ctx context.Context, b recovery.Binding) error {
	identity, err := s.identity(b.Issuer, b.Subject)
	if err != nil {
		return recovery.ErrUnavailable
	}

	tx, err := s.lock(ctx, identity)
	if err != nil {
		return recovery.ErrUnavailable
	}

	defer rollbackRecovery(tx)
	_, err = tx.Exec(ctx, `INSERT INTO auth.web_recovery_fences(identity_hash,revoked_through)
 VALUES($1,nextval('auth.recovery_fence_order')) ON CONFLICT(identity_hash)
 DO UPDATE SET revoked_through=EXCLUDED.revoked_through`, identity)
	if err != nil {
		return recovery.ErrUnavailable
	}

	_, err = tx.Exec(ctx, `UPDATE auth.web_sessions p SET state='revoked',revision=revision+1
    FROM auth.web_recovery_sessions b WHERE p.session_hash=b.session_hash
    AND b.identity_hash=$1 AND p.state IN ('active','refreshing')`, identity)
	if err != nil || tx.Commit(ctx) != nil {
		return recovery.ErrUnavailable
	}

	return nil
}
