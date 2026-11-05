package authn

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "RS256", "kid": kid})
	b, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(encoded))
	signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if e != nil {
		t.Fatal(e)
	}

	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func keyJSON(key *rsa.PrivateKey, kid string) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   "AQAB",
	}
}

func claims(issuer string, now time.Time) map[string]any {
	return map[string]any{
		"iss":            issuer,
		"sub":            "synthetic-subject",
		"aud":            []string{"govai-api"},
		"azp":            "mobile",
		"typ":            "Bearer",
		"iat":            now.Unix(),
		"exp":            now.Add(5 * time.Minute).Unix(),
		"email_verified": true,
		"realm_access":   map[string]any{"roles": []string{"citizen"}},
	}
}

func fixture(t *testing.T) (*Verifier, *rsa.PrivateKey, *atomic.Int32, *atomic.Int64, string) {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}

	count := &atomic.Int32{}
	clock := &atomic.Int64{}
	clock.Store(time.Now().Unix())
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{keyJSON(key, "one")}})
	}))
	t.Cleanup(server.Close)
	issuer := server.URL + "/realms/test"
	v, e := New(
		Config{
			Issuer:   issuer,
			Audience: "govai-api",
			Clients:  []string{"mobile"},
			Now:      func() time.Time { return time.Unix(clock.Load(), 0) },
		},
		server.Client(),
	)
	if e != nil {
		t.Fatal(e)
	}

	t.Cleanup(v.Close)
	return v, key, count, clock, issuer
}

func TestVerifierClaimsAndForgery(t *testing.T) {
	v, key, count, clock, issuer := fixture(t)
	now := time.Unix(clock.Load(), 0)
	identity, e := v.Verify(context.Background(), sign(t, key, "one", claims(issuer, now)))
	if e != nil || identity.Subject != "synthetic-subject" || !identity.EmailVerified {
		t.Fatal("valid access denied")
	}

	cases := map[string]func(map[string]any){
		"wrong issuer":    func(c map[string]any) { c["iss"] = "https://other.example.test/realm" },
		"wrong audience":  func(c map[string]any) { c["aud"] = []string{"account"} },
		"wrong client":    func(c map[string]any) { c["azp"] = "other" },
		"ID token":        func(c map[string]any) { c["typ"] = "ID" },
		"expired":         func(c map[string]any) { c["exp"] = now.Add(-time.Second).Unix() },
		"future iat":      func(c map[string]any) { c["iat"] = now.Add(time.Minute).Unix() },
		"future nbf":      func(c map[string]any) { c["nbf"] = now.Add(time.Minute).Unix() },
		"missing subject": func(c map[string]any) { delete(c, "sub") },
		"excess lifetime": func(c map[string]any) { c["exp"] = now.Add(2 * time.Hour).Unix() },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			c := claims(issuer, now)
			change(c)
			if _, e := v.Verify(context.Background(), sign(t, key, "one", c)); e == nil {
				t.Fatal("invalid claims accepted")
			}
		})
	}

	forged, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}

	if _, e = v.Verify(context.Background(), sign(t, forged, "one", claims(issuer, now))); e == nil {
		t.Fatal("forged signature accepted")
	}

	raw := sign(t, key, "one", claims(issuer, now))
	unsigned := "eyJhbGciOiJub25lIn0." + strings.Split(raw, ".")[1] + "."
	if _, e = v.Verify(context.Background(), unsigned); e == nil {
		t.Fatal("unsigned token accepted")
	}

	for i := 0; i < 8; i++ {
		_, _ = v.Verify(context.Background(), sign(t, forged, "unknown", claims(issuer, now)))
	}

	if count.Load() != 1 {
		t.Fatal("unknown-kid refresh cooldown failed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = v.Verify(ctx, raw); e == nil {
		t.Fatal("canceled verification accepted")
	}
}

