package login

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func TestIdentifierInput(t *testing.T) {
	for _, tc := range []struct {
		value, key string
		nic        bool
	}{
		{"Citizen@Example.Test", "citizen@example.test", false},
		{"123456789v", "nic:123456789V", true},
		{"123456789x", "nic:123456789X", true},
		{"200000000001", "nic:200000000001", true},
	} {
		b, _ := json.Marshal(map[string]string{"identifier": tc.value, "password": "  private-password  "})
		r, err := Parse(strings.NewReader(string(b)))
		if err != nil || r.LimitKey() != tc.key || r.isNIC != tc.nic || r.password != "  private-password  " {
			t.Fatal("identifier normalization failed")
		}

		encoded, _ := json.Marshal(r)
		if strings.Contains(fmt.Sprintf("%+v", r), tc.value) || strings.Contains(string(encoded), tc.value) {
			t.Fatal("identifier disclosure")
		}
	}

	for _, b := range []string{
		`{"identifier":"+94771234567","password":"x"}`,
		`{"identifier":"123456789V ","password":"x"}`,
		`{"email":"123456789V","password":"x"}`,
		`{"identifier":"123456789V","email":"a@example.test","password":"x"}`,
		`{"identifier":"123456789V","identifier":"200000000001","password":"x"}`,
		`{"identifier":null,"password":"x"}`,
	} {
		if _, err := Parse(strings.NewReader(b)); err == nil {
			t.Fatal("ambiguous or unsupported identifier accepted")
		}
	}
}

type identifierResolverDouble struct {
	account    NICAccount
	err        error
	calls      int
	normalized string
}

func (r *identifierResolverDouble) ResolveNIC(ctx context.Context, n registration.NIC) (NICAccount, error) {
	r.calls++
	r.normalized = n.NormalizedNumber()
	return r.account, r.err
}

type identifierProviderDouble struct {
	calls           int
	email, password string
	err             error
}

func (p *identifierProviderDouble) Password(ctx context.Context, r Request) (Issued, error) {
	p.calls++
	p.email = r.email
	p.password = r.password
	return Issued{Token: "synthetic-access", ExpiresIn: 60}, p.err
}

type identifierVerifierDouble struct{ claims Claims }

func (v identifierVerifierDouble) Verify(context.Context, string) (Claims, error) {
	return v.claims, nil
}

func TestNICLoginBinding(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		resolveErr, providerErr error
		subject                 string
		want                    error
		calls                   int
	}{
		{name: "success", subject: "synthetic-subject", calls: 1},
		{name: "unknown NIC", resolveErr: ErrFailed, want: ErrFailed},
		{
			name:       "lookup unavailable",
			resolveErr: errors.New("private database details"),
			want:       ErrUnavailable,
		},
		{
			name:        "wrong password",
			subject:     "synthetic-subject",
			providerErr: ErrFailed,
			want:        ErrFailed,
			calls:       1,
		},
		{name: "wrong account token", subject: "other-subject", want: ErrFailed, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &identifierResolverDouble{
				account: NICAccount{
					Email:   "citizen@example.test",
					Issuer:  "https://issuer.test",
					Subject: "synthetic-subject",
				},
				err: tc.resolveErr,
			}
			provider := &identifierProviderDouble{err: tc.providerErr}
			claims := Claims{
				Issuer:   "https://issuer.test",
				Subject:  tc.subject,
				Verified: true,
				Citizen:  true,
				Expires:  time.Now().Add(time.Minute),
			}
			profiles := &profileDouble{ready: true}
			service, err := NewService(provider, identifierVerifierDouble{claims}, profiles, resolver)
			if err != nil {
				t.Fatal(err)
			}

			request, err := Parse(strings.NewReader(`{"identifier":"123456789v","password":"private-password"}`))
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Login(context.Background(), request)
			if !errors.Is(err, tc.want) ||
				resolver.calls != 1 ||
				resolver.normalized != "123456789V" ||
				provider.calls != tc.calls {
				t.Fatal("NIC binding result mismatch")
			}

			if tc.calls == 1 &&
				(provider.email != "citizen@example.test" ||
					provider.password != "private-password") {
				t.Fatal("provider received incorrect credentials")
			}

			if tc.want != nil && profiles.calls != 0 {
				t.Fatal("failed identity reached profile authorization")
			}
		})
	}
}

func TestEmailLoginBypassesNICResolver(t *testing.T) {
	resolver := &identifierResolverDouble{err: ErrUnavailable}
	provider := &identifierProviderDouble{}
	service, _ := NewService(
		provider,
		identifierVerifierDouble{
			Claims{
				Issuer:   "https://issuer.test",
				Subject:  "synthetic-subject",
				Verified: true,
				Citizen:  true,
				Expires:  time.Now().Add(time.Minute),
			},
		},
		&profileDouble{ready: true},
		resolver,
	)
	for _, body := range []string{
		`{"email":"Citizen@Example.Test","password":"x"}`,
		`{"identifier":"Citizen@Example.Test","password":"x"}`,
	} {
		request, err := Parse(strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		if _, err = service.Login(context.Background(), request); err != nil ||
			resolver.calls != 0 ||
			provider.email != "citizen@example.test" {
			t.Fatal("email compatibility failed")
		}
	}
}
