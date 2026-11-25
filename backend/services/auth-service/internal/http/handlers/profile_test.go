package handler_test

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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/websession"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
)

const profileBody = `{"revision":0,"name_si":"","name_ta":"","date_of_birth":"","gender":"","address":{"line1":"Synthetic street","line2":"","city":"Synthetic city"}}`

type endpointProfiles struct {
	issuer  string
	records map[string]profile.Details
	calls   int
	err     error
}

func (s *endpointProfiles) HasReadyProfile(_ context.Context, issuer, subject string) (bool, error) {
	_, ok := s.records[subject]
	return issuer == s.issuer && ok, nil
}
func (s *endpointProfiles) RegisteredProfile(_ context.Context, issuer, subject string) (registration.ProfileData, error) {
	s.calls++
	if s.err != nil {
		return registration.ProfileData{}, s.err
	}
	if _, ok := s.records[subject]; !ok || issuer != s.issuer {
		return registration.ProfileData{}, profile.ErrNotFound
	}
	nic, _ := registration.ParseNIC("123456789v")
	if subject == "citizen-b" {
		nic, _ = registration.ParseNIC("223456789v")
	}
	return registration.ProfileData{FullName: "Registered " + subject, NIC: nic, MobileNumber: "+94770000000"}, nil
}
func (s *endpointProfiles) CitizenDetails(_ context.Context, issuer, subject string) (profile.Details, error) {
	s.calls++
	d, ok := s.records[subject]
	if !ok || issuer != s.issuer {
		return profile.Details{}, profile.ErrNotFound
	}
	return d, nil
}
func (s *endpointProfiles) SaveCitizenDetails(_ context.Context, issuer, subject string, revision int64, fields profile.EditableDetails) (profile.Details, error) {
	s.calls++
	d, ok := s.records[subject]
	if !ok || issuer != s.issuer {
		return profile.Details{}, profile.ErrNotFound
	}
	if d.Revision != revision {
		return profile.Details{}, profile.ErrConflict
	}
	d.Fields = fields
	d.Revision++
	s.records[subject] = d
	return d, nil
}

func signedProfileTokens(t *testing.T) (string, *authn.Verifier, func(map[string]any) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "profile", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	t.Cleanup(keys.Close)
	issuer := keys.URL + "/realms/profile"
	verifier, err := authn.New(authn.Config{Issuer: issuer, Audience: "govai-api", Clients: []string{"mobile"}}, keys.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(verifier.Close)
	sign := func(extra map[string]any) string {
		claims := map[string]any{"iss": issuer, "aud": "govai-api", "sub": "citizen-a", "azp": "mobile", "typ": "Bearer", "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "email_verified": true, "email": "synthetic@example.test", "realm_access": map[string]any{"roles": []string{"citizen"}}}
		for k, v := range extra {
			claims[k] = v
		}
		header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "profile", "typ": "JWT"})
		body, _ := json.Marshal(claims)
		encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
		sum := sha256.Sum256([]byte(encoded))
		signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if err != nil {
			t.Fatal(err)
		}
		return encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
	}
	return issuer, verifier, sign
}