func TestMiddlewareAuthorizationAndIdentity(t *testing.T) {
	v, key, _, clock, issuer := fixture(t)
	previous := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(previous)
	router := gin.New()
	reached := 0
	router.GET("/protected",
		v.Authenticate(),
		RequireVerifiedEmail(),
		RequireRole("citizen"),
		func(c *gin.Context) {
			identity, ok := Current(c)
			if !ok {
				t.Fatal("missing validated identity")
			}

			identity.Roles[0] = "changed"
			again, _ := Current(c)
			if again.Roles[0] != "citizen" {
				t.Fatal("mutable role context leaked")
			}

			reached++
			c.JSON(200, gin.H{"ok": true})
		})
	now := time.Unix(clock.Load(), 0)
	for _, test := range []struct {
		name   string
		change func(map[string]any)
		header string
		tls    bool
		want   int
	}{
		{name: "missing", tls: true, want: 401},
		{name: "malformed", header: "Basic private", tls: true, want: 401},
		{name: "valid", header: "TOKEN", tls: true, want: 200},
		{
			name:   "unverified",
			header: "TOKEN",
			tls:    true,
			want:   403,
			change: func(c map[string]any) { c["email_verified"] = false },
		},
		{
			name:   "wrong role",
			header: "TOKEN",
			tls:    true,
			want:   403,
			change: func(c map[string]any) { c["realm_access"] = map[string]any{"roles": []string{"staff"}} },
		},
		{
			name:   "expired",
			header: "TOKEN",
			tls:    true,
			want:   401,
			change: func(c map[string]any) { c["exp"] = now.Add(-time.Minute).Unix() },
		},
		{name: "HTTP", header: "TOKEN", want: 426},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := claims(issuer, now)
			if test.change != nil {
				test.change(c)
			}

			header := test.header
			if header == "TOKEN" {
				header = "Bearer " + sign(t, key, "one", c)
			}

			req := httptest.NewRequest("GET", "/protected?access_token=not-accepted", nil)
			if test.tls {
				req.TLS = &tls.ConnectionState{}
			}

			if header != "" {
				req.Header.Set("Authorization", header)
			}

			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-User-Role", "citizen")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != test.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d", w.Code)
			}

			if test.want == 401 && w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing challenge")
			}
		})
	}

	if reached != 1 {
		t.Fatal("unauthorized request reached protected handler")
	}

	req := httptest.NewRequest("GET", "/protected", nil)
	req.TLS = &tls.ConnectionState{}
	req.Header.Add("Authorization", "Bearer one")
	req.Header.Add("Authorization", "Bearer two")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("multiple headers accepted")
	}
}

func TestConfigAndTLSGuards(t *testing.T) {
	server := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"keys":[]}`) }),
	)
	defer server.Close()
	c := Config{Issuer: server.URL + "/realms/test", Audience: "govai-api", Clients: []string{"mobile"}}
	for _, issuer := range []string{
		"http://localhost/realms/test",
		server.URL + "/realms/test/",
		server.URL + "/realms/test?keys=evil",
	} {
		copy := c
		copy.Issuer = issuer
		if _, e := New(copy, server.Client()); e == nil {
			t.Fatal("unsafe issuer accepted")
		}
	}

	client := server.Client()
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if _, e := New(c, client); e == nil {
		t.Fatal("TLS bypass accepted")
	}
}

func TestRotationAfterRefreshInterval(t *testing.T) {
	one, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}

	two, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}

	var rotate atomic.Bool
	var count atomic.Int32
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		key, kid := one, "one"
		if rotate.Load() {
			key, kid = two, "two"
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{keyJSON(key, kid)}})
	}))
	defer server.Close()
	issuer := server.URL + "/realms/test"
	v, e := New(
		Config{
			Issuer:   issuer,
			Audience: "govai-api",
			Clients:  []string{"mobile"},
			Now:      func() time.Time { return time.Unix(clock.Load(), 0) },
		},
		server.Client(),
	)
	if e != nil {
		t.Fatal(e)
	}

	defer v.Close()
	if _, e = v.Verify(context.Background(), sign(t, one, "one", claims(issuer, time.Unix(clock.Load(), 0)))); e != nil {
		t.Fatal(e)
	}

	rotate.Store(true)
	clock.Add(6)
	if _, e = v.Verify(context.Background(), sign(t, two, "two", claims(issuer, time.Unix(clock.Load(), 0)))); e != nil ||

		count.Load() != 2 {
		t.Fatal("rotated key not refreshed")
	}
}
