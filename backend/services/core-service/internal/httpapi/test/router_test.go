package httpapi_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/httpapi"
)

type fakeAgent struct {
	calls     int
	message   string
	reply     chatcontract.Reply
	err       error
	panicCall bool
}

func (f *fakeAgent) Ask(ctx context.Context, q chatcontract.Question) (chatcontract.Reply, error) {
	f.calls++
	f.message = q.Text()
	if f.panicCall {
		panic("private upstream details")
	}

	return f.reply, f.err
}

type fixture struct {
	verifier *authn.Verifier
	router   http.Handler
	key      *rsa.PrivateKey
	issuer   string
	agent    *fakeAgent
	logs     *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	keys := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(
			map[string]any{
				"keys": []any{
					map[string]string{
						"kty": "RSA",
						"kid": "test",
						"use": "sig",
						"alg": "RS256",
						"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
						"e":   "AQAB",
					},
				},
			},
		)
	}))
	t.Cleanup(keys.Close)
	issuer := keys.URL + "/realms/test"
	verifier, err := authn.New(
		authn.Config{Issuer: issuer, Audience: "govai-api", Clients: []string{"mobile"}},
		keys.Client(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(verifier.Close)
	agent := &fakeAgent{
		reply: chatcontract.Reply{
			Answer: "Synthetic answer only.",
			Sources: []chatcontract.Source{
				{Number: 1, Title: "Synthetic source", Page: 2, URL: "https://official.example.test/guide"},
			},
		},
	}
	logs := &bytes.Buffer{}
	router, err := httpapi.NewRouter(
		verifier,
		agent,
		[]string{"https://web.example.test"},
		slog.New(slog.NewJSONHandler(logs, nil)),
	)
	if err != nil {
		t.Fatal(err)
	}

	return &fixture{verifier: verifier, router: router, key: key, issuer: issuer, agent: agent, logs: logs}
}

func (f *fixture) token(t *testing.T, overrides map[string]any) string {
	t.Helper()
	claims := map[string]any{
		"iss":            f.issuer,
		"aud":            []string{"govai-api"},
		"sub":            "synthetic-subject",
		"azp":            "mobile",
		"typ":            "Bearer",
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Minute).Unix(),
		"email_verified": true,
		"realm_access":   map[string]any{"roles": []string{"citizen"}},
	}
	for key, value := range overrides {
		claims[key] = value
	}

	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(encoded))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}

	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (f *fixture) send(token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		"POST",
		"https://core.test/api/chat/message?private_query=hidden",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func TestAuthenticationPrecedesMessageAndAgent(t *testing.T) {
	f := newFixture(t)
	for _, token := range []string{"", "invalid"} {
		if r := f.send(token, `{"message":"hello"}`); r.Code != 401 || f.agent.calls != 0 {
			t.Fatal("unauthorized request reached agent")
		}
	}

	for _, tc := range []struct {
		claims map[string]any
		status int
	}{
		{map[string]any{"iss": "https://wrong.example.test"}, 401},
		{map[string]any{"aud": []string{"other"}}, 401},
		{map[string]any{"azp": "other"}, 401},
		{map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}, 401},
		{map[string]any{"email_verified": false}, 403},
		{map[string]any{"realm_access": map[string]any{"roles": []string{"staff"}}}, 403},
	} {
		if r := f.send(f.token(t, tc.claims), `{"message":"hello"}`); r.Code != tc.status ||
			f.agent.calls != 0 {
			t.Fatal("invalid identity reached agent")
		}
	}
}

