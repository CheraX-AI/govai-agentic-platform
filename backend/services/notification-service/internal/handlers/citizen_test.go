package handlers

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/inbox"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/preferences"
)

const inboxID = "123e4567-e89b-42d3-a456-426614174000"

type apiStore struct {
	owner    inbox.Owner
	calls    int
	page     inbox.Page
	result   inbox.Result
	failure  error
	channels map[string]preferences.Channels
	read     map[string]bool
}

func (s *apiStore) touch(ctx context.Context, owner inbox.Owner) error {
	s.calls++
	s.owner = owner
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.failure
}

func (s *apiStore) List(ctx context.Context, owner inbox.Owner, page inbox.Page) (
	inbox.Result, error,
) {
	s.page = page
	if err := s.touch(ctx, owner); err != nil {
		return inbox.Result{}, err
	}
	if owner.Subject != "alice" {
		return inbox.Result{}, nil
	}
	return s.result, nil
}

func (s *apiStore) MarkRead(ctx context.Context, owner inbox.Owner, id string) (bool, error) {
	if err := s.touch(ctx, owner); err != nil {
		return false, err
	}
	if owner.Subject != "alice" || id != inboxID {
		return false, nil
	}
	s.read[id] = true
	return true, nil
}

func (s *apiStore) UnreadCount(ctx context.Context, owner inbox.Owner) (int64, error) {
	if err := s.touch(ctx, owner); err != nil {
		return 0, err
	}
	if owner.Subject == "alice" && !s.read[inboxID] {
		return 1, nil
	}
	return 0, nil
}

func (s *apiStore) Get(ctx context.Context, owner inbox.Owner) (preferences.Channels, error) {
	if err := s.touch(ctx, owner); err != nil {
		return preferences.Channels{}, err
	}
	return s.channels[owner.Subject], nil
}

func (s *apiStore) Replace(
	ctx context.Context, owner inbox.Owner, channels preferences.Channels,
) error {
	if err := s.touch(ctx, owner); err != nil {
		return err
	}
	s.channels[owner.Subject] = channels
	return nil
}

type apiFixture struct {
	router http.Handler
	key    *rsa.PrivateKey
	issuer string
	store  *apiStore
}

// The fixture uses locally signed JWTs and a TLS JWKS server, not injected identity.

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB",
		}}})
	}))
	t.Cleanup(keys.Close)
	issuer := keys.URL + "/realms/test"
	verifier, err := authn.New(authn.Config{
		Issuer: issuer, Audience: "govai-api", Clients: []string{"mobile", "web-bff"},
	}, keys.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(verifier.Close)
	store := &apiStore{
		channels: map[string]preferences.Channels{}, read: map[string]bool{},
		result: inbox.Result{Items: []inbox.Notification{{
			ID: inboxID, EventType: "citizen.login.succeeded", Priority: "high",
			Locale: "en", Title: "New sign-in", Body: "Synthetic account notification.",
			CreatedAt: time.Date(2026, 11, 26, 5, 0, 0, 0, time.UTC),
		}}},
	}
	router := Routes(func() bool { return false })
	if err := RegisterCitizenRoutes(router, verifier, store, store); err != nil {
		t.Fatal(err)
	}
	return &apiFixture{router: router, key: key, issuer: issuer, store: store}
}

func (f *apiFixture) token(t *testing.T, subject string, extra map[string]any) string {
	t.Helper()
	claims := map[string]any{
		"iss": f.issuer, "aud": []string{"govai-api"}, "sub": subject,
		"azp": "mobile", "typ": "Bearer", "email_verified": true,
		"exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(),
		"realm_access": map[string]any{"roles": []string{"citizen"}},
	}
	for key, value := range extra {
		claims[key] = value
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test"})
	body, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(encoded))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (f *apiFixture) send(method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "https://notifications.test"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	return response
}

func TestCitizenAuthenticationBeforeStorage(t *testing.T) {
	f := newAPIFixture(t)
	paths := []struct{ method, path string }{
		{"GET", "/api/notifications"}, {"GET", "/api/notifications/unread-count"},
		{"PATCH", "/api/notifications/" + inboxID + "/read"},
		{"GET", "/api/notifications/preferences"}, {"PUT", "/api/notifications/preferences"},
	}
	for _, route := range paths {
		for _, token := range []string{"", "invalid"} {
			response := f.send(route.method, route.path, token, "invalid JSON")
			if response.Code != 401 || f.store.calls != 0 {
				t.Fatal("unauthenticated request reached storage")
			}
		}
	}
	for _, test := range []struct {
		claims map[string]any
		status int
	}{
		{map[string]any{"iss": "https://wrong.example"}, 401},
		{map[string]any{"aud": "other-api"}, 401},
		{map[string]any{"azp": "unapproved"}, 401},
		{map[string]any{"typ": "ID"}, 401},
		{map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}, 401},
		{map[string]any{"email_verified": false}, 403},
		{map[string]any{"realm_access": map[string]any{"roles": []string{"staff"}}}, 403},
	} {
		response := f.send("GET", "/api/notifications", f.token(t, "alice", test.claims), "")
		if response.Code != test.status || f.store.calls != 0 {
			t.Fatal("unauthorized token reached storage")
		}
	}
	request := httptest.NewRequest("GET", "http://notifications.test/api/notifications", nil)
	request.Header.Set("Authorization", "Bearer "+f.token(t, "alice", nil))
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	if response.Code != 426 || f.store.calls != 0 {
		t.Fatal("plaintext request reached storage")
	}
}

func TestCitizenOwnerScopeAndReadState(t *testing.T) {
	f := newAPIFixture(t)
	alice, bob := f.token(t, "alice", nil), f.token(t, "bob", nil)
	response := f.send("GET", "/api/notifications", alice, "")
	if response.Code != 200 || f.store.owner.Issuer != f.issuer ||
		f.store.owner.Subject != "alice" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("verified owner or private response policy missing")
	}
	if strings.Contains(response.Body.String(), f.issuer) ||
		strings.Contains(response.Body.String(), "alice") {
		t.Fatal("owner reference leaked into inbox response")
	}
	response = f.send("GET", "/api/notifications", bob, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatal("second citizen saw another citizen's rows")
	}
	path := "/api/notifications/" + inboxID + "/read"
	if response = f.send("PATCH", path, bob, ""); response.Code != 404 || f.store.read[inboxID] {
		t.Fatal("other citizen changed read state")
	}
	response = f.send("GET", "/api/notifications/unread-count", alice, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"unread_count":1`) {
		t.Fatal("initial unread count incorrect")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if response = f.send("PATCH", path, alice, ""); response.Code != 204 {
			t.Fatal("repeat mark-read was not idempotent")
		}
	}
	response = f.send("GET", "/api/notifications/unread-count", alice, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"unread_count":0`) {
		t.Fatal("read notification remained unread")
	}
}

func TestCitizenStorageFailureIsPrivate(t *testing.T) {
	f := newAPIFixture(t)
	f.store.failure = errors.New("private database connection and account details")
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/notifications", ""}, {"GET", "/api/notifications/unread-count", ""},
		{"PATCH", "/api/notifications/" + inboxID + "/read", ""},
		{"GET", "/api/notifications/preferences", ""},
		{"PUT", "/api/notifications/preferences", `{"email_enabled":false,"in_app_enabled":true}`},
	} {
		response := f.send(route.method, route.path, f.token(t, "alice", nil), route.body)
		if response.Code != 503 || strings.Contains(response.Body.String(), "private") {
			t.Fatal("storage failure was not safely reported")
		}
	}
}
