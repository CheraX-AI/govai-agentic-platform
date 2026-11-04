package httpapi

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
)

// This exercises the real HTTP adapter and authenticated router, not a live RAG.
func TestAgentHTTPResponseBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		upstream, status int
		delay            bool
	}{
		{name: "citations", body: `{"answer":"Synthetic answer only.","sources":[{"number":1,"title":"Synthetic reference","url":"https://source.example.test/path?q=1#part"}]}`, upstream: 200, status: 200},
		{name: "agent-service citations", body: `{"answer":"Synthetic answer only.","sources":[{"number":2,"title":"Synthetic guide","page":3}]}`, upstream: 200, status: 200},
		{name: "no sources", body: `{"answer":"Synthetic answer only."}`, upstream: 200, status: 200},
		{name: "empty answer", body: `{"answer":" "}`, upstream: 200, status: 502, code: "invalid_agent_response"},
		{name: "duplicate field", body: `{"answer":"one","answer":"two"}`, upstream: 200, status: 502, code: "invalid_agent_response"},
		{name: "invalid source", body: `{"answer":"Synthetic","sources":[{"number":1,"title":"reference","url":"http://source.example.test"}]}`, upstream: 200, status: 502, code: "invalid_agent_response"},
		{name: "source without number", body: `{"answer":"Synthetic","sources":[{"title":"reference"}]}`, upstream: 200, status: 502, code: "invalid_agent_response"},
		{name: "agent internals", body: `{"answer":"Synthetic","sources":[],"scope":"govai_documents"}`, upstream: 200, status: 502, code: "invalid_agent_response"},
		{name: "malformed", body: `private-upstream-body`, upstream: 200, status: 502, code: "invalid_agent_response"},
		{name: "unavailable", body: `private-upstream-body`, upstream: 503, status: 503, code: "chat_unavailable"},
		{name: "rate limited", body: `private-upstream-body`, upstream: 429, status: 503, code: "chat_unavailable"},
		{name: "timeout", upstream: 200, status: 504, code: "chat_timeout", delay: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			var calls atomic.Int32
			var upstreamRequestID atomic.Value
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				upstreamRequestID.Store(r.Header.Get("X-Request-ID"))
				if r.Header.Get("Authorization") != "Bearer synthetic-service-key" || r.Header.Get("Cookie") != "" {
					t.Error("incorrect upstream credentials")
				}
				if tc.delay {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(time.Second):
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.upstream)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer upstream.Close()
			timeout := 2 * time.Second
			if tc.delay {
				timeout = 100 * time.Millisecond
			}
			agent, err := chat.NewHTTP(chat.HTTPConfig{URL: upstream.URL + "/chat", AuthMode: "bearer", BearerToken: "synthetic-service-key", Timeout: timeout}, upstream.Client())
			if err != nil {
				t.Fatal(err)
			}
			defer agent.Close()
			f.router, err = NewRouter(f.verifier, agent, nil, slog.New(slog.NewJSONHandler(f.logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			if r := f.send("", `{"message":"private-question"}`); r.Code != 401 || calls.Load() != 0 {
				t.Fatal("unauthenticated upstream call")
			}
			token := f.token(t, nil)
			if r := f.send(token, `{"message":""}`); r.Code != 400 || calls.Load() != 0 {
				t.Fatal("invalid question reached upstream")
			}
			r := f.send(token, `{"message":"private-question"}`)
			if r.Code != tc.status || calls.Load() != 1 {
				t.Fatalf("status=%d calls=%d", r.Code, calls.Load())
			}
			if r.Header().Get("Cache-Control") != "no-store" || r.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing safe response headers")
			}
			if upstreamRequestID.Load() != r.Header().Get("X-Request-ID") {
				t.Fatal("request ID not forwarded to the agent")
			}
			if tc.status == 200 {
				var got chat.Reply
				if json.Unmarshal(r.Body.Bytes(), &got) != nil || got.Answer != "Synthetic answer only." || got.Sources == nil {
					t.Fatal("response contract mismatch")
				}
				if tc.name == "citations" && (len(got.Sources) != 1 || got.Sources[0].URL != "https://source.example.test/path?q=1#part") {
					t.Fatal("citation changed")
				}
				if tc.name == "agent-service citations" && (len(got.Sources) != 1 || got.Sources[0] != (chat.Source{Number: 2, Title: "Synthetic guide", Page: 3})) {
					t.Fatal("agent-service citation changed")
				}
			} else {
				var got map[string]string
				if json.Unmarshal(r.Body.Bytes(), &got) != nil || len(got) != 2 || got["code"] != tc.code || got["error"] == "" {
					t.Fatal("unsafe error envelope")
				}
			}
			for _, secret := range []string{token, "private-question", "private-upstream-body", "synthetic-service-key", upstream.URL, "Synthetic answer only."} {
				if strings.Contains(f.logs.String(), secret) || (tc.status != 200 && strings.Contains(r.Body.String(), secret)) {
					t.Fatal("private content disclosure")
				}
			}
		})
	}
}
