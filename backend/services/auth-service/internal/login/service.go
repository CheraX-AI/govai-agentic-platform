package login

import (
	"context"
	"time"
)

type Access struct {
	privateValue
	Token     string
	ExpiresIn int64
}
type Issued struct {
	privateValue
	Token     string
	ExpiresIn int64
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
type Service struct {
	privateValue
	provider PasswordProvider
	verifier AccessVerifier
	profiles ProfileLookup
	now      func() time.Time
}

func NewService(p PasswordProvider, v AccessVerifier, profiles ProfileLookup) (*Service, error) {
	if p == nil || v == nil || profiles == nil {
		return nil, ErrUnavailable
	}
	return &Service{provider: p, verifier: v, profiles: profiles, now: time.Now}, nil
}
func (s *Service) Login(ctx context.Context, r Request) (Access, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	issued, e := s.provider.Password(ctx, r)
	if e != nil {
		return Access{}, e
	}
	claims, e := s.verifier.Verify(ctx, issued.Token)
	if e != nil {
		return Access{}, e
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
	return Access{Token: issued.Token, ExpiresIn: remaining}, nil
}
