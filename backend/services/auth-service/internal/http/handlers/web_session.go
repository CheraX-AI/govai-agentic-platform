package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/websession"
	"github.com/gin-gonic/gin"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const WebSessionCookie = "__Host-govai_session"

type WebSessionService interface {
	Start(context.Context, login.Request) (websession.Started, error)
	Check(context.Context, string) (websession.Identity, string, error)
	CSRF(context.Context, string) (string, error)
	Refresh(context.Context, string, string) error
	End(context.Context, string, string) error
}
type WebSessionHandler struct {
	service      WebSessionService
	limiter      RegistrationLimiter
	loginLimiter RegistrationLimiter
	origin       string
}

func NewWebSessionHandler(s WebSessionService, l RegistrationLimiter, origin string) (*WebSessionHandler, error) {
	u, err := url.Parse(origin)
	if s == nil || l == nil || err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != "" || u.String() != origin {
		return nil, websession.ErrUnavailable
	}
	return &WebSessionHandler{service: s, limiter: l, loginLimiter: l, origin: origin}, nil
}

// SetLoginLimiter is configuration-only and must be called before serving.
func (h *WebSessionHandler) SetLoginLimiter(l RegistrationLimiter) {
	if l != nil {
		h.loginLimiter = l
	}
}

type webPermitKey struct{}

func webFail(c *gin.Context, status int, code string) {
	errorResponse(c, status, code, "Unable to complete the web session request.")
}
func webError(c *gin.Context, err error) {
	if errors.Is(err, websession.ErrDenied) {
		webFail(c, 401, "session_rejected")
	} else {
		webFail(c, 503, "session_unavailable")
	}
}
func validWebSecret(value string) bool {
	b, e := base64.RawURLEncoding.DecodeString(value)
	return e == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == value
}
func webCookie(c *gin.Context) (string, bool) {
	value := ""
	count := 0
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == WebSessionCookie {
			value = cookie.Value
			count++
		}
	}
	return value, count == 1 && validWebSecret(value)
}
func clearWebCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: WebSessionCookie, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

