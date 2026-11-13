package handler

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/gin-gonic/gin"
)

type NativeSessionService interface {
	RefreshNative(context.Context, string) (login.SessionTokens, error)
	RevokeNative(context.Context, string) error
}

type NativeSessionHandler struct {
	service NativeSessionService
	limiter RegistrationLimiter
}

// NewNativeSessionHandler configures refresh and revocation limits.
func NewNativeSessionHandler(s NativeSessionService, l RegistrationLimiter) (*NativeSessionHandler, error) {
	if s == nil || l == nil {
		return nil, login.ErrUnavailable
	}

	return &NativeSessionHandler{s, l}, nil
}

// Refresh exchanges a client-held refresh token.
func (h *NativeSessionHandler) Refresh(c *gin.Context) { h.handle(c, false) }

// Revoke invalidates the supplied refresh credential.
func (h *NativeSessionHandler) Revoke(c *gin.Context) { h.handle(c, true) }

// These credential endpoints are native/Postman only; browser session routes
// require a separate cookie/CSRF contract. Origin absence is not authentication.
func (h *NativeSessionHandler) handle(c *gin.Context, revoke bool) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	fail := func(status int, code string) {
		errorResponse(c, status, code, "Unable to complete the session request.")
	}
	if c.Request.Method != http.MethodPost {
		c.Header("Allow", "POST")
		fail(405, "method_not_allowed")
		return
	}

	if c.Request.TLS == nil {
		fail(426, "https_required")
		return
	}

	if c.GetHeader("Origin") != "" || len(c.Request.Header.Values("Cookie")) != 0 {
		fail(403, "browser_session_disabled")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		fail(503, "session_unavailable")
		return
	}

	controller := http.NewResponseController(c.Writer)
	deadline, _ := ctx.Deadline()
	if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		fail(503, "session_unavailable")
		return
	}

	defer controller.SetReadDeadline(time.Time{})
	source, ok := registrationSource(c.Request.RemoteAddr)
	if !ok {
		fail(503, "session_unavailable")
		return
	}

	permit, retry, err := h.limiter.Acquire(ctx, source)
	if err != nil {
		if errors.Is(err, ErrRegistrationLimited) {
			c.Header("Retry-After", retrySeconds(retry))
			fail(429, "rate_limited")
		} else {
			fail(503, "session_unavailable")
		}

		return
	}

	if permit == nil {
		fail(503, "session_unavailable")
		return
	}

	defer permit.Release()
	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil ||
		media != "application/json" ||
		len(params) > 1 ||
		(params["charset"] != "" &&
			!strings.EqualFold(params["charset"], "utf-8")) ||
		c.GetHeader("Content-Encoding") != "" {
		fail(415, "unsupported_media_type")
		return
	}

	for key := range params {
		if key != "charset" {
			fail(415, "unsupported_media_type")
			return
		}
	}

	if c.Request.ContentLength > login.MaxRefreshRequestBytes {
		fail(413, "too_large")
		return
	}

	if c.Request.Body == nil {
		fail(400, "invalid_request")
		return
	}

	defer c.Request.Body.Close()
	request, err := login.ParseRefresh(c.Request.Body)
	if ctx.Err() != nil {
		fail(503, "session_unavailable")
		return
	}

	if err != nil {
		if errors.Is(err, login.ErrTooLarge) {
			fail(413, "too_large")
		} else {
			fail(400, "invalid_request")
		}

		return
	}

	var result login.SessionTokens
	if revoke {
		err = h.service.RevokeNative(ctx, request.Token())
	} else {
		result, err = h.service.RefreshNative(ctx, request.Token())
	}

	if ctx.Err() != nil {
		fail(503, "session_unavailable")
		return
	}

	if err != nil {
		if errors.Is(err, login.ErrFailed) {
			fail(401, "session_rejected")
		} else {
			fail(503, "session_unavailable")
		}

		return
	}

	if revoke {
		c.Status(http.StatusNoContent)
		return
	}

	if result.AccessToken == "" ||
		result.RefreshToken == "" ||
		result.ExpiresIn <= 0 ||
		result.RefreshExpiresIn <= 0 {
		fail(503, "session_unavailable")
		return
	}

	c.JSON(
		200,
		gin.H{
			"access_token":       result.AccessToken,
			"refresh_token":      result.RefreshToken,
			"token_type":         "Bearer",
			"expires_in":         result.ExpiresIn,
			"refresh_expires_in": result.RefreshExpiresIn,
		},
	)
}
