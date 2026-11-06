package login

import (
	"context"
	"errors"
	"testing"
	"time"
)

type sessionProviderDouble struct {
	issued             SessionTokens
	err                error
	refreshes, revokes int
}

func (p *sessionProviderDouble) Refresh(context.Context, string) (SessionTokens, error) {
	p.refreshes++
	return p.issued, p.err
}
func (p *sessionProviderDouble) RevokeRefresh(context.Context, string) error {
	p.revokes++
	return p.err
}

type sessionVerifierDouble struct {
	claims Claims
	err    error
	calls  int
}

func (v *sessionVerifierDouble) Verify(context.Context, string) (Claims, error) {
	v.calls++
	return v.claims, v.err
}

func TestSessionRefreshPolicies(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, name := range []string{"success", "provider rejected", "provider private error", "invalid JWT", "unverified", "wrong role", "wrong subject", "wrong issuer", "expired", "profile missing", "database error", "invalid token result", "cancelled", "invalid input"} {
		t.Run(name, func(t *testing.T) {
			p := &sessionProviderDouble{issued: SessionTokens{AccessToken: "synthetic-access", RefreshToken: "synthetic-next", ExpiresIn: 300, RefreshExpiresIn: 1800}}
			v := &sessionVerifierDouble{claims: Claims{Issuer: "https://issuer.test", Subject: "synthetic-subject", Expires: now.Add(90 * time.Second), Verified: true, Citizen: true}}
			profiles := &profileDouble{ready: true}
			s, err := NewSessionService(p, v, profiles)
			if err != nil {
				t.Fatal("setup")
			}
			s.now = func() time.Time { return now }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := error(nil)
			raw := "synthetic-old"
			wantDB := 0
			switch name {
			case "provider rejected":
				p.err = ErrFailed
				want = ErrFailed
			case "provider private error":
				p.err = errors.New("private provider detail")
				want = ErrUnavailable
			case "invalid JWT":
				v.err = errors.New("private verification detail")
				want = ErrUnavailable
			case "unverified":
				v.claims.Verified = false
				want = ErrFailed
			case "wrong role":
				v.claims.Citizen = false
				want = ErrFailed
			case "wrong subject":
				v.claims.Subject = "other"
				want = ErrFailed
			case "wrong issuer":
				v.claims.Issuer = "https://other.test"
				want = ErrFailed
			case "expired":
				v.claims.Expires = now
				want = ErrFailed
			case "profile missing":
				profiles.ready = false
				want = ErrFailed
				wantDB = 1
			case "database error":
				profiles.err = errors.New("private SQL detail")
				want = ErrUnavailable
				wantDB = 1
			case "invalid token result":
				p.issued.RefreshToken = ""
				want = ErrUnavailable
			case "cancelled":
				cancel()
				want = ErrUnavailable
			case "invalid input":
				raw = "a b"
				want = ErrInvalid
			default:
				wantDB = 1
			}
			got, err := s.RefreshBound(ctx, raw, SessionIdentity{Issuer: "https://issuer.test", Subject: "synthetic-subject"})
			if err != want {
				t.Fatalf("wrong safe error: %v", err)
			}
			if profiles.calls != wantDB {
				t.Fatal("database called before authentication policy")
			}
			if want != nil && (got.AccessToken != "" || got.RefreshToken != "") {
				t.Fatal("tokens leaked on failure")
			}
			if want == nil && (got.ExpiresIn != 90 || got.RefreshToken != "synthetic-next") {
				t.Fatal("incorrect token custody/lifetime")
			}
			if p.refreshes > 1 || ((name == "cancelled" || name == "invalid input") && p.refreshes != 0) {
				t.Fatal("unexpected provider call")
			}
		})
	}
}

func TestSessionNativeAndRevocation(t *testing.T) {
	p := &sessionProviderDouble{issued: SessionTokens{AccessToken: "synthetic-access", RefreshToken: "synthetic-next", ExpiresIn: 300, RefreshExpiresIn: 1800}}
	v := &sessionVerifierDouble{claims: Claims{Issuer: "https://issuer.test", Subject: "synthetic-subject", Expires: time.Now().Add(time.Minute), Verified: true, Citizen: true}}
	s, _ := NewSessionService(p, v, &profileDouble{ready: true})
	if _, err := s.RefreshNative(context.Background(), "synthetic-old"); err != nil {
		t.Fatal("native refresh failed")
	}
	if _, err := s.RefreshBound(context.Background(), "synthetic-old", SessionIdentity{}); err != ErrInvalid {
		t.Fatal("empty web binding accepted")
	}
	if err := s.RevokeNative(context.Background(), "synthetic-old"); err != nil || p.revokes != 1 {
		t.Fatal("revocation failed")
	}
	p.err = errors.New("private failure")
	if err := s.RevokeNative(context.Background(), "synthetic-old"); err != ErrUnavailable {
		t.Fatal("unsafe provider error")
	}
	if _, err := NewSessionService(nil, v, &profileDouble{}); err != ErrUnavailable {
		t.Fatal("nil dependency accepted")
	}
}
