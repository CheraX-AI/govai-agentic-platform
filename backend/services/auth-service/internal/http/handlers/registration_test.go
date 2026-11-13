package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/gin-gonic/gin"
)

const httpRegistrationFixture = `{"full_name":"Synthetic Citizen","nic":"123456789v","mobile_number":"+94771234567","email":"Citizen@EXAMPLE.TEST","password":" synthetic secret "}`

type registrationFunc func(context.Context, string, io.Reader) (registration.Receipt, error)

func (f registrationFunc) Register(c context.Context, k string, r io.Reader) (registration.Receipt, error) {
	return f(c, k, r)
}

func limitConfig() RegistrationLimitConfig {
	return RegistrationLimitConfig{
		Key:           []byte(strings.Repeat("k", 32)),
		IPAttempts:    100,
		EmailAttempts: 100,
		IPWindow:      time.Minute,
		EmailWindow:   time.Hour,
		MaxKeys:       32,
		MaxConcurrent: 4,
	}
}

func testRegistrationEngine(t *testing.T, f registrationFunc, l RegistrationLimiter, timeout time.Duration) *gin.Engine {
	t.Helper()
	if l == nil {
		var err error
		l, err = NewLocalRegistrationLimiter(limitConfig())
		if err != nil {
			t.Fatal(err)
		}
	}

	h, err := NewRegistrationHandler(f, l, RegistrationHTTPConfig{Timeout: timeout, UnavailableRetryAfter: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {

		c.Header("X-Request-ID", "trusted-test-id")
		c.Next()
	})
	r.Any("/registration", h.Handle)
	return r
}

func registrationRequest(body string) *http.Request {
	r := httptest.NewRequest("POST", "/registration", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "ABCDEF00-0000-4000-8000-000000000001")
	r.Header.Set("X-Request-ID", "forged")
	r.RemoteAddr = "192.0.2.1:1234"
	return r
}

func acceptedRegistration(context.Context, string, io.Reader) (registration.Receipt, error) {
	return registration.Receipt{Status: "received"}, nil
}

func TestRegistrationHTTPInput(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		code       string
	}{
		{"success", httpRegistrationFixture, 202, "received"},
		{"malformed", "{", 400, "invalid_request"},
		{"trailing", httpRegistrationFixture + "{}", 400, "invalid_request"},
		{
			"unknown",
			strings.Replace(httpRegistrationFixture, `"full_name"`, `"role"`, 1),
			400,
			"invalid_request",
		}, {
			"validation",
			strings.Replace(httpRegistrationFixture, "123456789v", "invalid", 1),
			422,
			"validation_failed",
		}, {"oversize", strings.Repeat("x", registration.MaxRequestBytes+1), 413, "too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			r := testRegistrationEngine(t,
				func(ctx context.Context, key string, b io.Reader) (registration.Receipt, error) {
					calls++
					p, e := registration.ParseRequest(b)
					if e != nil ||
						p.TransientPassword() != " synthetic secret " ||
						key != "abcdef00-0000-4000-8000-000000000001" {
						t.Fatal("service input changed")
					}

					return acceptedRegistration(ctx, key, b)
				}, nil, time.Second)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, registrationRequest(tc.body))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) {
				t.Fatalf("status/code mismatch: %d", w.Code)
			}

			if w.Header().Get("Cache-Control") != "no-store" ||
				w.Header().Get("X-Request-ID") != "trusted-test-id" {
				t.Fatal("missing response safeguards")
			}

			if tc.status != 202 && calls != 0 {
				t.Fatal("invalid input reached service")
			}

			if strings.Contains(w.Body.String(), "synthetic secret") ||
				strings.Contains(w.Body.String(), "Citizen@") {
				t.Fatal("input leaked")
			}
		})
	}
}

func TestRegistrationHTTPHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*http.Request)
		status int
	}{
		{"content type", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415}, {
			"charset",
			func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=utf-16") },
			415,
		},
		{"encoding", func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }, 415},
		{"missing key", func(r *http.Request) { r.Header.Del("Idempotency-Key") }, 400},
		{
			"duplicate key",
			func(r *http.Request) { r.Header.Add("Idempotency-Key", "another") },
			400,
		}, {"unknown length oversized", func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", registration.MaxRequestBytes+1)))
			r.ContentLength = -1
		}, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testRegistrationEngine(t,
				func(context.Context, string, io.Reader) (registration.Receipt, error) {
					t.Fatal("service called")
					return registration.Receipt{}, nil
				}, nil, time.Second)
			req := registrationRequest(httpRegistrationFixture)
			tc.edit(req)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestRegistrationHTTPOutcomes(t *testing.T) {
	var received string
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"accepted", nil, 202},
		{"duplicate", registration.ErrNICConflict, 202},
		{"conflict", &registration.ServiceError{Code: "registration_request_conflict"}, 409},
		{"policy", &registration.ServiceError{Code: "registration_policy_rejected"}, 422},
		{"pending", &registration.ServiceError{Code: "registration_verification_pending"}, 503},
		{
			"ambiguous",
			&registration.ServiceError{Code: "registration_reconciliation_required"},
			503,
		},
		{"dependency", errors.New("secret upstream personal data"), 503},
		{"canceled", context.Canceled, 503},
		{
			"unsafe fields",
			&registration.RequestError{
				Code:   "validation_failed",
				Fields: map[string]string{"email": "secret upstream personal data", "secret": "required", "nic": "invalid_format"},
			},
			422,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testRegistrationEngine(t,
				func(context.Context, string, io.Reader) (registration.Receipt, error) {
					return registration.Receipt{Status: "received"}, tc.err
				}, nil, time.Second)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, registrationRequest(httpRegistrationFixture))
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}

			if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "upstream") {
				t.Fatal("unsafe response")
			}

			if tc.name == "accepted" {
				received = w.Body.String()
			}

			if tc.name == "duplicate" && w.Body.String() != received {
				t.Fatal("enumerating duplicate response")
			}

			if tc.status == 503 && w.Header().Get("Retry-After") != "3" {
				t.Fatal("missing retry advice")
			}
		})
	}
}