func TestNativeProfileEndpoints(t *testing.T) {
	issuer, verifier, sign := signedProfileTokens(t)
	store := &endpointProfiles{issuer: issuer, records: map[string]profile.Details{"citizen-a": {}, "citizen-b": {}}}
	service, _ := profile.NewService(store, profile.CitizenFieldRules{})
	protected, err := handler.NewProtected(verifier, store)
	if err != nil {
		t.Fatal(err)
	}
	if err = protected.EnableProfile(service); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	router := handler.NewRouter(handler.RouterOptions{Protected: protected, CORSOrigins: []string{"https://web.test"}, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		name   string
		claims map[string]any
		token  string
		want   int
	}{
		{name: "no-token", want: 401},
		{name: "expired", claims: map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}, want: 401},
		{name: "wrong-issuer", claims: map[string]any{"iss": "https://other.test/realms/profile"}, want: 401},
		{name: "wrong-audience", claims: map[string]any{"aud": "other-api"}, want: 401},
		{name: "wrong-client", claims: map[string]any{"azp": "other"}, want: 401},
		{name: "id-token", claims: map[string]any{"typ": "ID"}, want: 401},
		{name: "wrong-role", claims: map[string]any{"realm_access": map[string]any{"roles": []string{"staff"}}}, want: 403},
		{name: "unverified", claims: map[string]any{"email_verified": false}, want: 403},
		{name: "unregistered", claims: map[string]any{"sub": "unknown"}, want: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := tc.token
			if tc.claims != nil {
				token = sign(tc.claims)
			}
			before := store.calls
			w := request("GET", "/api/v1/profile", token, "")
			if w.Code != tc.want || store.calls != before {
				t.Fatalf("status=%d wanted=%d or unauthenticated data access", w.Code, tc.want)
			}
		})
	}
	token := sign(nil)
	w := request("GET", "/api/v1/profile", token, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"email":"synthetic@example.test"`) || !strings.Contains(w.Body.String(), `"full_name":"Registered citizen-a"`) {
		t.Fatal("profile GET failed", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("profile cached")
	}
	for _, mode := range []string{"no-tls", "mixed-cookie", "mixed-web-header"} {
		r := httptest.NewRequest("GET", "/api/v1/profile", nil)
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Authorization", "Bearer "+token)
		want := 403
		switch mode {
		case "no-tls":
			r.TLS = nil
			want = 426
		case "mixed-cookie":
			r.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: syntheticWebSecret})
		case "mixed-web-header":
			r.Header.Set("X-GovAI-Web", "1")
		}
		denied := httptest.NewRecorder()
		router.ServeHTTP(denied, r)
		if denied.Code != want {
			t.Fatal("native credential boundary failed", mode, denied.Code)
		}
	}
	for _, field := range []string{"issuer", "subject", "access_token", "photo_object_key", "password"} {
		if strings.Contains(w.Body.String(), `"`+field+`"`) {
			t.Fatal("internal data exposed")
		}
	}
	if request("GET", "/api/v1/profile?subject=citizen-b", token, "").Code != 400 || request("GET", "/api/v1/profile/citizen-b", token, "").Code != 404 {
		t.Fatal("caller-selected identity allowed")
	}
	for _, field := range []string{"nic", "full_name", "mobile_number", "email", "subject", "issuer", "role", "photo_object_key"} {
		body := strings.TrimSuffix(profileBody, "}") + `,"` + field + `":"forged"}`
		before := store.calls
		if request("PUT", "/api/v1/profile", token, body).Code != 400 || store.calls != before {
			t.Fatal("forged field reached profile service")
		}
	}
	w = request("PUT", "/api/v1/profile", token, profileBody)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"revision":1`) {
		t.Fatal("PUT failed", w.Code)
	}
	if request("PUT", "/api/v1/profile", token, profileBody).Code != 409 {
		t.Fatal("stale revision not rejected")
	}
	w = request("GET", "/api/v1/profile", sign(map[string]any{"sub": "citizen-b"}), "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"revision":0`) || !strings.Contains(w.Body.String(), `"full_name":"Registered citizen-b"`) {
		t.Fatal("citizen A changed citizen B")
	}
	current := strings.Replace(profileBody, `"revision":0`, `"revision":1`, 1)
	if request("PUT", "/api/v1/profile", token, strings.Replace(current, `"date_of_birth":""`, `"date_of_birth":"2001-02-29"`, 1)).Code != 422 {
		t.Fatal("invalid date accepted")
	}
	w = request("PUT", "/api/v1/profile", token, strings.Replace(current, `"date_of_birth":""`, `"date_of_birth":"2000-02-29"`, 1))
	if w.Code != 422 || !strings.Contains(w.Body.String(), "dob_nic_mismatch") {
		t.Fatal("NIC/DOB mismatch accepted")
	}
	invalidGender := strings.Replace(current, `"gender":""`, `"gender":"other"`, 1)
	genderResult := request("PUT", "/api/v1/profile", token, invalidGender)
	if genderResult.Code != 422 || !strings.Contains(genderResult.Body.String(), "invalid_gender") {
		t.Fatal("unsupported gender accepted")
	}
	matched := strings.Replace(current, `"date_of_birth":""`, `"date_of_birth":"2012-12-10"`, 1)
	matched = strings.Replace(matched, `"gender":""`, `"gender":"male"`, 1)
	w = request("PUT", "/api/v1/profile", token, matched)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"revision":2`) {
		t.Fatal("matching DOB/gender update failed", w.Code)
	}
	if request("PUT", "/api/v1/profile", token, strings.Repeat("x", profile.MaxUpdateBytes+1)).Code != 413 {
		t.Fatal("oversized update accepted")
	}
	store.err = errors.New("private database details")
	w = request("GET", "/api/v1/profile", token, "")
	if w.Code != 503 || strings.Contains(w.Body.String(), "private database") {
		t.Fatal("database error leaked")
	}
	store.err = nil
	for _, method := range []string{"GET", "PUT"} {
		r := httptest.NewRequest("OPTIONS", "/api/v1/profile", nil)
		r.Header.Set("Origin", "https://web.test")
		r.Header.Set("Access-Control-Request-Method", method)
		r.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatal("profile preflight rejected")
		}
	}
	if strings.Contains(logs.String(), token) || strings.Contains(logs.String(), "Synthetic street") || strings.Contains(logs.String(), "123456789v") || strings.Contains(logs.String(), "synthetic@example.test") {
		t.Fatal("personal data in request logs")
	}
}

type profileBrowser struct {
	browserDouble
	caller profile.Caller
	denied bool
}

func (s *profileBrowser) ProfileIdentity(_ context.Context, raw, csrf string, write bool) (profile.Caller, error) {
	if s.denied || raw != syntheticWebSecret || (write && csrf != syntheticWebSecret) {
		return profile.Caller{}, websession.ErrDenied
	}
	return s.caller, nil
}

func TestWebProfileEndpointSecurity(t *testing.T) {
	store := &endpointProfiles{issuer: "https://issuer.test", records: map[string]profile.Details{"citizen-a": {}, "citizen-b": {}}}
	service, _ := profile.NewService(store, profile.CitizenFieldRules{})
	session := &profileBrowser{caller: profile.Caller{Issuer: store.issuer, Subject: "citizen-a", Email: "synthetic@example.test"}}
	limit, err := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: make([]byte, 32), IPAttempts: 100, EmailAttempts: 1, IPWindow: time.Minute, EmailWindow: time.Minute, MaxKeys: 10, MaxConcurrent: 4})
	if err != nil {
		t.Fatal(err)
	}
	web, err := handler.NewWebSessionHandler(session, limit, "https://govai.test")
	if err != nil {
		t.Fatal(err)
	}
	if err = web.EnableProfile(service); err != nil {
		t.Fatal(err)
	}
	router := handler.NewRouter(handler.RouterOptions{WebSessions: web, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, tc := range []struct {
		name   string
		method string
		want   int
	}{
		{"get", "GET", 200}, {"put", "PUT", 200}, {"no-cookie", "GET", 401}, {"no-csrf", "PUT", 403}, {"wrong-csrf", "PUT", 401},
		{"wrong-origin", "PUT", 403}, {"no-origin", "PUT", 403}, {"no-header", "GET", 403}, {"no-tls", "GET", 426},
		{"authorization", "GET", 403}, {"duplicate-cookie", "GET", 401}, {"expired-session", "GET", 401}, {"wrong-media", "PUT", 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store.records["citizen-a"] = profile.Details{}
			session.denied = tc.name == "expired-session"
			r := httptest.NewRequest(tc.method, "/api/v1/web/profile", strings.NewReader(profileBody))
			r.TLS = &tls.ConnectionState{}
			r.Header.Set("Origin", "https://govai.test")
			r.Header.Set("X-GovAI-Web", "1")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-CSRF-Token", syntheticWebSecret)
			r.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: syntheticWebSecret})
			switch tc.name {
			case "no-cookie":
				r.Header.Del("Cookie")
			case "no-csrf":
				r.Header.Del("X-CSRF-Token")
			case "wrong-csrf":
				r.Header.Set("X-CSRF-Token", base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
			case "wrong-origin":
				r.Header.Set("Origin", "https://evil.test")
			case "no-origin":
				r.Header.Del("Origin")
			case "no-header":
				r.Header.Del("X-GovAI-Web")
			case "no-tls":
				r.TLS = nil
			case "authorization":
				r.Header.Set("Authorization", "Bearer mixed")
			case "duplicate-cookie":
				r.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: syntheticWebSecret})
			case "wrong-media":
				r.Header.Set("Content-Type", "text/plain")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
	for _, method := range []string{"GET", "PUT", "POST", "DELETE"} {
		r := httptest.NewRequest("OPTIONS", "/api/v1/web/profile", nil)
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Origin", "https://govai.test")
		r.Header.Set("Access-Control-Request-Method", method)
		r.Header.Set("Access-Control-Request-Headers", "x-govai-web,x-csrf-token,content-type")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		want := 403
		if method == "GET" || method == "PUT" {
			want = 204
		}
		if w.Code != want {
			t.Fatal("unsafe web preflight method")
		}
	}
}
