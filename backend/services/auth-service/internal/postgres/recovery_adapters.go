package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/jackc/pgx/v5"
)

var operationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func applyRecoveryBindings(ctx context.Context, tx pgx.Tx) error {
	sql, err := migrations.ReadFile("migrations/005_recovery_bindings.sql")
	if err != nil {
		return ErrStore
	}
	sum := sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	var old string
	err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=5").Scan(&old)
	if err == nil {
		if old != checksum {
			return ErrStore
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ErrStore
	}
	if _, err = tx.Exec(ctx, string(sql)); err != nil {
		return ErrStore
	}
	if _, err = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(5,$1)", checksum); err != nil {
		return ErrStore
	}
	return nil
}

// RecoveryAccounts adapts internal lookup; no profile plaintext or Keycloak URL
// comes from the request. It deliberately returns only saved ownership binding.
type RecoveryAccounts struct {
	webPrivate
	store *Store
}

func NewRecoveryAccounts(s *Store) (*RecoveryAccounts, error) {
	if s == nil {
		return nil, ErrStore
	}
	return &RecoveryAccounts{store: s}, nil
}
func (a *RecoveryAccounts) Lookup(ctx context.Context, issuer string, nic registration.NIC) (recovery.Binding, error) {
	b, err := a.store.RecoveryByNIC(ctx, issuer, nic)
	if errors.Is(err, ErrRecoveryUnavailable) {
		return recovery.Binding{}, recovery.ErrNotFound
	}
	if err != nil {
		return recovery.Binding{}, recovery.ErrUnavailable
	}
	return recovery.Binding{Issuer: b.Issuer, Subject: b.Subject, OperationID: b.OperationID}, nil
}

type RecoveryChallenges struct {
	webPrivate
	store *RecoveryStore
}

func NewRecoveryChallenges(s *RecoveryStore) (*RecoveryChallenges, error) {
	if s == nil {
		return nil, ErrStore
	}
	return &RecoveryChallenges{store: s}, nil
}

var _ recovery.Challenges = (*RecoveryChallenges)(nil)
var _ recovery.Accounts = (*RecoveryAccounts)(nil)

func (a *RecoveryChallenges) Issue(ctx context.Context, c recovery.Challenge, ttl time.Duration) (recovery.Token, error) {
	if c.Account.Issuer == "" || len(c.Account.Issuer) > 2048 || !operationIDPattern.MatchString(c.Account.OperationID) || !operationIDPattern.MatchString(c.Account.Subject) || c.EmailTag == ([32]byte{}) || ttl < time.Second || ttl > 30*time.Minute {
		return recovery.Token{}, recovery.ErrUnavailable
	}
	token, err := recovery.NewToken()
	if err != nil {
		return recovery.Token{}, recovery.ErrUnavailable
	}
	hash, _ := token.Digest()
	tx, err := a.store.pool.Begin(ctx)
	if err != nil {
		return recovery.Token{}, recovery.ErrUnavailable
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(c)
	}()
	_, err = tx.Exec(ctx, `INSERT INTO auth.password_recovery(token_hash,issuer,subject,expires_at)
 SELECT $1,$2,$3,statement_timestamp()+($4*interval '1 second')`, hash[:], c.Account.Issuer, c.Account.Subject, ttl.Seconds())
	if err != nil {
		return recovery.Token{}, recovery.ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth.recovery_bindings(token_hash,operation_id,email_tag) VALUES($1,$2,$3)`, hash[:], c.Account.OperationID, c.EmailTag[:]); err != nil {
		return recovery.Token{}, recovery.ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return recovery.Token{}, recovery.ErrUnavailable
	}
	return token, nil
}
func (a *RecoveryChallenges) Claim(ctx context.Context, token recovery.Token) (recovery.Challenge, error) {
	hash, err := token.Digest()
	if err != nil {
		return recovery.Challenge{}, recovery.ErrInvalid
	}
	var c recovery.Challenge
	var tag []byte
	err = a.store.pool.QueryRow(ctx, `UPDATE auth.password_recovery p SET state='claimed'
 FROM auth.recovery_bindings b WHERE p.token_hash=$1 AND p.token_hash=b.token_hash
 AND p.state='pending' AND p.expires_at>clock_timestamp()
 RETURNING p.issuer,p.subject,b.operation_id,b.email_tag`, hash[:]).Scan(&c.Account.Issuer, &c.Account.Subject, &c.Account.OperationID, &tag)
	if errors.Is(err, pgx.ErrNoRows) {
		return recovery.Challenge{}, recovery.ErrInvalid
	}
	if err != nil || len(tag) != 32 {
		return recovery.Challenge{}, recovery.ErrUnavailable
	}
	copy(c.EmailTag[:], tag)
	return c, nil
}
func (a *RecoveryChallenges) Finish(ctx context.Context, token recovery.Token, success bool) error {
	if a.store.Finish(ctx, token, success) != nil {
		return recovery.ErrUnavailable
	}
	return nil
}
