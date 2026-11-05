package websession

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
)

type forwardVerifier struct {
	claims login.Claims
	err    error
}

func (v forwardVerifier) Verify(context.Context, string) (login.Claims, error) {
	return v.claims, v.err
}

type forwardProfile struct {
	ready bool
	err   error
}

func (p forwardProfile) HasReadyProfile(context.Context, string, string) (bool, error) {
	return p.ready, p.err
}

func TestForwardAuthorization(t *testing.T) {
	for _, name := range []string{
		"valid",
		"bad csrf",
		"expired session",
		"expired access",
		"expired refresh",
		"ended",
		"pending refresh",
		"database outage",
		"wrong subject",
		"wrong issuer",
		"unverified",
		"wrong role",
		"expired claims",
		"verification error",
		"missing profile",
		"profile outage",
		"canceled",
	} {
		t.Run(name, func(t *testing.T) {
			s, d, started := setup(t)
			csrf := started.CSRF
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			claims, _ := d.Verify(ctx, "")
			switch name {
			case "bad csrf":
				csrf = strings.Repeat("Z", 43)
			case "expired session":
				d.snap.Expires = time.Now().Add(-time.Minute)
			case "expired access":
				d.snap.Credentials.AccessUntil = 0
			case "expired refresh":
				d.snap.Credentials.RefreshUntil = 0
			case "ended":
				d.ended = true
			case "pending refresh":
				d.pending = true
			case "database outage":
				d.failRead = true
			case "wrong subject":
				claims.Subject = "other-subject"
			case "wrong issuer":
				claims.Issuer = "https://other.test"
			case "unverified":
				claims.Verified = false
			case "wrong role":
				claims.Citizen = false
			case "expired claims":
				claims.Expires = time.Now().Add(-time.Minute)
			case "missing profile":
				s.profiles = forwardProfile{}
			case "profile outage":
				s.profiles = forwardProfile{err: errors.New("private SQL")}
			case "canceled":
				cancel()
			}

			s.verifier = forwardVerifier{claims: claims}
			if name == "verification error" {
				s.verifier = forwardVerifier{err: errors.New("private verifier failure")}
			}

			calls := 0
			sentinel := errors.New("upstream sentinel")
			err := s.WithAccess(ctx, started.Cookie, csrf, func(got context.Context, token string) error {
				calls++
				if got != ctx || token != "synthetic-access" {
					t.Fatal("forwarding boundary changed context or token")
				}

				return sentinel
			})
			if name == "valid" {
				if calls != 1 || err != sentinel {
					t.Fatal("authorized forwarding did not preserve outcome")
				}

				return
			}

			if calls != 0 || (err != ErrDenied && err != ErrUnavailable) {
				t.Fatal("unauthorized or unavailable session reached upstream")
			}
		})
	}
}

func TestForwardMissingCallback(t *testing.T) {
	s, _, started := setup(t)
	if s.WithAccess(context.Background(), started.Cookie, started.CSRF, nil) != ErrUnavailable {
		t.Fatal("nil callback accepted")
	}
}
