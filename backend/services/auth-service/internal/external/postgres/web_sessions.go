package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type webPrivate struct{}

var ErrWebSessionMissing = errors.New("web session missing or inactive")

func webReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWebSessionMissing
	}

	return ErrStore
}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (webPrivate) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (webPrivate) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (webPrivate) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

type WebCredentials struct {
	webPrivate
	Issuer, Subject, AccessToken, RefreshToken, CSRF string
	AccessUntil, RefreshUntil                        int64
}

type WebSnapshot struct {
	webPrivate
	Credentials WebCredentials
	Revision    int64
	Expires     time.Time
}

type WebSessionStore struct {
	webPrivate
	pool *pgxpool.Pool
	aead cipher.AEAD
}

// key must be a dedicated stable 32-byte web-session encryption key supplied
// by configuration. No password, provider token or raw cookie is stored in SQL.
func NewWebSessionStore(pool *pgxpool.Pool, key []byte) (*WebSessionStore, error) {
	if pool == nil || len(key) != 32 {
		return nil, ErrStore
	}

	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrStore
	}

	a, err := cipher.NewGCM(b)
	if err != nil {
		return nil, ErrStore
	}

	return &WebSessionStore{pool: pool, aead: a}, nil
}

func webHash(raw string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != raw {
		return nil, ErrStore
	}

	h := sha256.Sum256(append([]byte("govai-web-session-id-v1\x00"), b...))
	return h[:], nil
}

func webPayload(c WebCredentials) map[string]string {
	return map[string]string{
		"issuer":        c.Issuer,
		"subject":       c.Subject,
		"access":        c.AccessToken,
		"refresh":       c.RefreshToken,
		"csrf":          c.CSRF,
		"access_until":  strconv.FormatInt(c.AccessUntil, 10),
		"refresh_until": strconv.FormatInt(c.RefreshUntil, 10),
	}
}

func validWebCredentials(c WebCredentials) bool {
	return c.Issuer != "" &&
		len(c.Issuer) <= 2048 &&
		c.Subject != "" &&
		len(c.Subject) <= 128 &&
		c.AccessToken != "" &&
		len(c.AccessToken) <= 16<<10 &&
		c.RefreshToken != "" &&
		len(c.RefreshToken) <= 16<<10 &&
		len(c.CSRF) == 43
}

func (s *WebSessionStore) seal(c WebCredentials, hash []byte) ([]byte, error) {
	if !validWebCredentials(c) {
		return nil, ErrStore
	}

	b, err := json.Marshal(webPayload(c))
	if err != nil {
		return nil, ErrStore
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, ErrStore
	}

	return s.aead.Seal(nonce, nonce, b, append([]byte("govai-web-tokens-v1\x00"), hash...)), nil
}

func (s *WebSessionStore) open(b, hash []byte) (WebCredentials, error) {
	n := s.aead.NonceSize()
	if len(b) < n || len(b) > 65536 {
		return WebCredentials{}, ErrStore
	}

	plain, err := s.aead.Open(nil, b[:n], b[n:], append([]byte("govai-web-tokens-v1\x00"), hash...))
	if err != nil {
		return WebCredentials{}, ErrStore
	}

	var p map[string]string
	if json.Unmarshal(plain, &p) != nil {
		return WebCredentials{}, ErrStore
	}

	c := WebCredentials{
		Issuer:       p["issuer"],
		Subject:      p["subject"],
		AccessToken:  p["access"],
		RefreshToken: p["refresh"],
		CSRF:         p["csrf"],
	}
	var parseErr error
	c.AccessUntil, parseErr = strconv.ParseInt(p["access_until"], 10, 64)
	if parseErr != nil {
		return WebCredentials{}, ErrStore
	}

	c.RefreshUntil, parseErr = strconv.ParseInt(p["refresh_until"], 10, 64)
	if parseErr != nil {
		return WebCredentials{}, ErrStore
	}

	if !validWebCredentials(c) {
		return WebCredentials{}, ErrStore
	}

	return c, nil
}

func (s *WebSessionStore) Create(ctx context.Context, c WebCredentials, expires time.Time) (string, error) {
	if !expires.After(time.Now()) || expires.After(time.Now().Add(24*time.Hour)) {
		return "", ErrStore
	}

	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", ErrStore
	}

	raw := base64.RawURLEncoding.EncodeToString(random[:])
	hash, _ := webHash(raw)
	sealed, err := s.seal(c, hash)
	if err != nil {
		return "", ErrStore
	}

	if _, err = s.pool.Exec(
		ctx,
		"INSERT INTO auth.web_sessions(session_hash,ciphertext,expires_at) VALUES($1,$2,$3)",
		hash,
		sealed,
		expires,
	); err != nil {
		return "", ErrStore
	}

	return raw, nil
}

