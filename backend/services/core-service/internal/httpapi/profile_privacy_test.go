package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/chat"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/httpapi"
)

// Profile fields/claims must not be added to an AI request. This does not claim
// that personal names typed inside the user's question are recognized/masked.
func TestProfileFieldsExcludedAtAIBoundary(t *testing.T) {
	f := newFixture(t)
	private := map[string]any{"name_si": "සිංහල පෞද්ගලික නම", "name_ta": "தமிழ் தனிப்பட்ட பெயர்", "address": "Private synthetic address", "nic": "200322812345", "email": "private@example.test"}
	var calls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if json.Unmarshal(body, &payload) != nil || len(payload) != 2 || payload["message"] != "How do I renew a licence?" || payload["conversation_id"] == "" {
			t.Error("AI payload exceeded the approved question/conversation contract")
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-agent-key" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Citizen-Name") != "" || r.Header.Get("X-Citizen-Address") != "" {
			t.Error("citizen credentials/headers crossed the AI boundary")
		}
		for _, value := range private {
			if strings.Contains(string(body), value.(string)) {
				t.Error("profile attribute reached AI payload")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"answer":"Synthetic guidance only.","sources":[]}`)
	}))
	defer upstream.Close()
	agent, err := chat.NewHTTP(chat.HTTPConfig{URL: upstream.URL + "/chat", AuthMode: "bearer", BearerToken: "synthetic-agent-key", Timeout: time.Second}, upstream.Client())
	if err != nil {
		t.Fatal("test agent adapter setup failed")
	}
	defer agent.Close()
	f.router, err = httpapi.NewRouter(f.verifier, agent, f.conversations, slog.New(slog.NewJSONHandler(f.logs, nil)), httpapi.Settings{})
	if err != nil {
		t.Fatal("test core router setup failed")
	}
	token := f.token(t, private)
	request := httptest.NewRequest("POST", "https://core.test/api/chat/message", strings.NewReader(`{"message":"How do I renew a licence?"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Citizen-Name", private["name_si"].(string))
	request.Header.Set("X-Citizen-Address", private["address"].(string))
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	if response.Code != 200 || calls.Load() != 1 {
		t.Fatal("authenticated privacy boundary flow failed", response.Code)
	}
	for key, value := range private {
		body, _ := json.Marshal(map[string]any{"message": "How do I renew a licence?", key: value})
		if f.send(token, string(body)).Code != 400 || calls.Load() != 1 {
			t.Fatal("profile field injection reached AI")
		}
		if strings.Contains(f.logs.String(), value.(string)) {
			t.Fatal("profile attribute appeared in logs")
		}
	}
	if strings.Contains(f.logs.String(), token) {
		t.Fatal("citizen JWT appeared in logs")
	}
}