func TestSuccessAndSafeMetadata(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	message := "கடவுச்சீட்டு private-question-marker"
	body, _ := json.Marshal(map[string]string{"message": message})
	r := f.send(token, string(body))
	if r.Code != 200 ||
		f.agent.calls != 1 ||
		f.agent.message != message ||
		r.Header().Get("Cache-Control") != "no-store" ||
		r.Header().Get("X-Request-ID") == "" {
		t.Fatal("valid citizen request failed")
	}

	var reply chatcontract.Reply
	if json.Unmarshal(r.Body.Bytes(), &reply) != nil ||
		reply.Answer != f.agent.reply.Answer ||
		reply.Sources[0] != f.agent.reply.Sources[0] {
		t.Fatal("response contract mismatch")
	}

	for _, secret := range []string{token, message, "private_query", "hidden", "synthetic-subject", f.agent.reply.Answer} {
		if strings.Contains(f.logs.String(), secret) {
			t.Fatal("private content logged")
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(f.logs.Bytes()))
	var event map[string]any
	if decoder.Decode(&event) != nil ||
		len(event) != 8 ||
		event["route"] != "/api/chat/message" ||
		event["status"] != float64(200) {
		t.Fatal("invalid metadata log")
	}
}

func TestMessageFailures(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	for _, body := range []string{
		`{}`,
		`{"message":""}`,
		`{"message":[]}`,
		`{"message":"hi","message":"other"}`,
		`{"message":"hi","subject":"caller"}`,
	} {
		if r := f.send(token, body); r.Code != 400 || f.agent.calls != 0 {
			t.Fatal("invalid message reached agent")
		}
	}

	if r := f.send(token, strings.Repeat("x", chatcontract.MaxRequestBytes+1)); r.Code != 413 || f.agent.calls != 0 {
		t.Fatal("oversized message reached agent")
	}

	req := httptest.NewRequest("POST", "https://core.test/api/chat/message", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != 415 {
		t.Fatal("wrong media type accepted")
	}
}

func TestAgentErrorsAreSafe(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	for _, tc := range []struct {
		err    error
		status int
	}{{errors.New("private upstream URL/password"), 503}, {context.DeadlineExceeded, 504}} {
		f.agent.err = tc.err
		r := f.send(token, `{"message":"hello"}`)
		if r.Code != tc.status || strings.Contains(r.Body.String(), "private") {
			t.Fatal("unsafe upstream error")
		}
	}

	f.agent.err = nil
	f.agent.reply = chatcontract.Reply{}
	if r := f.send(token, `{"message":"hello"}`); r.Code != 502 {
		t.Fatal("invalid answer accepted")
	}

	f.agent.panicCall = true
	if r := f.send(token, `{"message":"hello"}`); r.Code != 500 ||
		strings.Contains(r.Body.String(), "private") {
		t.Fatal("panic disclosure")
	}
}

func TestCORSAndHealth(t *testing.T) {
	f := newFixture(t)
	req := httptest.NewRequest("OPTIONS", "https://core.test/api/chat/message", nil)
	req.Header.Set("Origin", "https://web.example.test")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != 204 ||
		w.Header().Get("Access-Control-Allow-Origin") != "https://web.example.test" ||
		w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("invalid preflight")
	}

	req.Header.Set("Origin", "https://other.test")
	w = httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != 403 || f.agent.calls != 0 {
		t.Fatal("origin bypass")
	}

	req = httptest.NewRequest("GET", "https://core.test/api/health", nil)
	w = httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	data, _ := io.ReadAll(w.Result().Body)
	if w.Code != 200 || !bytes.Contains(data, []byte("govai-core")) {
		t.Fatal("health failed")
	}
}

func TestChatTLSAndCookieOnlyRejection(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	req := httptest.NewRequest("POST", "http://core.test/api/chat/message", strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != 426 || f.agent.calls != 0 {
		t.Fatal("plaintext request accepted")
	}

	req = httptest.NewRequest("POST", "https://core.test/api/chat/message", strings.NewReader(`{"message":"hello"}`))
	req.AddCookie(&http.Cookie{Name: "__Host-govai_session", Value: "synthetic-cookie"})
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != 401 || f.agent.calls != 0 {
		t.Fatal("cookie authenticated at bearer core API")
	}
}

type blockingAgent struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (a *blockingAgent) Ask(ctx context.Context, q chatcontract.Question) (chatcontract.Reply, error) {
	a.calls.Add(1)
	a.started <- struct{}{}
	select {
	case <-a.release:
		return chatcontract.Reply{Answer: "Synthetic bounded test reply."}, nil
	case <-ctx.Done():
		return chatcontract.Reply{}, ctx.Err()
	}
}

func TestChatBoundsConcurrentRequests(t *testing.T) {
	f := newFixture(t)
	agent := &blockingAgent{started: make(chan struct{}, 8), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(agent.release) })
	router, err := httpapi.NewRouter(f.verifier, agent, nil, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	f.router = router
	token := f.token(t, nil)
	results := make(chan int, 8)
	for i := 0; i < 8; i++ {
		go func() { results <- f.send(token, `{"message":"synthetic question"}`).Code }()
	}

	for i := 0; i < 8; i++ {
		select {
		case <-agent.started:
		case <-time.After(3 * time.Second):
			t.Fatal("requests did not start")
		}
	}

	response := f.send(token, `{"message":"synthetic question"}`)
	if response.Code != 503 || response.Header().Get("Retry-After") != "1" || agent.calls.Load() != 8 {
		t.Fatal("concurrency bound failed")
	}

	release.Do(func() { close(agent.release) })
	for i := 0; i < 8; i++ {
		select {
		case status := <-results:
			if status != 200 {
				t.Fatal("admitted request failed")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("admitted requests did not finish")
		}
	}
}
