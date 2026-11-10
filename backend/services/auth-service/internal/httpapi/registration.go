package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/gin-gonic/gin"
)

// RegistrationService is satisfied by *registration.Service. No HTTP handler
// can supply stores, identity/role overrides or administrative configuration.
type RegistrationService interface {
	Register(context.Context, string, io.Reader) (registration.Receipt, error)
}

type RegistrationHTTPConfig struct {
	Timeout               time.Duration
	UnavailableRetryAfter time.Duration
	MinResponseTime       time.Duration // optional bounded response floor, not enumeration resistance
}

type RegistrationHandler struct {
	service RegistrationService
	limiter RegistrationLimiter
	cfg     RegistrationHTTPConfig
}

// NewRegistrationHandler checks dependencies and request budgets.
func NewRegistrationHandler(service RegistrationService, limiter RegistrationLimiter, cfg RegistrationHTTPConfig) (*RegistrationHandler, error) {
	if service == nil ||
		limiter == nil ||
		cfg.Timeout <= 0 ||
		cfg.Timeout > 30*time.Second ||
		cfg.UnavailableRetryAfter <= 0 ||
		cfg.UnavailableRetryAfter > time.Hour ||
		cfg.MinResponseTime < 0 ||
		cfg.MinResponseTime > time.Second {
		return nil, errors.New("invalid registration handler configuration")
	}

	return &RegistrationHandler{service: service, limiter: limiter, cfg: cfg}, nil
}

var registrationKey = regexp.MustCompile(`^[A-Fa-f0-9]{8}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{12}$`)

var responseRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type registrationErrorBody struct {
	Error  string            `json:"error"`
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Handle is attached only by explicit registration runtime composition or tests.
// Request bytes/passwords remain transient, never logs.
func (h *RegistrationHandler) Handle(c *gin.Context) {
	started := time.Now()
	c.Header("Cache-Control", "no-store")
	defer func() {
		if recover() != nil {
			if !c.Writer.Written() {
				h.unavailable(c, started)
			} else {
				c.Abort()
			}
		}
	}()
	// Preserve a trusted middleware response ID, never reflect a caller header.
	if !responseRequestID.MatchString(c.Writer.Header().Get("X-Request-ID")) {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			h.unavailable(c, started)
			return
		}

		c.Header("X-Request-ID", hex.EncodeToString(id[:]))
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.cfg.Timeout)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	if ctx.Err() != nil {
		h.unavailable(c, started)
		return
	}

	if c.Request.Method != http.MethodPost {
		c.Header("Allow", "POST")
		h.respond(
			c,
			405,
			registrationErrorBody{Error: "method not allowed", Code: "method_not_allowed"},
			started,
		)
		return
	}

	ip, ok := registrationSource(c.Request.RemoteAddr)
	if !ok {
		h.unavailable(c, started)
		return
	}

	permit, retry, err := h.limiter.Acquire(ctx, ip)
	if err != nil {
		if permit != nil {
			permit.Release()
		}

		h.limitError(c, err, retry, started)
		return
	}

	if permit == nil {
		h.unavailable(c, started)
		return
	}

	defer permit.Release()
	contentTypes := c.Request.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		h.unsupported(c, started)
		return
	}

	media, params, err := mime.ParseMediaType(contentTypes[0])
	if err != nil ||
		media != "application/json" ||
		(params["charset"] != "" &&
			!strings.EqualFold(params["charset"], "utf-8")) {
		h.unsupported(c, started)
		return
	}

	if encoding := c.Request.Header.Get("Content-Encoding"); len(c.Request.Header.Values("Content-Encoding")) > 1 ||

		(encoding != "" &&
			!strings.EqualFold(encoding, "identity")) {
		h.unsupported(c, started)
		return
	}

	if len(c.Request.Header.Values("Idempotency-Key")) != 1 ||
		!registrationKey.MatchString(c.GetHeader("Idempotency-Key")) {
		h.invalid(c, started)
		return
	}

	if c.Request.ContentLength > registration.MaxRequestBytes {
		h.tooLarge(c, started)
		return
	}

	if c.Request.Body == nil {
		h.invalid(c, started)
		return
	}

	defer c.Request.Body.Close()
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, registration.MaxRequestBytes+1))
	if ctx.Err() != nil {
		h.unavailable(c, started)
		return
	}

	if len(body) > registration.MaxRequestBytes {
		h.tooLarge(c, started)
		return
	}

	if err != nil {
		h.invalid(c, started)
		return
	}
	// Parse once for canonical email admission. Pass the original bounded bytes
	// to the existing service, whose own validation remains authoritative too.
	r, inputErr := registration.ParseRequest(bytes.NewReader(body))
	if inputErr != nil {
		h.mapError(c, inputErr, started)
		return
	}

	if retry, err = permit.CheckEmail(ctx, r.Email); err != nil {
		h.limitError(c, err, retry, started)
		return
	}

	if ctx.Err() != nil {
		h.unavailable(c, started)
		return
	}

	receipt, err := h.service.Register(ctx, strings.ToLower(c.GetHeader("Idempotency-Key")), bytes.NewReader(body))
	if ctx.Err() != nil {
		h.unavailable(c, started)
		return
	}

	if err != nil {
		h.mapError(c, err, started)
		return
	}

	if receipt.Status != "received" {
		h.unavailable(c, started)
		return
	}

	h.received(c, started)
}