func TestRegistrationHTTPCancellation(t *testing.T) {
	for _, pre := range []bool{true, false} {
		t.Run(map[bool]string{true: "pre-canceled", false: "deadline"}[pre], func(t *testing.T) {
			called := false
			r := testRegistrationEngine(t,
				func(ctx context.Context, _ string, _ io.Reader) (registration.Receipt, error) {
					called = true
					<-ctx.Done()
					return registration.Receipt{}, ctx.Err()
				}, nil, 10*time.Millisecond)
			req := registrationRequest(httpRegistrationFixture)
			if pre {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 503 || called == pre {
				t.Fatal("cancellation not propagated")
			}
		})
	}
}

func TestRegistrationHTTPRateLimits(t *testing.T) {
	for _, email := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "canonical email"}[email], func(t *testing.T) {
			cfg := limitConfig()
			if email {
				cfg.EmailAttempts = 1
			} else {
				cfg.IPAttempts = 1
			}

			l, _ := NewLocalRegistrationLimiter(cfg)
			r := testRegistrationEngine(t, acceptedRegistration, l, time.Second)
			for i := 0; i < 2; i++ {
				req := registrationRequest(httpRegistrationFixture)
				if email && i == 1 {
					req.RemoteAddr = "192.0.2.2:1234"
					req.Body = io.NopCloser(
						strings.NewReader(
							strings.Replace(httpRegistrationFixture, "Citizen@EXAMPLE.TEST", "citizen@example.test", 1),
						),
					)
				}

				req.Header.Set("X-Forwarded-For", []string{"198.51.100.1", "198.51.100.2"}[i])
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				want := 202
				if i == 1 {
					want = 429
				}

				if w.Code != want {
					t.Fatalf("status %d", w.Code)
				}

				if i == 1 && w.Header().Get("Retry-After") == "" {
					t.Fatal("missing retry")
				}
			}
		})
	}
}

type failedRegistrationLimiter struct{}

func (failedRegistrationLimiter) Acquire(context.Context, string) (RegistrationPermit, time.Duration, error) {
	return nil, 0, errors.New("private limiter failure")
}

type failedRegistrationReader struct{}

func (failedRegistrationReader) Read([]byte) (int, error) {
	return 0, errors.New("private reader failure")
}

func (failedRegistrationReader) Close() error { return nil }

func TestRegistrationHTTPBoundaryFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		service registrationFunc
		limiter RegistrationLimiter
		reader  bool
		status  int
	}{
		{"limiter unavailable", acceptedRegistration, failedRegistrationLimiter{}, false, 503},
		{"reader failure", acceptedRegistration, nil, true, 400},
		{
			"panic",
			func(context.Context, string, io.Reader) (registration.Receipt, error) { panic("private panic") },
			nil,
			false,
			503,
		},
		{"unexpected receipt", func(context.Context, string, io.Reader) (registration.Receipt, error) {
			return registration.Receipt{Status: "private"}, nil
		}, nil, false, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testRegistrationEngine(t, tc.service, tc.limiter, time.Second)
			req := registrationRequest(httpRegistrationFixture)
			if tc.reader {
				req.Body = failedRegistrationReader{}
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "private") {
				t.Fatal("unsafe boundary outcome")
			}
		})
	}
}

func TestRegistrationHTTPConcurrentCapacity(t *testing.T) {
	cfg := limitConfig()
	cfg.MaxConcurrent = 1
	l, _ := NewLocalRegistrationLimiter(cfg)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 1)
	r := testRegistrationEngine(t, func(context.Context, string, io.Reader) (registration.Receipt, error) {
		entered <- struct{}{}
		<-release
		return registration.Receipt{Status: "received"}, nil
	}, l, time.Second)
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, registrationRequest(httpRegistrationFixture))
		done <- w
	}()
	<-entered
	w := httptest.NewRecorder()
	r.ServeHTTP(w, registrationRequest(httpRegistrationFixture))
	if w.Code != 429 {
		t.Errorf("concurrent admission: %d", w.Code)
	}

	close(release)
	if first := <-done; first.Code != 202 {
		t.Fatal("first request failed")
	}

	if l.inflight != 0 {
		t.Fatal("permit leaked")
	}
}
