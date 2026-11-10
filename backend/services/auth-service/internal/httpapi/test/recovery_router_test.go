package httpapi_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
)

func TestRecoveryRuntimeRoutesAndCORS(t *testing.T) {
	service := &recoveryFake{}
	_, h := recoveryRouter(t, service, 100)
	disabled := httpapi.NewRouter(httpapi.RouterOptions{})
	if response := sendRecovery(disabled, "/api/v1/auth/forgot-password", `{"nic":"000000001V"}`); response.Code != 404 {
		t.Fatal("disabled route registered")
	}

	router := httpapi.NewRouter(httpapi.RouterOptions{Recovery: h})
	response := sendRecovery(router, "/api/v1/auth/forgot-password", `{"nic":"000000001V"}`)
	if response.Code != 202 || response.Header().Get("X-Request-ID") == "" {
		t.Fatal("enabled route failed")
	}

	for _, tc := range []struct {
		origin, method, header string
		status                 int
	}{
		{"https://app.example.test", "POST", "Content-Type", 204},
		{"https://other.test", "POST", "Content-Type", 403},
		{"https://app.example.test", "GET", "Content-Type", 403},
		{"https://app.example.test", "POST", "Authorization", 403},
	} {
		request := httptest.NewRequest("OPTIONS", "https://local.test/api/v1/auth/reset-password", nil)
		request.Header.Set("Origin", tc.origin)
		request.Header.Set("Access-Control-Request-Method", tc.method)
		request.Header.Set("Access-Control-Request-Headers", tc.header)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != tc.status {
			t.Fatal("preflight status mismatch")
		}

		if tc.status == 204 &&
			(recorder.Header().Get("Access-Control-Allow-Origin") != tc.origin ||
				recorder.Header().Get("Access-Control-Allow-Credentials") != "" ||
				!strings.Contains(recorder.Header().Get("Access-Control-Expose-Headers"), "Retry-After")) {
			t.Fatal("incorrect recovery CORS policy")
		}
	}
}
