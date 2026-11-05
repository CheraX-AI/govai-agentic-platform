package login

import (
	"context"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

type Access struct {
	privateValue
	Token            string
	ExpiresIn        int64
	RefreshToken     string
	RefreshExpiresIn int64
}

type Issued struct {
	privateValue
	Token            string
	ExpiresIn        int64
	RefreshToken     string
	RefreshExpiresIn int64
}

type Claims struct {
	privateValue
	Issuer, Subject   string
	Expires           time.Time
	Verified, Citizen bool
}

type PasswordProvider interface {
	Password(context.Context, Request) (Issued, error)
}

type AccessVerifier interface {
	Verify(context.Context, string) (Claims, error)
}

type ProfileLookup interface {
	HasReadyProfile(context.Context, string, string) (bool, error)
}

type NICAccount struct {
	privateValue
	Email, Issuer, Subject string
}

type IdentifierResolver interface {
	ResolveNIC(context.Context, registration.NIC) (NICAccount, error)
}

type Service struct {
	resolver IdentifierResolver
	privateValue
	provider PasswordProvider
	verifier AccessVerifier
	profiles ProfileLookup
	now      func() time.Time
}

// NewService connects password login, token verification and profile lookup.
func NewService(
	p PasswordProvider,
	v AccessVerifier,
	profiles ProfileLookup,
	resolvers ...IdentifierResolver,
) (*Service, error) {
	if p == nil || v == nil || profiles == nil {
		return nil, ErrUnavailable
	}

	if len(resolvers) > 1 {
		return nil, ErrUnavailable
	}

	var resolver IdentifierResolver
	if len(resolvers) == 1 {
		resolver = resolvers[0]
	}

	return &Service{provider: p, verifier: v, profiles: profiles, resolver: resolver, now: time.Now}, nil
}

// Login verifies provider credentials and the linked application profile.
func (s *Service) Login(ctx context.Context, r Request) (Access, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var bound NICAccount
	if r.isNIC {
		if s.resolver == nil {
			return Access{}, ErrFailed
		}

		var err error
		bound, err = s.resolver.ResolveNIC(ctx, r.nic)
		if err != nil {
			if errors.Is(err, ErrFailed) {
				return Access{}, ErrFailed
			}

			return Access{}, ErrUnavailable
		}

		if bound.Email == "" ||
			registration.CanonicalEmail(bound.Email) == "" ||
			bound.Issuer == "" ||
			bound.Subject == "" {
			return Access{}, ErrFailed
		}

		r.email = bound.Email
	}

	issued, e := s.provider.Password(ctx, r)
	if e != nil {
		return Access{}, e
	}

	claims, e := s.verifier.Verify(ctx, issued.Token)
	if e != nil {
		return Access{}, e
	}

	if r.isNIC && (claims.Issuer != bound.Issuer || claims.Subject != bound.Subject) {
		return Access{}, ErrFailed
	}

	if claims.Issuer == "" || claims.Subject == "" || !claims.Verified || !claims.Citizen {
		return Access{}, ErrFailed
	}

	ready, e := s.profiles.HasReadyProfile(ctx, claims.Issuer, claims.Subject)
	if e != nil {
		return Access{}, ErrUnavailable
	}

	if !ready {
		return Access{}, ErrFailed
	}

	remaining := int64(claims.Expires.Sub(s.now()).Seconds())
	if remaining > issued.ExpiresIn {
		remaining = issued.ExpiresIn
	}

	if remaining <= 0 || ctx.Err() != nil {
		return Access{}, ErrUnavailable
	}

	return Access{
		Token:            issued.Token,
		ExpiresIn:        remaining,
		RefreshToken:     issued.RefreshToken,
		RefreshExpiresIn: issued.RefreshExpiresIn,
	}, nil
}
