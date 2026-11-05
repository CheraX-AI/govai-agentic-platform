package handler

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/gin-gonic/gin"
)

type nativeSessionDouble struct {
	calls int
	err   error
}

func (s *nativeSessionDouble) RefreshNative(ctx context.Context, raw string) (login.SessionTokens, error) {
	s.calls++
	if raw != "synthetic-old" {
		return login.SessionTokens{}, errors.New("private invalid credential")
	}

	return login.SessionTokens{
		AccessToken:      "synthetic-access",
		RefreshToken:     "synthetic-next",
		ExpiresIn:        300,
		RefreshExpiresIn: 1800,
	}, s.err
}

func (s *nativeSessionDouble) RevokeNative(context.Context, string) error {

	s.calls++
	return s.err
}

func TestNativeSessionHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		revoke     bool
		provider   error
	}{
		{"refresh", `{"refresh_token":"synthetic-old"}`, 200, false, nil},
		{"revoke", `{"refresh_token":"synthetic-old"}`, 204, true, nil},
		{"duplicate", `{"refresh_token":"synthetic-old","refresh_token":"other"}`, 400, false, nil},
		{"unknown", `{"refresh_token":"synthetic-old","client_id":"admin"}`, 400, false, nil},
		{"null", `{"refresh_token":null}`, 400, false, nil},
		{"trailing", `{"refresh_token":"synthetic-old"}{}`, 400, false, nil},
		{"empty", `{"refresh_token":""}`, 400, false, nil},
		{"bad unicode", `{"refresh_token":"\ud800"}`, 400, false, nil},
		{"oversize", strings.Repeat(" ", login.MaxRefreshRequestBytes+1), 413, false, nil},
		{"invalid grant", `{"refresh_token":"synthetic-old"}`, 401, false, login.ErrFailed},
		{
			"private error",
			`{"refresh_token":"synthetic-old"}`,
			503,
			false,
			errors.New("private database detail"),
		},
		{"HTTP", `{"refresh_token":"synthetic-old"}`, 426, false, nil},
		{"origin", `{"refresh_token":"synthetic-old"}`, 403, false, nil},
		{"cookie", `{"refresh_token":"synthetic-old"}`, 403, false, nil},
		{"media", `{"refresh_token":"synthetic-old"}`, 415, false, nil},
		{"cancel", `{"refresh_token":"synthetic-old"}`, 503, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &nativeSessionDouble{err: tc.provider}
			limiter, err := NewLocalRegistrationLimiter(
				RegistrationLimitConfig{
					Key:           make([]byte, 32),
					IPAttempts:    10,
					EmailAttempts: 10,
					IPWindow:      time.Minute,
					EmailWindow:   time.Minute,
					MaxKeys:       32,
					MaxConcurrent: 2,
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			h, err := NewNativeSessionHandler(service, limiter)
			if err != nil {
				t.Fatal(err)
			}

			router := gin.New()
			route := h.Refresh
			if tc.revoke {
				route = h.Revoke
			}

			router.POST("/session", route)
			request := httptest.NewRequest("POST", "https://localhost/session", strings.NewReader(tc.body))
			request.TLS = &tls.ConnectionState{}
			request.RemoteAddr = "127.0.0.1:1234"
			request.Header.Set("Content-Type", "application/json")
			switch tc.name {
			case "HTTP":
				request.TLS = nil
			case "origin":
				request.Header.Set("Origin", "https://web.test")
			case "cookie":
				request.Header.Set("Cookie", "session=synthetic")
			case "media":
				request.Header.Set("Content-Type", "text/plain")
			case "cancel":
				ctx, cancel := context.WithCancel(request.Context())
				cancel()
				request = request.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			if w.Code != tc.status {
				t.Fatalf("status %d expected %d", w.Code, tc.status)
			}

			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("incorrect custody headers")
			}

			if tc.status >= 400 &&
				(strings.Contains(w.Body.String(), "private") ||
					strings.Contains(w.Body.String(), "synthetic-")) {
				t.Fatal("sensitive error response")
			}

			if tc.status == 400 ||
				tc.status == 413 ||
				tc.status == 415 ||
				tc.status == 426 ||
				tc.status == 403 ||
				tc.name == "cancel" {
				if service.calls != 0 {
					t.Fatal("invalid request reached provider")
				}
			}

			if tc.status == 204 && w.Body.Len() != 0 {
				t.Fatal("revoke body must be empty")
			}
		})
	}
}

func TestNativeSessionRateLimit(t *testing.T) {
	service := &nativeSessionDouble{}
	limiter, _ := NewLocalRegistrationLimiter(
		RegistrationLimitConfig{
			Key:           make([]byte, 32),
			IPAttempts:    1,
			EmailAttempts: 1,
			IPWindow:      time.Minute,
			EmailWindow:   time.Minute,
			MaxKeys:       2,
			MaxConcurrent: 1,
		},
	)
	h, _ := NewNativeSessionHandler(service, limiter)
	router := gin.New()
	router.POST("/session", h.Refresh)
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(
			http.MethodPost,
			"https://localhost/session",
			strings.NewReader(`{"refresh_token":"synthetic-old"}`),
		)
		r.TLS = &tls.ConnectionState{}
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if i == 1 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("rate limit missing")
		}
	}

	if service.calls != 1 {
		t.Fatal("rate limit bypass")
	}
}
