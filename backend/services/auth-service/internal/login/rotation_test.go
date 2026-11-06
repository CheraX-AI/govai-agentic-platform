package login

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSigningRotationAndForgery(t *testing.T) {
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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		key, kid := one, "one"
		if rotate.Load() {
			key, kid = two, "two"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk(key, kid)}})
	}))
	defer server.Close()
	issuer := server.URL + "/realms/test"
	p, e := NewKeycloak(KeycloakConfig{Issuer: issuer, ClientID: "mobile-client", ClientSecret: "secret", Audience: "govai-api"}, server.Client())
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.Verify(context.Background(), signToken(t, one, "one", baseClaims(issuer))); e != nil {
		t.Fatal("initial signing key rejected")
	}
	rotate.Store(true)
	if _, e = p.Verify(context.Background(), signToken(t, two, "two", baseClaims(issuer))); e != nil {
		t.Fatal("signing rotation rejected")
	}
	if count.Load() != 2 {
		t.Fatal("rotation fetch missing")
	}
	forged := signToken(t, one, "two", baseClaims(issuer))
	if _, e = p.Verify(context.Background(), forged); e == nil {
		t.Fatal("forged signature accepted")
	}
	unsigned := "eyJhbGciOiJub25lIn0." + strings.Split(forged, ".")[1] + "."
	if _, e = p.Verify(context.Background(), unsigned); e == nil {
		t.Fatal("unsigned algorithm accepted")
	}
}
