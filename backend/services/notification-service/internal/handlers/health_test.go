package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestFoundationHealth prevents broker connectivity from implying pipeline readiness.

func TestFoundationHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, brokerConnected := range []bool{false, true} {
		router := Routes(func() bool { return brokerConnected })
		for _, path := range []string{"/health", "/api/health", "/api/ready"} {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			expected := http.StatusOK
			if path == "/api/ready" || (path == "/api/health" && !brokerConnected) {
				expected = http.StatusServiceUnavailable
			}
			if response.Code != expected {
				t.Fatalf("%s: expected %d, got %d", path, expected, response.Code)
			}
			var body map[string]any
			if json.Unmarshal(response.Body.Bytes(), &body) != nil {
				t.Fatal("health response is not JSON")
			}
			if path == "/api/health" && body["processing_enabled"] != false {
				t.Fatal("foundation incorrectly claims event processing is enabled")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health response can be cached")
			}
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/api/notifications", nil))
		if response.Code != http.StatusNotFound {
			t.Fatal("citizen notification API should not exist in the foundation")
		}
	}
}
