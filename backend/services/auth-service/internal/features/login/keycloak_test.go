package login

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "RS256", "kid": kid})
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(encoded))
	sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if e != nil {
		t.Fatal("test signing failed")
	}

	return encoded + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func jwk(key *rsa.PrivateKey, kid string) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   "AQAB",
	}
}

func baseClaims(issuer string) map[string]any {
	return map[string]any{
		"iss":            issuer,
		"sub":            "synthetic-subject",
		"aud":            []string{"govai-api"},
		"azp":            "mobile-client",
		"typ":            "Bearer",
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
		"email_verified": true,
		"realm_access":   map[string]any{"roles": []string{"citizen"}},
	}
}

type profileDouble struct {
	ready bool
	err   error
	calls int
}

func (p *profileDouble) HasReadyProfile(_ context.Context, issuer, subject string) (bool, error) {
	p.calls++
	if issuer == "" || subject != "synthetic-subject" {
		return false, errors.New("ownership mismatch")
	}

	return p.ready, p.err
}

func TestKeycloakTokenValidationAndCustody(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}

	var token string
	var calls atomic.Int32
	var keys atomic.Int32
	var issuer string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/test/protocol/openid-connect/token":
			calls.Add(1)
			if r.ParseForm() != nil ||
				r.Form.Get("username") != "citizen@example.test" ||
				r.Form.Get("password") != " space 🔒 " ||
				r.Form.Get("client_secret") != "server-only-secret" ||
				r.Form.Get("grant_type") != "password" ||
				r.Form.Get("scope") != "email" ||
				r.Header.Get("Idempotency-Key") != "" {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(
				map[string]any{
					"access_token":  token,
					"token_type":    "Bearer",
					"expires_in":    300,
					"refresh_token": "never-return-refresh",
					"id_token":      "never-return-id",
				},
			)
		case "/realms/test/protocol/openid-connect/certs":
			keys.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk(key, "key-one")}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	issuer = server.URL + "/realms/test"
	provider, e := NewKeycloak(
		KeycloakConfig{
			Issuer:       issuer,
			ClientID:     "mobile-client",
			ClientSecret: "server-only-secret",
			Audience:     "govai-api",
		},
		server.Client(),
	)
	if e != nil {
		t.Fatal(e)
	}

	defer provider.Close()
	request, _ := Parse(strings.NewReader(`{"email":"Citizen@Example.Test","password":" space 🔒 "}`))
	profiles := &profileDouble{ready: true}
	service, _ := NewService(provider, provider, profiles)
	claims := baseClaims(issuer)
	token = signToken(t, key, "key-one", claims)
	access, e := service.Login(context.Background(), request)
	if e != nil ||
		access.Token != token ||
		access.ExpiresIn <= 0 ||
		access.ExpiresIn > 300 ||
		profiles.calls != 1 {
		t.Fatal("valid login failed")
	}

	if b, _ := json.Marshal(access); bytes.Contains(b, []byte(token)) {
		t.Fatal("accidental token serialization")
	}

	for name, change := range map[string]func(map[string]any){
		"expired":           func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"wrong issuer":      func(c map[string]any) { c["iss"] = "https://wrong.example.test" },
		"wrong audience":    func(c map[string]any) { c["aud"] = []string{"account"} },
		"wrong party":       func(c map[string]any) { c["azp"] = "web-client" },
		"ID token":          func(c map[string]any) { c["typ"] = "ID" },
		"future issue":      func(c map[string]any) { c["iat"] = time.Now().Add(time.Minute).Unix() },
		"future not-before": func(c map[string]any) { c["nbf"] = time.Now().Add(time.Minute).Unix() },
		"missing subject":   func(c map[string]any) { delete(c, "sub") },
	} {
		t.Run(name, func(t *testing.T) {
			c := baseClaims(issuer)
			change(c)
			token = signToken(t, key, "key-one", c)
			if _, e := service.Login(context.Background(), request); !errors.Is(e, ErrUnavailable) {
				t.Fatal("invalid provider token accepted")
			}
		})
	}

	for _, change := range []func(map[string]any){
		func(c map[string]any) { c["email_verified"] = false },
		func(c map[string]any) { c["realm_access"] = map[string]any{"roles": []string{"admin"}} },
	} {
		c := baseClaims(issuer)
		change(c)
		token = signToken(t, key, "key-one", c)
		if _, e = service.Login(context.Background(), request); !errors.Is(e, ErrFailed) {
			t.Fatal("citizen policy bypassed")
		}
	}

	token = signToken(t, key, "key-one", baseClaims(issuer))
	profiles.ready = false
	if _, e = service.Login(context.Background(), request); !errors.Is(e, ErrFailed) {
		t.Fatal("missing profile accepted")
	}

	profiles.err = errors.New("private SQL details")
	if _, e = service.Login(context.Background(), request); !errors.Is(e, ErrUnavailable) {
		t.Fatal("database outage not handled")
	}

	if keys.Load() != 1 {
		t.Fatal("JWKS cache not reused")
	}

	if calls.Load() != 13 {
		t.Fatal("credential call count indicates retry or missing test path")
	}
}

func TestProviderErrorsNoRetryOrRedirect(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{
			"invalid grant",
			400,
			`{"error":"invalid_grant","error_description":"private reason"}`,
			ErrFailed,
		},
		{
			"invalid client",
			401,
			`{"error":"invalid_client","error_description":"private secret"}`,
			ErrUnavailable,
		},
		{"malformed success", 200, `{"access_token":"not-jwt"}`, ErrUnavailable},
		{"oversize", 200, strings.Repeat("x", (64<<10)+1), ErrUnavailable},
		{"redirect", 302, ``, ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "https://example.test/must-not-follow")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			p, e := NewKeycloak(
				KeycloakConfig{
					Issuer:       server.URL + "/realms/test",
					ClientID:     "mobile",
					ClientSecret: "secret",
					Audience:     "govai-api",
				},
				server.Client(),
			)
			if e != nil {
				t.Fatal(e)
			}

			defer p.Close()
			_, e = p.Password(context.Background(), Request{email: "citizen@example.test", password: "password"})
			if !errors.Is(e, test.want) || calls.Load() != 1 {
				t.Fatal("unsafe error classification/retry")
			}
		})
	}
}

func TestLoginTLSAndCancellation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer server.Close()
	cfg := KeycloakConfig{
		Issuer:       server.URL + "/realms/test",
		ClientID:     "mobile",
		ClientSecret: "secret",
		Audience:     "govai-api",
	}
	p, e := NewKeycloak(cfg, http.DefaultClient)
	if e != nil {
		t.Fatal(e)
	}

	defer p.Close()
	if _, e = p.Password(context.Background(), Request{email: "a@example.test", password: "p"}); !errors.Is(e, ErrUnavailable) {
		t.Fatal("untrusted TLS accepted")
	}

	insecure := server.Client()
	insecure.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if _, e = NewKeycloak(cfg, insecure); e == nil {
		t.Fatal("TLS bypass accepted")
	}

	cfg.Issuer = strings.Replace(cfg.Issuer, "https://", "http://", 1)
	if _, e = NewKeycloak(cfg, server.Client()); e == nil {
		t.Fatal("HTTP configuration accepted")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = p.Password(ctx, Request{email: "a@example.test", password: "p"}); !errors.Is(e, ErrUnavailable) {
		t.Fatal("cancellation ignored")
	}
}
