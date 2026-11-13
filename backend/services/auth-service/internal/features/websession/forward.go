package websession

import (
	"context"
	"time"
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

	if ctx.Err() != nil {
		return ErrUnavailable
	}

	check, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	snap, err := s.store.Load(check, raw)
	if err != nil {
		return storeError(err)
	}

	c := snap.Credentials
	if !csrfMatches(c.CSRF, csrf) ||
		!snap.Expires.After(s.now()) ||
		c.AccessUntil <= s.now().Unix() ||
		c.RefreshUntil <= s.now().Unix() ||
		c.AccessToken == "" {
		return ErrDenied
	}

	claims, err := s.verifier.Verify(check, c.AccessToken)
	if err != nil {
		return ErrDenied
	}

	if claims.Issuer != c.Issuer ||
		claims.Subject != c.Subject ||
		!claims.Verified ||
		!claims.Citizen ||
		!claims.Expires.After(s.now()) {
		return ErrDenied
	}

	ready, err := s.profiles.HasReadyProfile(check, c.Issuer, c.Subject)
	if err != nil || check.Err() != nil {
		return ErrUnavailable
	}

	if !ready {
		return ErrDenied
	}
	// The caller owns the upstream deadline. Do not give it the short lookup budget.
	return forward(ctx, c.AccessToken)
}
