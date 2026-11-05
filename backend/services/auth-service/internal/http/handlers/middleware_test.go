package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware(t *testing.T) {
	for _, tc := range []struct {
		method, path, origin, preflight, headers string
		status                                   int
	}{
		{"GET", "/health", "http://localhost:5173", "", "", 200},
		{"GET", "/health", "http://localhost:5173.evil", "", "", 403},
		{"OPTIONS", "/health", "http://localhost:5173", "GET", "Authorization, Content-Type", 204},
		{"OPTIONS", "/health", "http://localhost:5173", "DELETE", "", 403},
		{"OPTIONS", "/health", "http://localhost:5173", "GET", "X-Override", 403},
		{"POST", "/health", "", "", "", 405},
		{"GET", "/missing?token=private", "", "", "", 404},
		{"GET", "/panic", "", "", "", 500},
	} {
		t.Run(tc.method+tc.path+tc.origin+tc.preflight+tc.headers, func(t *testing.T) {
			var logs bytes.Buffer
			router := newRouter(slog.New(slog.NewJSONHandler(&logs, nil)), []string{"http://localhost:5173"})
			router.GET("/panic", func(c *gin.Context) { panic("private") })
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("private"))
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Authorization", "Bearer private")
			req.Header.Set("X-Request-ID", "private")
			req.Header.Set("Access-Control-Request-Method", tc.preflight)
			req.Header.Set("Access-Control-Request-Headers", tc.headers)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}

			id := response.Header().Get("X-Request-ID")
			if len(id) != 32 || !strings.Contains(logs.String(), id) {
				t.Fatal("missing request ID/log")
			}

			if strings.Contains(logs.String(), "private") {
				t.Fatal("sensitive log")
			}

			if tc.status >= 400 {
				var body map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body["error"] == "" {
					t.Fatal("invalid JSON error")
				}
			}

			if tc.origin == "http://localhost:5173" &&
				response.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("missing CORS origin")
			}

			if tc.origin != "http://localhost:5173" &&
				response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unexpected CORS origin")
			}
		})
	}
}