func registrationSource(remote string) (string, bool) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}

	return ip.WithZone("").Unmap().String(), true
}

func (h *RegistrationHandler) respond(c *gin.Context, status int, body any, started time.Time) {
	if remaining := h.cfg.MinResponseTime - time.Since(started); remaining > 0 {
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-c.Request.Context().Done():
		}
	}

	abortJSON(c, status, body)
}

func (h *RegistrationHandler) received(c *gin.Context, t time.Time) {
	h.respond(c, 202, struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{"received", "If registration can proceed, check your email for the next step."}, t)
}

func (h *RegistrationHandler) invalid(c *gin.Context, t time.Time) {
	h.respond(c, 400, registrationErrorBody{Error: "invalid request", Code: "invalid_request"}, t)
}

func (h *RegistrationHandler) tooLarge(c *gin.Context, t time.Time) {
	h.respond(c, 413, registrationErrorBody{Error: "request body too large", Code: "too_large"}, t)
}

func (h *RegistrationHandler) unsupported(c *gin.Context, t time.Time) {
	h.respond(
		c,
		415,
		registrationErrorBody{Error: "unsupported content type", Code: "unsupported_media_type"},
		t,
	)
}

func retrySeconds(d time.Duration) string {
	if d <= 0 {
		d = time.Second
	}

	if d > 24*time.Hour {
		d = 24 * time.Hour
	}

	return strconv.FormatInt(int64((d+time.Second-1)/time.Second), 10)
}

func (h *RegistrationHandler) unavailable(c *gin.Context, t time.Time) {
	c.Header("Retry-After", retrySeconds(h.cfg.UnavailableRetryAfter))
	h.respond(
		c,
		503,
		registrationErrorBody{Error: "registration unavailable", Code: "registration_unavailable"},
		t,
	)
}

func (h *RegistrationHandler) limitError(c *gin.Context, err error, retry time.Duration, t time.Time) {
	if errors.Is(err, ErrRegistrationLimited) {
		c.Header("Retry-After", retrySeconds(retry))
		h.respond(
			c,
			429,
			registrationErrorBody{Error: "too many registration attempts", Code: "rate_limited"},
			t,
		)
		return
	}

	h.unavailable(c, t)
}

func (h *RegistrationHandler) mapError(c *gin.Context, err error, t time.Time) {
	var input *registration.RequestError
	if errors.As(err, &input) {
		switch input.Code {
		case "invalid_request":
			h.invalid(c, t)
			return
		case "too_large":
			h.tooLarge(c, t)
			return
		case "validation_failed":
			h.respond(
				c,
				422,
				registrationErrorBody{
					Error:  "invalid registration details",
					Code:   "validation_failed",
					Fields: safeRegistrationFields(input.Fields),
				},
				t,
			)
			return
		}
	}

	var service *registration.ServiceError
	if errors.As(err, &service) {
		switch service.Code {
		case "registration_invalid_key":
			h.invalid(c, t)
			return
		case "registration_request_conflict":
			h.respond(
				c,
				409,
				registrationErrorBody{Error: "registration request conflict", Code: "request_conflict"},
				t,
			)
			return
		case "registration_policy_rejected":
			h.respond(
				c,
				422,
				registrationErrorBody{Error: "invalid registration details", Code: "validation_failed"},
				t,
			)
			return
		}
	}

	if errors.Is(err, registration.ErrNICConflict) {
		h.received(c, t)
		return
	}

	h.unavailable(c, t)
}

func safeRegistrationFields(fields map[string]string) map[string]string {
	clean := map[string]string{}
	for _, name := range []string{"full_name", "nic", "mobile_number", "email", "password"} {
		switch code := fields[name]; code {
		case "required", "invalid_type", "invalid_format", "too_long":
			clean[name] = code
		case "password_policy":
			if name == "password" {
				clean[name] = code
			}
		}
	}

	return clean
}