func (s *WebSessionStore) Load(ctx context.Context, raw string) (WebSnapshot, error) {
	return s.read(ctx, raw, false)
}

// LoadForTermination permits CSRF-checked local logout of a pending refresh.
// This snapshot must never authorize application access.
func (s *WebSessionStore) LoadForTermination(ctx context.Context, raw string) (WebSnapshot, error) {
	hash, err := webHash(raw)
	if err != nil {
		return WebSnapshot{}, ErrWebSessionMissing
	}

	var b []byte
	var snap WebSnapshot
	if err := s.pool.QueryRow(
		ctx,
		"SELECT ciphertext,revision,expires_at FROM auth.web_sessions WHERE session_hash=$1 AND state IN ('active','refreshing')",
		hash,
	).Scan(&b, &snap.Revision, &snap.Expires); err != nil {
		return WebSnapshot{}, webReadError(err)
	}

	snap.Credentials, err = s.open(b, hash)
	if err != nil {
		return WebSnapshot{}, ErrStore
	}

	return snap, nil
}

// BeginRefresh durably consumes the active state before a provider call. A
// crash or ambiguous request leaves refreshing, never an automatic token replay.
func (s *WebSessionStore) BeginRefresh(ctx context.Context, raw string) (WebSnapshot, error) {
	return s.read(ctx, raw, true)
}

func (s *WebSessionStore) read(ctx context.Context, raw string, begin bool) (WebSnapshot, error) {
	hash, err := webHash(raw)
	if err != nil {
		return WebSnapshot{}, ErrWebSessionMissing
	}

	query := "SELECT ciphertext,revision,expires_at FROM auth.web_sessions WHERE session_hash=$1 AND state='active' AND expires_at>clock_timestamp()"
	if begin {
		query = "UPDATE auth.web_sessions SET state='refreshing',revision=revision+1 WHERE session_hash=$1 AND state='active' AND expires_at>clock_timestamp() RETURNING ciphertext,revision,expires_at"
	}

	var b []byte
	var snap WebSnapshot
	if err := s.pool.QueryRow(ctx, query, hash).Scan(&b, &snap.Revision, &snap.Expires); err != nil {
		return WebSnapshot{}, webReadError(err)
	}

	snap.Credentials, err = s.open(b, hash)
	if err != nil {
		return WebSnapshot{}, ErrStore
	}

	return snap, nil
}

// FinishRefresh accepts only the original identity and CSRF binding. A stale
// owner cannot overwrite another refresh or revive a locally revoked session.
func (s *WebSessionStore) FinishRefresh(ctx context.Context, raw string, expected WebSnapshot, next WebCredentials) error {
	if expected.Revision < 1 ||
		next.Issuer != expected.Credentials.Issuer ||
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
	// Bind the caller's checkpoint to the encrypted row as well as its revision.
	var current []byte
	if s.pool.QueryRow(
		ctx,
		"SELECT ciphertext FROM auth.web_sessions WHERE session_hash=$1 AND revision=$2 AND state='refreshing'",
		hash,
		expected.Revision,
	).Scan(&current) != nil {
		return ErrStore
	}

	owner, err := s.open(current, hash)
	if err != nil || owner.Issuer != next.Issuer || owner.Subject != next.Subject || owner.CSRF != next.CSRF {
		return ErrStore
	}

	tag, err := s.pool.Exec(
		ctx,
		"UPDATE auth.web_sessions SET ciphertext=$3,state='active',revision=revision+1 WHERE session_hash=$1 AND revision=$2 AND state='refreshing' AND expires_at>clock_timestamp()",
		hash,
		expected.Revision,
		sealed,
	)
	if err != nil || tag.RowsAffected() != 1 {
		return ErrStore
	}

	return nil
}

// Terminate makes the local session unusable, including during refresh. It
// does not claim provider logout. Ciphertext is retained for a future bounded
// revocation/reconciliation policy; no active loader accepts the revoked row.
func (s *WebSessionStore) Terminate(ctx context.Context, raw string) error {
	hash, err := webHash(raw)
	if err != nil {
		return ErrStore
	}

	if _, err = s.pool.Exec(
		ctx,
		"UPDATE auth.web_sessions SET state='revoked',revision=revision+1 WHERE session_hash=$1 AND state<>'revoked'",
		hash,
	); err != nil {
		return ErrStore
	}

	return nil
}
