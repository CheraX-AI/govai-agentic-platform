package login

import (
	"context"
	"errors"
	"time"
)

type SessionProvider interface {
	Refresh(context.Context, string) (SessionTokens, error)
	RevokeRefresh(context.Context, string) error
}

// SessionIdentity is a trusted server-side binding, never a request body field.
type SessionIdentity struct {
	privateValue
	Issuer, Subject string
}

type SessionService struct {
	privateValue
	provider SessionProvider
	verifier AccessVerifier
	profiles ProfileLookup
	now      func() time.Time
}

// NewSessionService connects refresh handling to identity and profile checks.
func NewSessionService(p SessionProvider, v AccessVerifier, profiles ProfileLookup) (*SessionService, error) {
	if p == nil || v == nil || profiles == nil {
		return nil, ErrUnavailable
	}

	return &SessionService{provider: p, verifier: v, profiles: profiles, now: time.Now}, nil
}

// RefreshNative accepts a client-held bearer refresh credential. The fixed
// provider validates its client/session binding; caller-supplied identities
// must not be substituted for that check.
func (s *SessionService) RefreshNative(ctx context.Context, raw string) (SessionTokens, error) {
	return s.refresh(ctx, raw, nil)
}

// RefreshBound additionally pins a web session to its original identity.
// Web orchestration must serialize refreshes and persist the returned tokens
// atomically before emitting a successful response.
func (s *SessionService) RefreshBound(ctx context.Context, raw string, owner SessionIdentity) (SessionTokens, error) {
	if owner.Issuer == "" || owner.Subject == "" {
		return SessionTokens{}, ErrInvalid
	}

	return s.refresh(ctx, raw, &owner)
}

func (s *SessionService) refresh(ctx context.Context, raw string, owner *SessionIdentity) (SessionTokens, error) {
	if !validRefresh(raw) {
		return SessionTokens{}, ErrInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		return SessionTokens{}, ErrUnavailable
	}

	issued, err := s.provider.Refresh(ctx, raw)
	if err != nil {
		return SessionTokens{}, safeSessionError(err)
	}

	if !validRefresh(issued.AccessToken) ||
		!validRefresh(issued.RefreshToken) ||
		issued.ExpiresIn <= 0 ||
		issued.ExpiresIn > 3600 ||
		issued.RefreshExpiresIn <= 0 {
		return SessionTokens{}, ErrUnavailable
	}

	claims, err := s.verifier.Verify(ctx, issued.AccessToken)
	if err != nil {
		return SessionTokens{}, ErrUnavailable
	}

	if claims.Issuer == "" || claims.Subject == "" || !claims.Verified || !claims.Citizen {
		return SessionTokens{}, ErrFailed
	}

	if owner != nil && (owner.Issuer != claims.Issuer || owner.Subject != claims.Subject) {
		return SessionTokens{}, ErrFailed
	}

	remaining := int64(claims.Expires.Sub(s.now()).Seconds())
	if remaining <= 0 {
		return SessionTokens{}, ErrFailed
	}

	ready, err := s.profiles.HasReadyProfile(ctx, claims.Issuer, claims.Subject)
	if err != nil {
		return SessionTokens{}, ErrUnavailable
	}

	if !ready {
		return SessionTokens{}, ErrFailed
	}

	if ctx.Err() != nil {
		return SessionTokens{}, ErrUnavailable
	}

	remaining = int64(claims.Expires.Sub(s.now()).Seconds())
	if remaining <= 0 {
		return SessionTokens{}, ErrFailed
	}

	if remaining < issued.ExpiresIn {
		issued.ExpiresIn = remaining
	}

	return issued, nil
}

// RevokeNative revokes a client-held credential. It does not claim global
// logout, clear a client device or invalidate offline access JWTs immediately.
func (s *SessionService) RevokeNative(ctx context.Context, raw string) error {
	if !validRefresh(raw) {
		return ErrInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		return ErrUnavailable
	}

	err := s.provider.RevokeRefresh(ctx, raw)
	if ctx.Err() != nil {
		return ErrUnavailable
	}

	if err != nil {
		return safeSessionError(err)
	}

	return nil
}

func safeSessionError(err error) error {
	if errors.Is(err, ErrFailed) {
		return ErrFailed
	}

	return ErrUnavailable
}
