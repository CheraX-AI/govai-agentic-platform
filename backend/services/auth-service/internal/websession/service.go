// Package websession owns browser session lifecycle. It never returns provider
// tokens from public methods; HTTP cookie/origin/CSRF adapters remain separate.
package websession

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"io"
	"log/slog"
	"time"
)

var ErrDenied = errors.New("web_session_rejected")
var ErrUnavailable = errors.New("web_session_unavailable")
var ErrRevocationPending = errors.New("web_session_ended_provider_revocation_pending")

type private struct{}

func (private) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (private) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (private) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

type Started struct {
	private
	Cookie, CSRF string
	Expires      time.Time
}
type Identity struct{ Issuer, Subject string }
type Login interface {
	Login(context.Context, login.Request) (login.Access, error)
}
type Rotation interface {
	RefreshBound(context.Context, string, login.SessionIdentity) (login.SessionTokens, error)
	RevokeNative(context.Context, string) error
}
type Store interface {
	Create(context.Context, postgres.WebCredentials, time.Time) (string, error)
	Load(context.Context, string) (postgres.WebSnapshot, error)
	LoadForTermination(context.Context, string) (postgres.WebSnapshot, error)
	BeginRefresh(context.Context, string) (postgres.WebSnapshot, error)
	FinishRefresh(context.Context, string, postgres.WebSnapshot, postgres.WebCredentials) error
	Terminate(context.Context, string) error
}
type RevocationStore interface {
	TerminateQueued(context.Context, string) (string, error)
	CompleteRevocation(context.Context, string) error
	QueueRevocation(context.Context, string, string) (string, error)
	LoadRevocation(context.Context, string) (postgres.WebRevocation, error)
}
type Service struct {
	private
	login    Login
	rotation Rotation
	verifier login.AccessVerifier
	profiles login.ProfileLookup
	store    Store
	lifetime time.Duration
	now      func() time.Time
}

func New(l Login, r Rotation, v login.AccessVerifier, p login.ProfileLookup, store Store, lifetime time.Duration) (*Service, error) {
	if l == nil || r == nil || v == nil || p == nil || store == nil || lifetime < time.Minute || lifetime > 24*time.Hour {
		return nil, ErrUnavailable
	}
	return &Service{login: l, rotation: r, verifier: v, profiles: p, store: store, lifetime: lifetime, now: time.Now}, nil
}
func (s *Service) Start(ctx context.Context, request login.Request) (Started, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	issued, err := s.login.Login(ctx, request)
	if err != nil {
		if errors.Is(err, login.ErrFailed) {
			return Started{}, ErrDenied
		}
		return Started{}, ErrUnavailable
	}
	if issued.RefreshToken == "" || issued.RefreshExpiresIn <= 0 || issued.RefreshExpiresIn > 86400*365 || issued.ExpiresIn <= 0 || issued.ExpiresIn > 3600 {
		return Started{}, ErrUnavailable
	}
	claims, err := s.verifier.Verify(ctx, issued.Token)
	if err != nil {
		return Started{}, ErrUnavailable
	}
	if !claims.Verified || !claims.Citizen || claims.Issuer == "" || claims.Subject == "" || !claims.Expires.After(s.now()) {
		return Started{}, ErrDenied
	}
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return Started{}, ErrUnavailable
	}
	csrf := base64.RawURLEncoding.EncodeToString(random[:])
	now := s.now()
	expires := now.Add(s.lifetime)
	refreshExpiry := now.Add(time.Duration(issued.RefreshExpiresIn) * time.Second)
	if refreshExpiry.Before(expires) {
		expires = refreshExpiry
	}
	accessExpiry := now.Add(time.Duration(issued.ExpiresIn) * time.Second)
	if claims.Expires.Before(accessExpiry) {
		accessExpiry = claims.Expires
	}
	c := postgres.WebCredentials{Issuer: claims.Issuer, Subject: claims.Subject, AccessToken: issued.Token, RefreshToken: issued.RefreshToken, CSRF: csrf, AccessUntil: accessExpiry.Unix(), RefreshUntil: refreshExpiry.Unix()}
	raw, err := s.store.Create(ctx, c, expires)
	if err != nil || ctx.Err() != nil {
		return Started{}, ErrUnavailable
	}
	return Started{Cookie: raw, CSRF: csrf, Expires: expires}, nil
}

// Check performs no implicit refresh on a read-only browser request.
func (s *Service) Check(ctx context.Context, raw string) (Identity, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	snap, err := s.store.Load(ctx, raw)
	if err != nil {
		return Identity{}, "", storeError(err)
	}
	c := snap.Credentials
	if !snap.Expires.After(s.now()) || c.AccessUntil <= s.now().Unix() || c.RefreshUntil <= s.now().Unix() {
		return Identity{}, "", ErrDenied
	}
	claims, err := s.verifier.Verify(ctx, c.AccessToken)
	if err != nil {
		return Identity{}, "", ErrDenied
	}
	if claims.Issuer != c.Issuer || claims.Subject != c.Subject || !claims.Verified || !claims.Citizen {
		return Identity{}, "", ErrDenied
	}
	ready, err := s.profiles.HasReadyProfile(ctx, c.Issuer, c.Subject)
	if err != nil || ctx.Err() != nil {
		return Identity{}, "", ErrUnavailable
	}
	if !ready {
		return Identity{}, "", ErrDenied
	}
	return Identity{Issuer: c.Issuer, Subject: c.Subject}, c.CSRF, nil
}

