package handler

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeLogin struct {
	calls int
	err   error
}

func (s *fakeLogin) Login(context.Context, login.Request) (login.Access, error) {
	s.calls++
	if s.err != nil {
		return login.Access{}, s.err
	}
	return login.Access{Token: "synthetic-access-token", ExpiresIn: 300}, nil
}
func TestLoginHTTPContract(t *testing.T) {
	for _, test := range []struct {
		name, body, origin, content string
		tls                         bool
		serviceErr                  error
		want, calls                 int
	}{
		{name: "success", body: `{"email":"citizen@example.test","password":"p"}`, tls: true, want: 200, calls: 1},
		{name: "wrong credentials", body: `{"email":"citizen@example.test","password":"p"}`, tls: true, serviceErr: login.ErrFailed, want: 401, calls: 1},
		{name: "outage", body: `{"email":"citizen@example.test","password":"p"}`, tls: true, serviceErr: errors.New("private-upstream-response"), want: 503, calls: 1},
		{name: "no TLS", body: `{}`, want: 426},
		{name: "browser blocked", body: `{}`, tls: true, origin: "https://web.example.test", want: 403},
		{name: "unknown fields", body: `{"email":"a@example.test","password":"p","scope":"admin"}`, tls: true, want: 400},
		{name: "duplicate", body: `{"email":"a@example.test","password":"p","password":"q"}`, tls: true, want: 400},
		{name: "size", body: strings.Repeat("x", 4097), tls: true, want: 413},
		{name: "wrong media", body: `{}`, tls: true, content: "text/plain", want: 415},
		{name: "unknown media parameter", body: `{}`, tls: true, content: "application/json; unsupported=x", want: 415},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeLogin{err: test.serviceErr}
			limiter, _ := NewLocalRegistrationLimiter(RegistrationLimitConfig{Key: []byte(strings.Repeat("k", 32)), IPAttempts: 5, EmailAttempts: 5, IPWindow: time.Minute, EmailWindow: time.Minute, MaxKeys: 20, MaxConcurrent: 2})
			h, _ := NewLoginHandler(service, limiter)
			router := NewRouterWithAuth(nil, h)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(test.body))
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("Content-Type", "application/json")
			if test.content != "" {
				req.Header.Set("Content-Type", test.content)
			}
			req.Header.Set("Origin", test.origin)
			req.Header.Set("X-Forwarded-Proto", "https")
			if test.tls {
				req.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != test.want || service.calls != test.calls {
				t.Fatalf("status=%d calls=%d", w.Code, service.calls)
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-ID") == "" || len(w.Result().Cookies()) != 0 {
				t.Fatal("unsafe response metadata")
			}
			for _, private := range []string{"refresh_token", "id_token", "private-upstream-response"} {
				if strings.Contains(w.Body.String(), private) {
					t.Fatal("unwanted response data")
				}
			}
		})
	}
}
func TestLoginRateLimitStopsForwarding(t *testing.T) {
	service := &fakeLogin{err: login.ErrFailed}
	limiter, _ := NewLocalRegistrationLimiter(RegistrationLimitConfig{Key: []byte(strings.Repeat("k", 32)), IPAttempts: 5, EmailAttempts: 5, IPWindow: time.Minute, EmailWindow: time.Minute, MaxKeys: 20, MaxConcurrent: 2})
	h, _ := NewLoginHandler(service, limiter)
	router := NewRouterWithAuth(nil, h)
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"a@example.test","password":"p"}`))
		req.RemoteAddr = "127.0.0.1:1234"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if i < 5 && w.Code != 401 {
			t.Fatal("credential error changed")
		}
		if i == 5 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("limit missing")
		}
	}
	if service.calls != 5 {
		t.Fatal("throttled request forwarded")
	}
	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://web.example.test")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("browser preflight allowed")
	}
}
