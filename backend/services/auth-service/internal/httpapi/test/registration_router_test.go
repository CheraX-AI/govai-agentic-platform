package httpapi_test

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

type routerRegistrationService struct{ calls int }

func (s *routerRegistrationService) Register(context.Context, string, io.Reader) (registration.Receipt, error) {
	s.calls++
	return registration.Receipt{Status: "received"}, nil
}

func TestRegistrationRouterTLSAndCORS(t *testing.T) {
	service := &routerRegistrationService{}
	limiter, e := httpapi.NewLocalRegistrationLimiter(
		httpapi.RegistrationLimitConfig{
			Key:           []byte(strings.Repeat("k", 32)),
			IPAttempts:    20,
			EmailAttempts: 20,
			IPWindow:      time.Minute,
			EmailWindow:   time.Minute,
			MaxKeys:       100,
			MaxConcurrent: 2,
		},
	)
	if e != nil {
		t.Fatal(e)
	}

	h, e := httpapi.NewRegistrationHandler(
		service,
		limiter,
		httpapi.RegistrationHTTPConfig{Timeout: time.Second, UnavailableRetryAfter: time.Second},
	)
	if e != nil {
		t.Fatal(e)
	}

	router := httpapi.NewRouter(httpapi.RouterOptions{
		CORSOrigins:  []string{"https://web.example.test"},
		Registration: h,
	})
	path := "/api/v1/registrations"
	for _, test := range []struct {
		name      string
		secure    bool
		forwarded string
		want      int
	}{
		{"plain HTTP", false, "", 426},
		{"untrusted forwarding header", false, "https", 426},
		{"direct TLS", true, "", 202},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(
				"POST",
				path,
				strings.NewReader(
					`{"full_name":"Synthetic Citizen","nic":"200000000001","mobile_number":"+94771234567","email":"synthetic@example.test","password":"Example8!"}`,
				),
			)
			req.RemoteAddr = "127.0.0.1:12345"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "11111111-1111-1111-1111-111111111111")
			req.Header.Set("X-Forwarded-Proto", test.forwarded)
			if test.secure {
				req.TLS = &tls.ConnectionState{}
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != test.want {
				t.Fatalf("status=%d", w.Code)
			}
		})
	}

	if service.calls != 1 {
		t.Fatal("plaintext request reached registration service")
	}

	for _, test := range []struct {
		name, path, origin, method, headers string
		want                                int
	}{
		{"allowed", path, "https://web.example.test", "POST", "Content-Type, Idempotency-Key", 204},
		{"wrong origin", path, "https://bad.example.test", "POST", "Content-Type", 403},
		{"wrong method", path, "https://web.example.test", "DELETE", "Content-Type", 403},
		{"unapproved header", path, "https://web.example.test", "POST", "X-Unsafe", 403},
		{
			"health remains GET only",
			"/api/health",
			"https://web.example.test",
			"POST",
			"Content-Type",
			403,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, test.path, nil)
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Access-Control-Request-Method", test.method)
			req.Header.Set("Access-Control-Request-Headers", test.headers)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != test.want {
				t.Fatalf("status=%d", w.Code)
			}
		})
	}

	req := httptest.NewRequest(http.MethodPost, path, nil)
	w := httptest.NewRecorder()
	httpapi.NewRouter(httpapi.RouterOptions{}).ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("disabled registration route enabled")
	}
}