func csrfMatches(a, b string) bool {
	return len(a) == 43 && len(b) == 43 && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// CSRF restores the synchronizer token after a page reload even when the
// access JWT expired. It grants no application access and never refreshes.
func (s *Service) CSRF(ctx context.Context, raw string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// A pending refresh grants no application access, but must remain log-outable
	// after reload. Only the CSRF secret is returned from this snapshot.
	snap, err := s.store.LoadForTermination(ctx, raw)
	if err != nil {
		return "", storeError(err)
	}
	if !snap.Expires.After(s.now()) || snap.Credentials.RefreshUntil <= s.now().Unix() || len(snap.Credentials.CSRF) != 43 {
		return "", ErrDenied
	}
	return snap.Credentials.CSRF, nil
}
func (s *Service) Refresh(ctx context.Context, raw, csrf string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	snap, err := s.store.Load(ctx, raw)
	if err != nil {
		return storeError(err)
	}
	if !csrfMatches(snap.Credentials.CSRF, csrf) || !snap.Expires.After(s.now()) || snap.Credentials.RefreshUntil <= s.now().Unix() {
		return ErrDenied
	}
	snap, err = s.store.BeginRefresh(ctx, raw)
	if err != nil {
		return storeError(err)
	}
	if !csrfMatches(snap.Credentials.CSRF, csrf) {
		return ErrDenied
	}
	tokens, err := s.rotation.RefreshBound(ctx, snap.Credentials.RefreshToken, login.SessionIdentity{Issuer: snap.Credentials.Issuer, Subject: snap.Credentials.Subject})
	if err != nil {
		s.queueFailedRefresh(ctx, raw, snap.Credentials.Issuer, "")
		if errors.Is(err, login.ErrFailed) {
			return ErrDenied
		}
		return ErrUnavailable
	}
	if tokens.ExpiresIn <= 0 || tokens.ExpiresIn > 3600 || tokens.RefreshExpiresIn <= 0 || tokens.RefreshExpiresIn > 86400*365 || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		s.queueFailedRefresh(ctx, raw, snap.Credentials.Issuer, tokens.RefreshToken)
		return ErrUnavailable
	}
	next := snap.Credentials
	next.AccessToken = tokens.AccessToken
	next.RefreshToken = tokens.RefreshToken
	next.AccessUntil = s.now().Add(time.Duration(tokens.ExpiresIn) * time.Second).Unix()
	next.RefreshUntil = s.now().Add(time.Duration(tokens.RefreshExpiresIn) * time.Second).Unix()
	if ctx.Err() != nil || s.store.FinishRefresh(ctx, raw, snap, next) != nil {
		s.queueFailedRefresh(ctx, raw, snap.Credentials.Issuer, tokens.RefreshToken)
		return ErrUnavailable
	}
	return nil
}

// End invalidates local custody before contacting the provider. Revocation
// failure must not restore local access; provider retry/reconciliation is pending.
func (s *Service) End(ctx context.Context, raw, csrf string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	snap, err := s.store.LoadForTermination(ctx, raw)
	if err != nil {
		return storeError(err)
	}
	if !csrfMatches(snap.Credentials.CSRF, csrf) {
		return ErrDenied
	}
	job := ""
	queue, queued := s.store.(RevocationStore)
	if queued {
		job, err = queue.TerminateQueued(ctx, raw)
	} else {
		err = s.store.Terminate(ctx, raw)
	}
	if err != nil {
		return ErrUnavailable
	}
	refreshToken := snap.Credentials.RefreshToken
	if queued {
		captured, e := queue.LoadRevocation(ctx, job)
		if e != nil {
			return ErrRevocationPending
		}
		refreshToken = captured.RefreshToken
	}
	if s.rotation.RevokeNative(ctx, refreshToken) != nil || ctx.Err() != nil {
		return ErrRevocationPending
	}
	if queued && queue.CompleteRevocation(ctx, job) != nil {
		return ErrRevocationPending
	}
	return nil
}

func (s *Service) queueFailedRefresh(ctx context.Context, raw, issuer, rotated string) {
	queue, ok := s.store.(RevocationStore)
	if !ok {
		return
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, _ = queue.TerminateQueued(cleanup, raw)
	if rotated != "" {
		_, _ = queue.QueueRevocation(cleanup, issuer, rotated)
	}
	// Persistent database failure cannot manufacture durable recovery. Caller
	// still receives failure; pending-state custody remains fail-closed.
}

func storeError(err error) error {
	if errors.Is(err, postgres.ErrWebSessionMissing) {
		return ErrDenied
	}
	return ErrUnavailable
}
