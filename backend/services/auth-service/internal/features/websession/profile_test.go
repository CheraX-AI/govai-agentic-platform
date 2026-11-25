package websession

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProfileSessionIdentity(t *testing.T) {
	for _, mode := range []string{"valid-read", "valid-write", "bad-csrf", "expired-session", "expired-access", "expired-refresh", "wrong-subject", "wrong-issuer", "wrong-role", "unverified", "expired-claims", "ended", "pending-refresh", "store-failure", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			s, d, started := setup(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			claims, _ := d.Verify(ctx, "")
			claims.Email = "synthetic@example.test"
			csrf := started.CSRF
			write := mode != "valid-read"
			switch mode {
			case "valid-read":
				csrf = ""
			case "bad-csrf":
				csrf = strings.Repeat("Z", 43)
			case "expired-session":
				d.snap.Expires = time.Now().Add(-time.Minute)
			case "expired-access":
				d.snap.Credentials.AccessUntil = 0
			case "expired-refresh":
				d.snap.Credentials.RefreshUntil = 0
			case "wrong-subject":
				claims.Subject = "other"
			case "wrong-issuer":
				claims.Issuer = "https://other.test"
			case "wrong-role":
				claims.Citizen = false
			case "unverified":
				claims.Verified = false
			case "expired-claims":
				claims.Expires = time.Now().Add(-time.Minute)
			case "ended":
				d.ended = true
			case "pending-refresh":
				d.pending = true
			case "store-failure":
				d.failRead = true
			case "canceled":
				cancel()
			}
			s.verifier = forwardVerifier{claims: claims}
			identity, err := s.ProfileIdentity(ctx, started.Cookie, csrf, write)
			if mode == "valid-read" || mode == "valid-write" {
				if err != nil || identity.Subject != claims.Subject || identity.Issuer != claims.Issuer || identity.Email != claims.Email {
					t.Fatal("verified profile identity not returned")
				}
			} else if !errors.Is(err, ErrDenied) && !errors.Is(err, ErrUnavailable) {
				t.Fatal("invalid session accepted")
			}
		})
	}
}
