package handler

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/chatgateway"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/websession"
)

type chatSessionDouble struct {
	browserDouble
	denied bool
}

func (s *chatSessionDouble) WithAccess(ctx context.Context, raw, csrf string, f func(context.Context, string) error) error {
	if s.denied || csrf != syntheticWebSecret {
		return websession.ErrDenied
	}

	return f(ctx, "synthetic-access")
}

func TestWebChatEndpoint(t *testing.T) {
	for _, mode := range []string{
		"success",
		"no cookie",
		"no csrf",
		"wrong csrf",
		"wrong origin",
		"no tls",
		"authorization header",
		"empty",
		"duplicate",
		"unknown field",
		"oversized",
		"wrong media",
		"denied session",
		"bad response",
		"unsafe source",
		"upstream failure",
		"upstream timeout",
		"preflight",
		"disabled",
	} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer synthetic-access" ||
					r.Header.Get("Cookie") != "" {
					t.Error("credential boundary")
				}

				body, _ := io.ReadAll(r.Body)
				if string(body) != `{"message":"Synthetic question"}` {
					t.Error("request changed")
				}

				w.Header().Set("Content-Type", "application/json")
				switch mode {
				case "upstream failure":
					w.WriteHeader(503)
					return
				case "upstream timeout":
					w.WriteHeader(504)
					return
				case "bad response":
					io.WriteString(w, `{"answer":"x","answer":"y"}`)
					return
				case "unsafe source":
					io.WriteString(w, `{"answer":"x","sources":[{"number":1,"title":"x","url":"javascript:bad"}]}`)
					return
				}

				io.WriteString(
					w,
					`{"answer":"Synthetic answer [1]","sources":[{"number":1,"title":"Reference","page":3},{"number":2,"title":"Guide","url":"https://example.test/guide"}]}`,
				)
			}))
			defer upstream.Close()
			roots := x509.NewCertPool()
			roots.AddCert(upstream.Certificate())
			client, err := chatgateway.New(upstream.URL+"/api/chat/message", roots, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			defer client.Close()
			s := &chatSessionDouble{denied: mode == "denied session"}
			limit, err := NewLocalRegistrationLimiter(
				RegistrationLimitConfig{
					Key:           make([]byte, 32),
					IPAttempts:    60,
					EmailAttempts: 1,
					IPWindow:      time.Minute,
					EmailWindow:   time.Minute,
					MaxKeys:       10,
					MaxConcurrent: 3,
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			h, err := NewWebSessionHandler(s, limit, "https://govai.test")
			if err != nil {
				t.Fatal(err)
			}

			if mode != "disabled" {
				if err = h.EnableChat(client); err != nil {
					t.Fatal(err)
				}
			}

			router := NewRouterWithWeb(nil, nil, nil, nil, h)
			body := `{"message":"Synthetic question"}`
			want := 200
			expectedCalls := 1
			switch mode {
			case "no cookie", "denied session", "wrong csrf":
				want = 401
				expectedCalls = 0
			case "no csrf", "wrong origin", "authorization header":
				want = 403
				expectedCalls = 0
			case "no tls":
				want = 426
				expectedCalls = 0
			case "empty":
				body = `{"message":" "}`
				want = 400
				expectedCalls = 0
			case "duplicate":
				body = `{"message":"x","message":"y"}`
				want = 400
				expectedCalls = 0
			case "unknown field":
				body = `{"message":"x","user_id":"other"}`
				want = 400
				expectedCalls = 0
			case "oversized":
				body = strings.Repeat("x", 17000)
				want = 413
				expectedCalls = 0
			case "wrong media":
				want = 415
				expectedCalls = 0
			case "bad response", "unsafe source":
				want = 502
			case "upstream failure":
				want = 503
			case "upstream timeout":
				want = 504
			case "disabled":
				want = 403
				expectedCalls = 0
			case "preflight":
				want = 204
				expectedCalls = 0
			}

			req := httptest.NewRequest("POST", "https://govai.test/api/v1/web/chat", strings.NewReader(body))
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("Origin", "https://govai.test")
			req.Header.Set("X-GovAI-Web", "1")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", syntheticWebSecret)
			if mode != "no cookie" {
				req.AddCookie(&http.Cookie{Name: WebSessionCookie, Value: syntheticWebSecret})
			}

			switch mode {
			case "no csrf":
				req.Header.Del("X-CSRF-Token")
			case "wrong csrf":
				req.Header.Set("X-CSRF-Token", strings.Repeat("A", 42)+"E")
			case "wrong origin":
				req.Header.Set("Origin", "https://other.test")
			case "no tls":
				req.TLS = nil
			case "authorization header":
				req.Header.Set("Authorization", "Bearer untrusted")
			case "wrong media":
				req.Header.Set("Content-Type", "text/plain")
			case "preflight":
				req.Method = "OPTIONS"
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-GovAI-Web, X-CSRF-Token")
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != want || calls != expectedCalls {
				t.Fatalf("status=%d want=%d upstream=%d want=%d", w.Code, want, calls, expectedCalls)
			}

			if strings.Contains(w.Body.String(), "synthetic-access") {
				t.Fatal("token exposed")
			}

			if mode == "success" &&
				(!strings.Contains(w.Body.String(), `{"number":1,"title":"Reference","page":3}`) ||
					!strings.Contains(w.Body.String(), `{"number":2,"title":"Guide","url":"https://example.test/guide"}`)) {
				t.Fatal("source lost")
			}

			if mode != "disabled" && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cache policy missing")
			}
		})
	}
}

func TestEnableChatRequiresServerForwarder(t *testing.T) {
	h := &WebSessionHandler{service: &browserDouble{}}
	if err := h.EnableChat(nil); !errors.Is(err, chatgateway.ErrUnavailable) {
		t.Fatal("unsafe dependency accepted")
	}
}
