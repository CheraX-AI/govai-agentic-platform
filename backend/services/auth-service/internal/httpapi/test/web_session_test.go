package httpapi_test

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/websession"
	"github.com/gin-gonic/gin"
)

var syntheticWebSecret = base64.RawURLEncoding.EncodeToString(make([]byte, 32))

type browserDouble struct {
	calls int
	err   error
}

func (s *browserDouble) Start(context.Context, login.Request) (websession.Started, error) {
	s.calls++
	return websession.Started{Cookie: syntheticWebSecret, CSRF: syntheticWebSecret, Expires: time.Now().Add(time.Hour)}, s.err
}

func (s *browserDouble) Check(context.Context, string) (websession.Identity, string, error) {
	s.calls++
	return websession.Identity{Subject: "synthetic-subject"}, syntheticWebSecret, s.err
}

func (s *browserDouble) CSRF(context.Context, string) (string, error) {
	s.calls++
	return syntheticWebSecret, s.err
}

func (s *browserDouble) Refresh(context.Context, string, string) error {

	s.calls++
	return s.err
}

func (s *browserDouble) End(context.Context, string, string) error {

	s.calls++
	return s.err
}

// browserHandler builds a test handler through its public constructor.
func browserHandler(t *testing.T, s *browserDouble) *httpapi.WebSessionHandler {
	t.Helper()
	l, err := httpapi.NewLocalRegistrationLimiter(
		httpapi.RegistrationLimitConfig{
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

	h, err := httpapi.NewWebSessionHandler(s, l, "https://govai.test")
	if err != nil {
		t.Fatal(err)
	}

	return h
}

func browserRouter(t *testing.T, s *browserDouble) *gin.Engine {
	t.Helper()
	h := browserHandler(t, s)
	r := gin.New()
	r.POST("/login", h.Login)
	r.GET("/session", h.Check)
	r.GET("/csrf", h.CSRF)
	r.POST("/refresh", h.Refresh)
	r.POST("/logout", h.Logout)
	return r
}

func browserRequest(path string) *http.Request {
	method := "POST"
	body := ""
	if path == "/login" {
		body = `{"email":"synthetic@example.test","password":"synthetic-password"}`
	}

	if path == "/session" || path == "/csrf" {
		method = "GET"
	}

	r := httptest.NewRequest(method, "https://govai.test"+path, strings.NewReader(body))
	r.TLS = &tls.ConnectionState{}
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Origin", "https://govai.test")
	r.Header.Set("X-GovAI-Web", "1")
	r.Header.Set("Content-Type", "application/json")
	if path != "/login" {
		r.AddCookie(&http.Cookie{Name: httpapi.WebSessionCookie, Value: syntheticWebSecret})
		r.Header.Set("X-CSRF-Token", syntheticWebSecret)
	}

	return r
}

func TestBrowserCookieLifecycle(t *testing.T) {
	s := &browserDouble{}
	r := browserRouter(t, s)
	for _, path := range []string{"/login", "/session", "/csrf", "/refresh", "/logout"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, browserRequest(path))
		want := 200
		if path == "/refresh" || path == "/logout" {
			want = 204
		}

		if w.Code != want {
			t.Fatalf("%s status %d", path, w.Code)
		}

		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable session")
		}

		if strings.Contains(w.Body.String(), "access_token") ||
			strings.Contains(w.Body.String(), "refresh_token") {
			t.Fatal("provider token response")
		}

		cookies := w.Result().Cookies()
		if path == "/login" || path == "/logout" {
			if len(cookies) != 1 {
				t.Fatal("cookie missing")
			}

			c := cookies[0]
			if c.Name != httpapi.WebSessionCookie ||
				!c.Secure ||
				!c.HttpOnly ||
				c.SameSite != http.SameSiteStrictMode ||
				c.Path != "/" ||
				c.Domain != "" {
				t.Fatal("unsafe cookie")
			}

			if path == "/logout" && c.MaxAge != -1 {
				t.Fatal("cookie not cleared")
			}
		}
	}
}

func TestBrowserRequestBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		edit       func(*http.Request)
		status     int
	}{
		{"missing origin", "/login", func(r *http.Request) { r.Header.Del("Origin") }, 403},
		{
			"wrong origin",
			"/logout",
			func(r *http.Request) { r.Header.Set("Origin", "https://evil.test") },
			403,
		},
		{
			"duplicate origin",
			"/login",
			func(r *http.Request) { r.Header.Add("Origin", "https://govai.test") },
			403,
		},
		{"no custom header", "/csrf", func(r *http.Request) { r.Header.Del("X-GovAI-Web") }, 403},
		{
			"cross site",
			"/session",
			func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
			403,
		},
		{"HTTP", "/login", func(r *http.Request) { r.TLS = nil }, 426},
		{
			"bearer confused",
			"/session",
			func(r *http.Request) { r.Header.Set("Authorization", "Bearer synthetic") },
			403,
		},
		{"no csrf", "/refresh", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, 403},
		{
			"double csrf",
			"/logout",
			func(r *http.Request) { r.Header.Add("X-CSRF-Token", syntheticWebSecret) },
			403,
		},
		{
			"duplicate cookie",
			"/session",
			func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: httpapi.WebSessionCookie, Value: syntheticWebSecret})
			},
			401,
		},
		{
			"login fixation",
			"/login",
			func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: httpapi.WebSessionCookie, Value: syntheticWebSecret})
			},
			409,
		},
		{"write body", "/refresh", func(r *http.Request) { r.ContentLength = 1 }, 400},
		{
			"media",
			"/login",
			func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
			415,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &browserDouble{}
			router := browserRouter(t, s)
			request := browserRequest(tc.path)
			tc.edit(request)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			if w.Code != tc.status || s.calls != 0 {
				t.Fatalf("boundary failed status=%d calls=%d", w.Code, s.calls)
			}

			if w.Header().Get("Set-Cookie") != "" {
				t.Fatal("cookie changed on rejected request")
			}
		})
	}
}

func TestBrowserPendingRevocationClearsCookie(t *testing.T) {
	s := &browserDouble{err: websession.ErrRevocationPending}
	w := httptest.NewRecorder()
	browserRouter(t, s).ServeHTTP(w, browserRequest("/logout"))
	if w.Code != 503 || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("ended local cookie retained")
	}
}

func TestBrowserRuntimeCORS(t *testing.T) {
	service := &browserDouble{}
	limiter, _ := httpapi.NewLocalRegistrationLimiter(
		httpapi.RegistrationLimitConfig{
			Key:           make([]byte, 32),
			IPAttempts:    60,
			EmailAttempts: 5,
			IPWindow:      time.Minute,
			EmailWindow:   time.Minute,
			MaxKeys:       10,
			MaxConcurrent: 4,
		},
	)
	h, _ := httpapi.NewWebSessionHandler(service, limiter, "https://govai.test")
	router := httpapi.NewRouter(httpapi.RouterOptions{
		CORSOrigins: []string{"https://other-general-origin.test"},
		WebSessions: h,
	})
	for _, tc := range []struct {
		origin, header string
		status         int
	}{
		{"https://govai.test", "content-type,x-govai-web,x-csrf-token", 204},
		{"https://other-general-origin.test", "x-govai-web", 403},
		{"https://evil.test", "x-govai-web", 403},
		{"https://govai.test", "authorization", 403},
	} {
		request := httptest.NewRequest("OPTIONS", "https://api.govai.test/api/v1/web/refresh", nil)
		request.TLS = &tls.ConnectionState{}
		request.Header.Set("Origin", tc.origin)
		request.Header.Set("Access-Control-Request-Method", "POST")
		request.Header.Set("Access-Control-Request-Headers", tc.header)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		if w.Code != tc.status {
			t.Fatal("CORS boundary failed")
		}

		if tc.status == 204 &&
			(w.Header().Get("Access-Control-Allow-Credentials") != "true" ||
				w.Header().Get("Access-Control-Allow-Origin") != "https://govai.test") {
			t.Fatal("credential CORS missing")
		}

		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable preflight")
		}
	}

	disabled := httpapi.NewRouter(httpapi.RouterOptions{})
	r := browserRequest("/login")
	r.URL.Path = "/api/v1/web/login"
	w := httptest.NewRecorder()
	disabled.ServeHTTP(w, r)
	if w.Code != 403 && w.Code != 404 {
		t.Fatal("web enabled implicitly")
	}
}

func TestWebLoginAcceptsNICIdentifier(t *testing.T) {
	service := &browserDouble{}
	router := browserRouter(t, service)
	request := browserRequest("/login")
	request.Body = io.NopCloser(strings.NewReader(`{"identifier":"123456789v","password":"synthetic-password"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 200 || service.calls != 1 {
		t.Fatalf("NIC web login status=%d calls=%d", response.Code, service.calls)
	}

	if strings.Contains(response.Body.String(), "123456789") {
		t.Fatal("NIC disclosed")
	}
}