// begin supplies security policy and a bounded source/concurrency permit to all
// browser handlers. A missing Origin is allowed only for reads with a custom
// header; cross-origin scripts must pass the separate exact CORS allowlist.
func (h *WebSessionHandler) begin(c *gin.Context, write bool, isLogin ...bool) (context.Context, func(), bool) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.TLS == nil {
		webFail(c, 426, "https_required")
		return nil, nil, false
	}
	if c.GetHeader("Authorization") != "" || c.GetHeader("X-GovAI-Web") != "1" || c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		webFail(c, 403, "web_request_rejected")
		return nil, nil, false
	}
	origins := c.Request.Header.Values("Origin")
	if len(origins) > 1 || (write && (len(origins) != 1 || origins[0] != h.origin)) || (!write && len(origins) == 1 && origins[0] != h.origin) {
		webFail(c, 403, "origin_rejected")
		return nil, nil, false
	}
	if (write && c.Request.Method != "POST") || (!write && c.Request.Method != "GET") {
		webFail(c, 405, "method_not_allowed")
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	source, ok := registrationSource(c.Request.RemoteAddr)
	if !ok {
		cancel()
		webFail(c, 503, "session_unavailable")
		return nil, nil, false
	}
	limiter := h.limiter
	if len(isLogin) > 0 && isLogin[0] {
		limiter = h.loginLimiter
	}
	permit, retry, err := limiter.Acquire(ctx, source)
	if err != nil || permit == nil {
		cancel()
		if errors.Is(err, ErrRegistrationLimited) {
			c.Header("Retry-After", retrySeconds(retry))
			webFail(c, 429, "rate_limited")
		} else {
			webFail(c, 503, "session_unavailable")
		}
		return nil, nil, false
	}
	return context.WithValue(ctx, webPermitKey{}, permit), func() { permit.Release(); cancel() }, true
}
func (h *WebSessionHandler) Login(c *gin.Context) {
	ctx, done, ok := h.begin(c, true, true)
	if !ok {
		return
	}
	defer done()
	// Existing sessions must be explicitly logged out before switching accounts.
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == WebSessionCookie {
			webFail(c, 409, "logout_required")
			return
		}
	}
	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" || len(params) > 1 || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) || c.GetHeader("Content-Encoding") != "" {
		webFail(c, 415, "unsupported_media_type")
		return
	}
	for k := range params {
		if k != "charset" {
			webFail(c, 415, "unsupported_media_type")
			return
		}
	}
	controller := http.NewResponseController(c.Writer)
	deadline, _ := ctx.Deadline()
	if err = controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		webFail(c, 503, "session_unavailable")
		return
	}
	defer controller.SetReadDeadline(time.Time{})
	if c.Request.Body == nil {
		webFail(c, 400, "invalid_request")
		return
	}
	defer c.Request.Body.Close()
	request, err := login.Parse(c.Request.Body)
	if err != nil {
		if errors.Is(err, login.ErrTooLarge) {
			webFail(c, 413, "too_large")
		} else {
			webFail(c, 400, "invalid_request")
		}
		return
	}
	if ctx.Err() != nil {
		webFail(c, 503, "session_unavailable")
		return
	}
	permit, _ := ctx.Value(webPermitKey{}).(RegistrationPermit)
	if permit == nil {
		webFail(c, 503, "session_unavailable")
		return
	}
	retry, err := permit.CheckEmail(ctx, request.Email())
	if err != nil {
		if errors.Is(err, ErrRegistrationLimited) {
			c.Header("Retry-After", retrySeconds(retry))
			webFail(c, 429, "rate_limited")
		} else {
			webFail(c, 503, "session_unavailable")
		}
		return
	}
	result, err := h.service.Start(ctx, request)
	if err != nil {
		webError(c, err)
		return
	}
	if ctx.Err() != nil || !validWebSecret(result.Cookie) || !validWebSecret(result.CSRF) || !result.Expires.After(time.Now()) {
		webFail(c, 503, "session_unavailable")
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: WebSessionCookie, Value: result.Cookie, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: result.Expires})
	c.JSON(200, gin.H{"csrf_token": result.CSRF, "expires_at": result.Expires})
}
func (h *WebSessionHandler) Check(c *gin.Context) {
	ctx, done, ok := h.begin(c, false)
	if !ok {
		return
	}
	defer done()
	raw, ok := webCookie(c)
	if !ok {
		webFail(c, 401, "session_rejected")
		return
	}
	identity, _, err := h.service.Check(ctx, raw)
	if err != nil {
		webError(c, err)
		return
	}
	if ctx.Err() != nil {
		webFail(c, 503, "session_unavailable")
		return
	}
	c.JSON(200, gin.H{"subject": identity.Subject})
}
func (h *WebSessionHandler) CSRF(c *gin.Context) {
	ctx, done, ok := h.begin(c, false)
	if !ok {
		return
	}
	defer done()
	raw, ok := webCookie(c)
	if !ok {
		webFail(c, 401, "session_rejected")
		return
	}
	token, err := h.service.CSRF(ctx, raw)
	if err != nil {
		webError(c, err)
		return
	}
	if ctx.Err() != nil || !validWebSecret(token) {
		webFail(c, 503, "session_unavailable")
		return
	}
	c.JSON(200, gin.H{"csrf_token": token})
}
func (h *WebSessionHandler) Refresh(c *gin.Context) { h.write(c, false) }
func (h *WebSessionHandler) Logout(c *gin.Context)  { h.write(c, true) }
func (h *WebSessionHandler) write(c *gin.Context, logout bool) {
	ctx, done, ok := h.begin(c, true)
	if !ok {
		return
	}
	defer done()
	raw, ok := webCookie(c)
	if !ok {
		webFail(c, 401, "session_rejected")
		return
	}
	tokens := c.Request.Header.Values("X-CSRF-Token")
	if len(tokens) != 1 || !validWebSecret(tokens[0]) {
		webFail(c, 403, "csrf_required")
		return
	}
	// Session mutation routes take no body or token values from JavaScript.
	if c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0 {
		webFail(c, 400, "body_not_allowed")
		return
	}
	var err error
	if logout {
		err = h.service.End(ctx, raw, tokens[0])
	} else {
		err = h.service.Refresh(ctx, raw, tokens[0])
	}
	if errors.Is(err, websession.ErrRevocationPending) {
		clearWebCookie(c)
		webFail(c, 503, "provider_revocation_pending")
		return
	}
	if ctx.Err() != nil {
		err = websession.ErrUnavailable
	}
	if err != nil {
		webError(c, err)
		return
	}
	if logout {
		clearWebCookie(c)
	}
	c.Status(204)
}
