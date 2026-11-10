package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
	"github.com/gin-gonic/gin"
)

type recoveryFake struct {
	requests, resets int
	nic, password    string
	err              error
	panicCall        bool
	unknown          bool
}

func (f *recoveryFake) Request(ctx context.Context, nic string) (recovery.Receipt, error) {
	f.requests++
	f.nic = nic
	if f.panicCall {
		panic("private failure")
	}

	if f.err != nil {
		return recovery.Receipt{}, f.err
	}

	receipt := recovery.Receipt{Status: "received"}
	if !f.unknown {
		receipt.MaskedEmail = "ci*****@example.test"
	}

	return receipt, nil
}

func (f *recoveryFake) Reset(ctx context.Context, token, password string) error {
	f.resets++
	f.password = password
	return f.err
}

func recoveryRouter(t *testing.T, f *recoveryFake, ipAttempts int) (*gin.Engine, *httpapi.RecoveryHandler) {
	t.Helper()
	makeLimit := func() *httpapi.LocalRegistrationLimiter {
		l, err := httpapi.NewLocalRegistrationLimiter(
			httpapi.RegistrationLimitConfig{
				Key:           []byte(strings.Repeat("k", 32)),
				IPAttempts:    ipAttempts,
				EmailAttempts: 1,
				IPWindow:      time.Minute,
				EmailWindow:   time.Minute,
				MaxKeys:       50,
				MaxConcurrent: 2,
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		return l
	}
	h, err := httpapi.NewRecoveryHandler(
		f,
		makeLimit(),
		makeLimit(),
		httpapi.RecoveryHTTPConfig{Origins: []string{"https://app.example.test"}, Timeout: time.Second},
	)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Any("/request", h.Request)
	router.Any("/reset", h.Reset)
	return router, h
}

func sendRecovery(router http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "https://local.test"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	router.ServeHTTP(result, req)
	return result
}

func TestRecoveryHTTPSuccess(t *testing.T) {
	f := &recoveryFake{}
	router, _ := recoveryRouter(t, f, 100)
	response := sendRecovery(router, "/request", `{"nic":"000000001v"}`)
	if response.Code != 202 ||
		f.nic != "000000001V" ||
		!strings.Contains(response.Body.String(), "ci*****@example.test") ||
		response.Header().Get("Cache-Control") != "no-store" ||
		len(response.Result().Cookies()) != 0 {
		t.Fatal("incorrect request response")
	}

	token, _ := recovery.NewToken()
	body, _ := json.Marshal(
		map[string]string{
			"token":            token.Reveal(),
			"new_password":     " new password ",
			"confirm_password": " new password ",
		},
	)
	response = sendRecovery(router, "/reset", string(body))
	if response.Code != 204 || response.Body.Len() != 0 || f.resets != 1 || f.password != " new password " {
		t.Fatal("incorrect reset result")
	}

	f = &recoveryFake{unknown: true}
	router, _ = recoveryRouter(t, f, 100)
	response = sendRecovery(router, "/request", `{"nic":"000000009V"}`)
	if response.Code != 202 || strings.Contains(response.Body.String(), "masked_email") {
		t.Fatal("unknown account result")
	}
}

func TestRecoveryHTTPParsing(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`null`,
		`[]`,
		`{"nic":null}`,
		`{"nic":2}`,
		`{"NIC":"000000001V"}`,
		`{"nic":"000000001V","nic":"000000002V"}`,
		`{"nic":"000000001V","subject":"caller"}`,
		`{"nic":"000000001V"} {}`,
		`{"nic":"\ud800"}`,
		`{"nic":"bad"}`,
	} {
		f := &recoveryFake{}
		router, _ := recoveryRouter(t, f, 100)
		r := sendRecovery(router, "/request", body)
		if r.Code != 400 || f.requests != 0 {
			t.Fatal("invalid body accepted")
		}
	}

	f := &recoveryFake{}
	router, _ := recoveryRouter(t, f, 100)
	if r := sendRecovery(router, "/request", strings.Repeat("x", 4097)); r.Code != 413 || f.requests != 0 {
		t.Fatal("body limit not enforced")
	}

	token, _ := recovery.NewToken()
	body, _ := json.Marshal(
		map[string]string{"token": token.Reveal(), "new_password": "12345678", "confirm_password": "different"},
	)
	if r := sendRecovery(router, "/reset", string(body)); r.Code != 400 || f.resets != 0 {
		t.Fatal("password mismatch accepted")
	}
}

func TestRecoveryHTTPBoundaryAndErrors(t *testing.T) {
	for _, kind := range []string{"http", "origin", "media", "method", "canceled"} {
		f := &recoveryFake{}
		router, _ := recoveryRouter(t, f, 100)
		req := httptest.NewRequest("POST", "https://local.test/request", strings.NewReader(`{"nic":"000000001V"}`))
		req.Header.Set("Content-Type", "application/json")
		want := 0
		switch kind {
		case "http":
			req.TLS = nil
			req.Header.Set("X-Forwarded-Proto", "https")
			want = 426
		case "origin":
			req.Header.Set("Origin", "https://other.test")
			want = 403
		case "media":
			req.Header.Set("Content-Type", "text/plain")
			want = 415
		case "method":
			req.Method = "GET"
			want = 405
		case "canceled":
			ctx, cancel := context.WithCancel(req.Context())
			cancel()
			req = req.WithContext(ctx)
			want = 503
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want || f.requests != 0 {
			t.Fatal("boundary failed")
		}
	}

	for _, err := range []error{recovery.ErrInvalid, errors.New("private email secret")} {
		f := &recoveryFake{err: err}
		router, _ := recoveryRouter(t, f, 100)
		response := sendRecovery(router, "/request", `{"nic":"000000001V"}`)
		want := 503
		if errors.Is(err, recovery.ErrInvalid) {
			want = 400
		}

		if response.Code != want || strings.Contains(response.Body.String(), "private") {
			t.Fatal("unsafe error response")
		}
	}

	f := &recoveryFake{panicCall: true}
	router, _ := recoveryRouter(t, f, 100)
	if r := sendRecovery(router, "/request", `{"nic":"000000001V"}`); r.Code != 503 ||
		strings.Contains(r.Body.String(), "private") {
		t.Fatal("panic leaked")
	}
}

func TestRecoveryHTTPLimits(t *testing.T) {
	f := &recoveryFake{}
	router, _ := recoveryRouter(t, f, 100)
	if sendRecovery(router, "/request", `{"nic":"000000001v"}`).Code != 202 {
		t.Fatal("first request failed")
	}

	response := sendRecovery(router, "/request", `{"nic":"000000001V"}`)
	if response.Code != 429 || response.Header().Get("Retry-After") != "60" || f.requests != 1 {
		t.Fatal("NIC resend cooldown failed")
	}

	f = &recoveryFake{}
	router, _ = recoveryRouter(t, f, 1)
	_ = sendRecovery(router, "/request", `{"nic":"000000001V"}`)
	req := httptest.NewRequest("POST", "https://local.test/request", strings.NewReader(`{"nic":"000000002V"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "198.51.100.42")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 429 || f.requests != 1 {
		t.Fatal("IP limit bypassed through forwarded header")
	}
}

func TestRecoveryUnicodePasswordAndResetLimit(t *testing.T) {
	f := &recoveryFake{}
	router, _ := recoveryRouter(t, f, 100)
	token, _ := recovery.NewToken()
	password := "valid-password-\uFFFD-தமிழ்"
	body, _ := json.Marshal(
		map[string]string{"token": token.Reveal(), "new_password": password, "confirm_password": password},
	)
	if r := sendRecovery(router, "/reset", string(body)); r.Code != 204 || f.password != password {
		t.Fatal("Unicode password altered")
	}

	if r := sendRecovery(router, "/reset", string(body)); r.Code != 429 || f.resets != 1 {
		t.Fatal("token attempt limit failed")
	}

	f = &recoveryFake{err: recovery.ErrInvalid}
	router, _ = recoveryRouter(t, f, 100)
	if r := sendRecovery(router, "/reset", string(body)); r.Code != 400 ||
		!strings.Contains(r.Body.String(), "invalid_or_expired_reset") {
		t.Fatal("invalid token response")
	}
}

func TestLocalRecoveryDefaults(t *testing.T) {
	h, err := httpapi.NewLocalRecoveryHandler(
		&recoveryFake{},
		[]byte(strings.Repeat("k", 32)),
		httpapi.RecoveryHTTPConfig{Timeout: time.Second},
	)
	if err != nil || h == nil {
		t.Fatal("local defaults failed")
	}

	if _, err = httpapi.NewLocalRecoveryHandler(&recoveryFake{}, nil, httpapi.RecoveryHTTPConfig{Timeout: time.Second}); err == nil {
		t.Fatal("missing rate key accepted")
	}
}
