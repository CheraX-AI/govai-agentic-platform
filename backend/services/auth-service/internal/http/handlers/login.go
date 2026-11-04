package handler

import (
	"bytes"
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

type LoginService interface {
	Login(context.Context, login.Request) (login.Access, error)
}
type LoginHandler struct {
	service LoginService
	limiter RegistrationLimiter
}

func NewLoginHandler(service LoginService, limiter RegistrationLimiter) (*LoginHandler, error) {
	if service == nil || limiter == nil {
		return nil, login.ErrUnavailable
	}
	return &LoginHandler{service, limiter}, nil
}
func (h *LoginHandler) Handle(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	fail := func(status int, code, message string) {
		errorResponse(c, status, code, message)
	}
	unavailable := func() { fail(503, "login_unavailable", "Login is temporarily unavailable.") }
	denied := func() {
		fail(401, "login_failed", "Unable to sign in. Check your details and complete any required account actions.")
	}
	if c.Request.TLS == nil {
		fail(426, "https_required", "HTTPS required.")
		return
	}
	if c.GetHeader("Origin") != "" {
		fail(403, "browser_login_disabled", "Browser login is not enabled on this endpoint.")
		return
	}
	if len(c.Request.Header.Values("Cookie")) != 0 {
		fail(403, "browser_login_disabled", "Browser login is not enabled on this endpoint.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	controller := http.NewResponseController(c.Writer)
	deadline, _ := ctx.Deadline()
	if e := controller.SetReadDeadline(deadline); e != nil && !errors.Is(e, http.ErrNotSupported) {
		unavailable()
		return
	}
	defer controller.SetReadDeadline(time.Time{})
	ip, ok := registrationSource(c.Request.RemoteAddr)
	if !ok {
		unavailable()
		return
	}
	limited := func(retry time.Duration) {
		c.Header("Retry-After", retrySeconds(retry))
		fail(429, "rate_limited", "Too many login attempts.")
	}
	permit, retry, e := h.limiter.Acquire(ctx, ip)
	if e != nil {
		if errors.Is(e, ErrRegistrationLimited) {
			limited(retry)
		} else {
			unavailable()
		}
		return
	}
	if permit == nil {
		unavailable()
		return
	}
	defer permit.Release()
	media, params, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	for key := range params {
		if key != "charset" {
			fail(415, "unsupported_media_type", "Use UTF-8 JSON.")
			return
		}
	}
	if e != nil || media != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) || len(params) > 1 || c.GetHeader("Content-Encoding") != "" {
		fail(415, "unsupported_media_type", "Use UTF-8 JSON.")
		return
	}
	if c.Request.ContentLength > login.MaxRequestBytes {
		fail(413, "too_large", "Request body is too large.")
		return
	}
	if c.Request.Body == nil {
		fail(400, "invalid_request", "Invalid login request.")
		return
	}
	defer c.Request.Body.Close()
	body, e := io.ReadAll(io.LimitReader(c.Request.Body, login.MaxRequestBytes+1))
	if e != nil || ctx.Err() != nil {
		unavailable()
		return
	}
	request, e := login.Parse(bytes.NewReader(body))
	if e != nil {
		if errors.Is(e, login.ErrTooLarge) {
			fail(413, "too_large", "Request body is too large.")
		} else {
			fail(400, "invalid_request", "Invalid login request.")
		}
		return
	}
	retry, e = permit.CheckEmail(ctx, request.LimitKey())
	if e != nil {
		if errors.Is(e, ErrRegistrationLimited) {
			limited(retry)
		} else {
			unavailable()
		}
		return
	}
	access, e := h.service.Login(ctx, request)
	if e != nil {
		if errors.Is(e, login.ErrFailed) {
			denied()
		} else {
			unavailable()
		}
		return
	}
	if ctx.Err() != nil || access.Token == "" || access.ExpiresIn <= 0 {
		unavailable()
		return
	}
	// This is the ONLY deliberate serialization of the sensitive Access value.
	response := gin.H{"access_token": access.Token, "token_type": "Bearer", "expires_in": access.ExpiresIn}
	if access.RefreshToken != "" {
		if access.RefreshExpiresIn <= 0 {
			unavailable()
			return
		}
		response["refresh_token"] = access.RefreshToken
		response["refresh_expires_in"] = access.RefreshExpiresIn
	}
	c.JSON(http.StatusOK, response)
}
