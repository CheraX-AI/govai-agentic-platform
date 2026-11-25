package websession

import (
	"context"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
)

// WithAccess is an internal server-side forwarding boundary. The callback must
// never log, persist or return the access token to a browser. It is not a public
// HTTP API. The guarded store applies logout and password-recovery fences.
// Authorization is checked when forwarding begins; this cannot recall an
// upstream request if logout or recovery happens after that point.
func (s *Service) WithAccess(ctx context.Context, raw, csrf string, forward func(context.Context, string) error) error {
	if forward == nil {
		return ErrUnavailable
	}

	_, token, err := s.checkedAccess(ctx, raw, csrf, true)
	if err != nil {
		return err
	}
	return forward(ctx, token)
}

// ProfileIdentity reuses the same verified, fenced session boundary as the
// existing chat gateway, without ever releasing provider tokens to a browser.
func (s *Service) ProfileIdentity(ctx context.Context, raw, csrf string, write bool) (profile.Caller, error) {
	claims, _, err := s.checkedAccess(ctx, raw, csrf, write)
	if err != nil {
		return profile.Caller{}, err
	}
	return profile.Caller{Issuer: claims.Issuer, Subject: claims.Subject, Email: claims.Email}, nil
}

func (s *Service) checkedAccess(ctx context.Context, raw, csrf string, requireCSRF bool) (login.Claims, string, error) {
	if ctx.Err() != nil {
		return login.Claims{}, "", ErrUnavailable
	}

	check, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	snap, err := s.store.Load(check, raw)
	if err != nil {
		return login.Claims{}, "", storeError(err)
	}

	c := snap.Credentials
	if (requireCSRF && !csrfMatches(c.CSRF, csrf)) ||
		!snap.Expires.After(s.now()) ||
		c.AccessUntil <= s.now().Unix() ||
		c.RefreshUntil <= s.now().Unix() ||
		c.AccessToken == "" {
		return login.Claims{}, "", ErrDenied
	}

	claims, err := s.verifier.Verify(check, c.AccessToken)
	if err != nil {
		return login.Claims{}, "", ErrDenied
	}

	if claims.Issuer != c.Issuer ||
		claims.Subject != c.Subject ||
		!claims.Verified ||
		!claims.Citizen ||
		!claims.Expires.After(s.now()) {
		return login.Claims{}, "", ErrDenied
	}

	ready, err := s.profiles.HasReadyProfile(check, c.Issuer, c.Subject)
	if err != nil || check.Err() != nil {
		return login.Claims{}, "", ErrUnavailable
	}

	if !ready {
		return login.Claims{}, "", ErrDenied
	}
	return claims, c.AccessToken, nil
}
