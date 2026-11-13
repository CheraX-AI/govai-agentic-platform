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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// testKeycloak is a fake Keycloak that serves one public signing key.
type testKeycloak struct {
	server *httptest.Server
	issuer string
	key    *rsa.PrivateKey
	jwks   atomic.Value // published public key; tests can rotate it safely
}

func newTestKeycloak(t *testing.T) *testKeycloak {
	t.Helper()
	kc := &testKeycloak{key: newKey(t)}
	kc.jwks.Store(publicKey(kc.key, "one"))
	kc.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{kc.jwks.Load()}})
	}))
	t.Cleanup(kc.server.Close)
	kc.issuer = kc.server.URL + "/realms/test"
	return kc
}

func (kc *testKeycloak) verifier(t *testing.T) *Verifier {
	t.Helper()
	v, err := New(Config{Issuer: kc.issuer, Audience: "govai-api", Clients: []string{"mobile"}}, kc.server.Client())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(v.Close)
	return v
}

// validClaims is a correct citizen access token; tests change one field at a time.
func (kc *testKeycloak) validClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":            kc.issuer,
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

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

func publicKey(key *rsa.PrivateKey, kid string) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   "AQAB",
	}
}

// sign builds an RS256 JWT, the same way Keycloak does.
func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "RS256", "kid": kid})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func TestVerify(t *testing.T) {
	kc := newTestKeycloak(t)
	v := kc.verifier(t)

	identity, err := v.Verify(context.Background(), sign(t, kc.key, "one", kc.validClaims()))
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	if identity.Subject != "synthetic-subject" ||
		identity.Client != "mobile" ||
		!identity.EmailVerified ||
		len(identity.Roles) != 1 ||
		identity.Roles[0] != "citizen" {
		t.Fatalf("wrong identity: %+v", identity)
	}

	invalid := map[string]func(map[string]any){
		"expired":         func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"not valid yet":   func(c map[string]any) { c["nbf"] = time.Now().Add(time.Hour).Unix() },
		"wrong issuer":    func(c map[string]any) { c["iss"] = "https://other.example.test/realms/test" },
		"wrong audience":  func(c map[string]any) { c["aud"] = []string{"account"} },
		"wrong client":    func(c map[string]any) { c["azp"] = "other-app" },
		"ID token":        func(c map[string]any) { c["typ"] = "ID" },
		"missing subject": func(c map[string]any) { delete(c, "sub") },
	}
	for name, change := range invalid {
		t.Run(name, func(t *testing.T) {
			claims := kc.validClaims()
			change(claims)
			if _, err := v.Verify(context.Background(), sign(t, kc.key, "one", claims)); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}

	t.Run("bad signature", func(t *testing.T) {
		forged := sign(t, newKey(t), "one", kc.validClaims())
		if _, err := v.Verify(context.Background(), forged); err == nil {
			t.Fatal("token signed by another key accepted")
		}
	})

	t.Run("unsigned", func(t *testing.T) {
		body := strings.Split(sign(t, kc.key, "one", kc.validClaims()), ".")[1]
		if _, err := v.Verify(context.Background(), "eyJhbGciOiJub25lIn0."+body+"."); err == nil {
			t.Fatal("unsigned token accepted")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := v.Verify(context.Background(), ""); err == nil {
			t.Fatal("empty token accepted")
		}
	})
}

func TestKeyRotation(t *testing.T) {
	kc := newTestKeycloak(t)
	v := kc.verifier(t)
	if _, err := v.Verify(context.Background(), sign(t, kc.key, "one", kc.validClaims())); err != nil {
		t.Fatal(err)
	}

	// Keycloak switches to a new signing key; the verifier must fetch it.
	kc.key = newKey(t)
	kc.jwks.Store(publicKey(kc.key, "two"))
	if _, err := v.Verify(context.Background(), sign(t, kc.key, "two", kc.validClaims())); err != nil {
		t.Fatalf("token signed with rotated key rejected: %v", err)
	}
}

func TestMiddleware(t *testing.T) {
	kc := newTestKeycloak(t)
	v := kc.verifier(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	reached := 0
	router.GET("/protected", v.Authenticate(), RequireVerifiedEmail(), RequireRole("citizen"), func(c *gin.Context) {
		identity, ok := Current(c)
		if !ok || identity.Subject != "synthetic-subject" {
			t.Error("identity missing in handler")
		}

		reached++
		c.Status(http.StatusOK)
	})

	token := func(change func(map[string]any)) string {
		claims := kc.validClaims()
		if change != nil {
			change(claims)
		}

		return "Bearer " + sign(t, kc.key, "one", claims)
	}

	tests := []struct {
		name    string
		headers []string
		noTLS   bool
		want    int
	}{
		{name: "valid", headers: []string{token(nil)}, want: 200},
		{name: "missing token", want: 401},
		{name: "not Bearer", headers: []string{"Basic abc"}, want: 401},
		{name: "two headers", headers: []string{token(nil), token(nil)}, want: 401},
		{name: "expired", headers: []string{token(func(c map[string]any) {
			c["exp"] = time.Now().Add(-time.Minute).Unix()
		})}, want: 401},
		{name: "wrong role", headers: []string{token(func(c map[string]any) {
			c["realm_access"] = map[string]any{"roles": []string{"staff"}}
		})}, want: 403},
		{name: "unverified email", headers: []string{token(func(c map[string]any) {
			c["email_verified"] = false
		})}, want: 403},
		{name: "plain HTTP", headers: []string{token(nil)}, noTLS: true, want: 426},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if !test.noTLS {
				req.TLS = &tls.ConnectionState{}
			}

			for _, h := range test.headers {
				req.Header.Add("Authorization", h)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != test.want {
				t.Fatalf("status = %d, want %d", w.Code, test.want)
			}

			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("response may be cached")
			}

			if test.want == 401 && w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("401 without WWW-Authenticate")
			}
		})
	}

	if reached != 1 {
		t.Fatalf("handler reached %d times, want only for the valid token", reached)
	}
}

func TestNewRejectsUnsafeConfig(t *testing.T) {
	kc := newTestKeycloak(t)
	good := Config{Issuer: kc.issuer, Audience: "govai-api", Clients: []string{"mobile"}}

	bad := map[string]func(*Config){
		"HTTP issuer":     func(c *Config) { c.Issuer = "http://localhost/realms/test" },
		"trailing slash":  func(c *Config) { c.Issuer += "/" },
		"query in issuer": func(c *Config) { c.Issuer += "?x=1" },
		"no audience":     func(c *Config) { c.Audience = "" },
		"no clients":      func(c *Config) { c.Clients = nil },
	}
	for name, change := range bad {
		t.Run(name, func(t *testing.T) {
			config := good
			change(&config)
			if _, err := New(config, kc.server.Client()); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}

	t.Run("TLS checks off", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
		if _, err := New(good, client); err == nil {
			t.Fatal("client without certificate checks accepted")
		}
	})

	t.Run("no client", func(t *testing.T) {
		if _, err := New(good, nil); err == nil {
			t.Fatal("nil client accepted")
		}
	})
}
