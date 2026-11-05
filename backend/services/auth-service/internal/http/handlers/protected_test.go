package handler

import (
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
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
)

type protectedProfiles struct {
	ready  bool
	err    error
	calls  int
	issuer string
}

func (p *protectedProfiles) HasReadyProfile(_ context.Context, issuer, subject string) (bool, error) {
	p.calls++
	if issuer != p.issuer || subject != "synthetic-subject" {
		return false, errors.New("unexpected ownership")
	}

	return p.ready, p.err
}

func TestProtectedProfileAndChatBoundaries(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}

	keys := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(
			map[string]any{
				"keys": []any{
					map[string]string{
						"kty": "RSA",
						"kid": "one",
						"use": "sig",
						"alg": "RS256",
						"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
						"e":   "AQAB",
					},
				},
			},
		)
	}))
	defer keys.Close()
	issuer := keys.URL + "/realms/test"
	verifier, e := authn.New(
		authn.Config{Issuer: issuer, Audience: "govai-api", Clients: []string{"mobile"}},
		keys.Client(),
	)
	if e != nil {
		t.Fatal(e)
	}

	defer verifier.Close()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "one", "typ": "JWT"})
	body, _ := json.Marshal(
		map[string]any{
			"iss":            issuer,
			"aud":            []string{"govai-api"},
			"sub":            "synthetic-subject",
			"azp":            "mobile",
			"typ":            "Bearer",
			"iat":            time.Now().Unix(),
			"exp":            time.Now().Add(5 * time.Minute).Unix(),
			"email_verified": true,
			"realm_access":   map[string]any{"roles": []string{"citizen"}},
		},
	)
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(encoded))
	signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if e != nil {
		t.Fatal(e)
	}

	token := encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
	profiles := &protectedProfiles{ready: true, issuer: issuer}
	p, e := NewProtected(verifier, profiles)
	if e != nil {
		t.Fatal(e)
	}

	router := NewRouterWithProtected(nil, nil, p, "https://web.example.test")
	request := func(method, path, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.TLS = &tls.ConnectionState{}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/v1/me", ""); w.Code != 401 || profiles.calls != 0 {
		t.Fatal("unauthenticated request reached database")
	}

	if w := request("GET", "/api/v1/me", token); w.Code != 200 {
		t.Fatalf("me=%d", w.Code)
	}

	profiles.ready = false
	if w := request("GET", "/api/v1/me", token); w.Code != 403 {
		t.Fatal("missing profile accepted")
	}

	profiles.err = errors.New("private SQL failure")
	if w := request("GET", "/api/v1/me", token); w.Code != 503 {
		t.Fatal("profile outage not safely mapped")
	}

	profiles.ready = true
	profiles.err = nil
	if w := request("POST", "/api/v1/ai/chat", ""); w.Code != 401 {
		t.Fatal("chat exposed")
	}

	if w := request("POST", "/api/v1/ai/chat", token); w.Code != 501 {
		t.Fatal("placeholder misreported working chat")
	}

	r := httptest.NewRequest("OPTIONS", "/api/v1/ai/chat", nil)
	r.Header.Set("Origin", "https://web.example.test")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("protected chat preflight failed")
	}

	if request("GET", "/api/v1/me", token).Header().Get("Cache-Control") != "no-store" {
		t.Fatal("me cached")
	}
}
