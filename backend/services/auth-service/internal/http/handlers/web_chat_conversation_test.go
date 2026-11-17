package handler_test

import (
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/chatgateway"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
)

const (
	knownConversation = "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"
	newConversation   = "01936f3e-7b2a-7c4d-9e5f-0a1b2c3d4e5f"
)

// fakeCore stands in for core-service: it records what the gateway forwards and
// answers with a fixed status and body.
type fakeCore struct {
	status int
	reply  string
	calls  int
	sent   string
}

func (f *fakeCore) serve(w http.ResponseWriter, r *http.Request) {
	f.calls++
	body, _ := io.ReadAll(r.Body)
	f.sent = string(body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.status)
	_, _ = io.WriteString(w, f.reply)
}

// sendWebChat posts one browser chat through the real router, session double
// and gateway client, with valid cookie, CSRF, origin and TLS unless noCSRF.
func sendWebChat(t *testing.T, core *fakeCore, body string, noCSRF bool) *httptest.ResponseRecorder {
	t.Helper()
	upstream := httptest.NewTLSServer(http.HandlerFunc(core.serve))
	t.Cleanup(upstream.Close)
	roots := x509.NewCertPool()
	roots.AddCert(upstream.Certificate())
	client, err := chatgateway.New(upstream.URL+"/api/chat/message", roots, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(client.Close)
	limit, err := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{
		Key:           make([]byte, 32),
		IPAttempts:    60,
		EmailAttempts: 1,
		IPWindow:      time.Minute,
		EmailWindow:   time.Minute,
		MaxKeys:       10,
		MaxConcurrent: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	h, err := handler.NewWebSessionHandler(&chatSessionDouble{}, limit, "https://govai.test")
	if err != nil {
		t.Fatal(err)
	}

	if err = h.EnableChat(client); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "https://govai.test/api/v1/web/chat", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Origin", "https://govai.test")
	req.Header.Set("X-GovAI-Web", "1")
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: syntheticWebSecret})
	if !noCSRF {
		req.Header.Set("X-CSRF-Token", syntheticWebSecret)
	}

	w := httptest.NewRecorder()
	handler.NewRouter(handler.RouterOptions{WebSessions: h}).ServeHTTP(w, req)
	return w
}

func replyConversation(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var reply map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &reply) != nil {
		t.Fatalf("chat failed: %d %s", w.Code, w.Body.String())
	}

	id, _ := reply["conversation_id"].(string)
	return id
}

func errorCodeOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatalf("invalid error body: %s", w.Body.String())
	}

	code, _ := body["code"].(string)
	return code
}

func TestWebChatLegacyRequestGetsCoreConversationID(t *testing.T) {
	core := &fakeCore{status: 200, reply: `{"answer":"Synthetic answer","sources":[],"conversation_id":"` + newConversation + `"}`}

	w := sendWebChat(t, core, `{"message":"Synthetic question"}`, false)

	// No ID is invented here: the legacy body is forwarded unchanged and core's ID returned
	if core.sent != `{"message":"Synthetic question"}` {
		t.Fatalf("legacy request changed: %s", core.sent)
	}

	if replyConversation(t, w) != newConversation {
		t.Fatal("core's conversation ID not returned to the browser")
	}
}

func TestWebChatForwardsConversationID(t *testing.T) {
	core := &fakeCore{status: 200, reply: `{"answer":"Synthetic answer","sources":[],"conversation_id":"` + knownConversation + `"}`}

	w := sendWebChat(t, core, `{"message":"Synthetic question","conversation_id":"`+knownConversation+`"}`, false)

	if core.sent != `{"conversation_id":"`+knownConversation+`","message":"Synthetic question"}` {
		t.Fatalf("conversation ID not forwarded: %s", core.sent)
	}

	if replyConversation(t, w) != knownConversation {
		t.Fatal("conversation ID not returned")
	}
}

func TestWebChatRejectsSwitchedConversation(t *testing.T) {
	core := &fakeCore{status: 200, reply: `{"answer":"Synthetic answer","sources":[],"conversation_id":"` + newConversation + `"}`}

	w := sendWebChat(t, core, `{"message":"Synthetic question","conversation_id":"`+knownConversation+`"}`, false)

	if w.Code != 502 || strings.Contains(w.Body.String(), newConversation) {
		t.Fatalf("switched conversation accepted: %d", w.Code)
	}
}

func TestWebChatConversationNotFoundIs404(t *testing.T) {
	core := &fakeCore{status: 404, reply: `{"error":"Conversation not found.","code":"conversation_not_found"}`}

	w := sendWebChat(t, core, `{"message":"Synthetic question","conversation_id":"`+knownConversation+`"}`, false)

	if w.Code != 404 || errorCodeOf(t, w) != "conversation_not_found" || core.calls != 1 {
		t.Fatalf("status=%d calls=%d", w.Code, core.calls)
	}
}

func TestWebChatMalformedConversationIDIs400(t *testing.T) {
	core := &fakeCore{status: 200, reply: `{"answer":"x"}`}
	for _, body := range []string{
		`{"message":"Synthetic question","conversation_id":"not-a-uuid"}`,
		`{"message":"Synthetic question","conversation_id":"` + strings.ToUpper(knownConversation) + `"}`,
		`{"message":"Synthetic question","conversation_id":null}`,
	} {
		w := sendWebChat(t, core, body, false)
		if w.Code != 400 || errorCodeOf(t, w) != "invalid_conversation_id" {
			t.Fatalf("malformed ID: %d %s", w.Code, body)
		}
	}

	if core.calls != 0 {
		t.Fatal("malformed ID reached core")
	}
}

func TestWebChatConversationIDDoesNotBypassChecks(t *testing.T) {
	core := &fakeCore{status: 200, reply: `{"answer":"x"}`}

	// Missing CSRF is still refused before anything is forwarded
	if w := sendWebChat(t, core, `{"message":"Synthetic question","conversation_id":"`+knownConversation+`"}`, true); w.Code != 403 {
		t.Fatalf("missing CSRF accepted: %d", w.Code)
	}

	// A client-supplied user ID is never accepted
	if w := sendWebChat(t, core, `{"message":"Synthetic question","conversation_id":"`+knownConversation+`","user_id":"other"}`, false); w.Code != 400 {
		t.Fatalf("client user ID accepted: %d", w.Code)
	}

	if core.calls != 0 {
		t.Fatal("rejected request reached core")
	}
}
