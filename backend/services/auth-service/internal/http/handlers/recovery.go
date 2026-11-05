package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/gin-gonic/gin"
)

type RecoveryService interface {
	Request(context.Context, string) (recovery.Receipt, error)
	Reset(context.Context, string, string) error
}

type RecoveryHTTPConfig struct {
	Origins []string
	Timeout time.Duration
}

type RecoveryHandler struct {
	service                  RecoveryService
	requestLimit, resetLimit RegistrationLimiter
	cfg                      RecoveryHTTPConfig
}

func NewRecoveryHandler(
	service RecoveryService,
	requestLimit, resetLimit RegistrationLimiter,
	cfg RecoveryHTTPConfig,
) (*RecoveryHandler, error) {
	if service == nil ||
		requestLimit == nil ||
		resetLimit == nil ||
		cfg.Timeout <= 0 ||
		cfg.Timeout > 30*time.Second {
		return nil, recovery.ErrUnavailable
	}

	cfg.Origins = append([]string(nil), cfg.Origins...)
	for _, origin := range cfg.Origins {
		u, err := url.Parse(origin)
		if err != nil ||
			u.Scheme != "https" ||
			u.Hostname() == "" ||
			u.User != nil ||
			u.Path != "" ||
			u.RawQuery != "" ||
			u.ForceQuery ||
			u.Fragment != "" ||
			u.String() != origin {
			return nil, recovery.ErrUnavailable
		}
	}

	return &RecoveryHandler{service: service, requestLimit: requestLimit, resetLimit: resetLimit, cfg: cfg}, nil
}

func recoveryFail(c *gin.Context, status int, code string) {
	errorResponse(c, status, code, "")
}

func recoveryUnavailable(c *gin.Context) {
	c.Header("Retry-After", "5")
	recoveryFail(c, 503, "recovery_unavailable")
}

func recoveryLimitError(c *gin.Context, retry time.Duration, err error) {
	if errors.Is(err, ErrRegistrationLimited) {
		if retry < time.Second {
			retry = time.Second
		}

		c.Header("Retry-After", retrySeconds(retry))
		recoveryFail(c, 429, "rate_limited")
		return
	}

	recoveryUnavailable(c)
}

// Exact, bounded JSON object parsing: no unknown/duplicate fields, null values,
// wrong types, trailing JSON or replacement of invalid Unicode in credentials.
func recoveryObject(reader io.Reader, fields ...string) (map[string]string, int) {
	if reader == nil {
		return nil, 400
	}

	body, err := io.ReadAll(io.LimitReader(reader, 4097))
	if len(body) > 4096 {
		return nil, 413
	}

	if err != nil || !utf8.Valid(body) || !json.Valid(body) || !recoverySurrogates(body) {
		return nil, 400
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, 400
	}

	result := map[string]string{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, 400
		}

		name, ok := key.(string)
		if !ok {
			return nil, 400
		}

		allowed := false
		for _, field := range fields {
			if name == field {
				allowed = true
			}
		}

		if _, seen := result[name]; !allowed || seen {
			return nil, 400
		}

		var raw json.RawMessage
		if decoder.Decode(&raw) != nil || len(raw) == 0 || raw[0] != '"' {
			return nil, 400
		}

		var value string
		if json.Unmarshal(raw, &value) != nil {
			return nil, 400
		}

		result[name] = value
	}

	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, 400
	}

	var extra any
	if decoder.Decode(&extra) != io.EOF || len(result) != len(fields) {
		return nil, 400
	}

	return result, 0
}

// Handlers are injectable only. The main router does not attach these methods.
func (h *RecoveryHandler) Request(c *gin.Context) { h.handle(c, false) }

// Reset validates the reset token and new password.
func (h *RecoveryHandler) Reset(c *gin.Context) { h.handle(c, true) }

func (h *RecoveryHandler) handle(c *gin.Context, reset bool) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
	defer func() {
		if recover() != nil {
			if !c.Writer.Written() {
				recoveryUnavailable(c)
			} else {
				c.Abort()
			}
		}
	}()
	if c.Request.TLS == nil {
		recoveryFail(c, 426, "https_required")
		return
	}

	if c.Request.Method != http.MethodPost {
		c.Header("Allow", "POST")
		recoveryFail(c, 405, "method_not_allowed")
		return
	}

	if len(c.Request.Header.Values("Origin")) > 1 {
		recoveryFail(c, 403, "origin_not_allowed")
		return
	}

	if origin := c.GetHeader("Origin"); origin != "" {
		allowed := false
		for _, candidate := range h.cfg.Origins {
			if origin == candidate {
				allowed = true
			}
		}

		if !allowed {
			recoveryFail(c, 403, "origin_not_allowed")
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.cfg.Timeout)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	if ctx.Err() != nil {
		recoveryUnavailable(c)
		return
	}

	controller := http.NewResponseController(c.Writer)
	deadline, _ := ctx.Deadline()
	if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		recoveryUnavailable(c)
		return
	}

	defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
	ip, ok := registrationSource(c.Request.RemoteAddr)
	if !ok {
		recoveryUnavailable(c)
		return
	}

	limiter := h.requestLimit
	if reset {
		limiter = h.resetLimit
	}

	permit, retry, err := limiter.Acquire(ctx, ip)
	if err != nil {
		recoveryLimitError(c, retry, err)
		return
	}

	defer permit.Release()
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		recoveryFail(c, 415, "json_required")
		return
	}

	fields := []string{"nic"}
	if reset {
		fields = []string{"token", "new_password", "confirm_password"}
	}

	body, status := recoveryObject(c.Request.Body, fields...)
	if status != 0 {
		recoveryFail(c, status, "invalid_request")
		return
	}

	if !reset {
		nic, err := registration.ParseNIC(body["nic"])
		if err != nil {
			recoveryFail(c, 400, "invalid_request")
			return
		}
		// Reuse the bounded keyed subject counter; this limiter is recovery-only.
		if retry, err := permit.CheckEmail(ctx, "ga91:nic:"+nic.NormalizedNumber()); err != nil {
			recoveryLimitError(c, retry, err)
			return
		}

		receipt, err := h.service.Request(ctx, nic.NormalizedNumber())
		if ctx.Err() != nil {
			recoveryUnavailable(c)
			return
		}

		if err != nil {
			if errors.Is(err, recovery.ErrInvalid) {
				recoveryFail(c, 400, "invalid_request")
			} else {
				recoveryUnavailable(c)
			}

			return
		}

		c.JSON(202, receipt)
		return
	}

	token, err := recovery.ParseToken(body["token"])
	if err != nil || body["new_password"] != body["confirm_password"] {
		recoveryFail(c, 400, "invalid_request")
		return
	}

	if _, err = recovery.ParsePassword(body["new_password"]); err != nil {
		recoveryFail(c, 400, "invalid_request")
		return
	}

	digest, _ := token.Digest()
	if retry, err := permit.CheckEmail(ctx, "ga91:token:"+hex.EncodeToString(digest[:])); err != nil {
		recoveryLimitError(c, retry, err)
		return
	}

	err = h.service.Reset(ctx, token.Reveal(), body["new_password"])
	if ctx.Err() != nil {
		recoveryUnavailable(c)
		return
	}

	if err != nil {
		if errors.Is(err, recovery.ErrInvalid) {
			recoveryFail(c, 400, "invalid_or_expired_reset")
		} else {
			recoveryUnavailable(c)
		}

		return
	}

	c.Status(204)
}

// json.Valid must run first. Do not let encoding/json replace lone surrogates
// in passwords; literal U+FFFD and valid surrogate pairs remain unchanged.
func recoverySurrogates(body []byte) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}

		i++
		if body[i] != 'u' {
			continue
		}

		n, _ := strconv.ParseUint(string(body[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return false
		}

		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(body) || body[i+1] != '\\' || body[i+2] != 'u' {
				return false
			}

			low, _ := strconv.ParseUint(string(body[i+3:i+7]), 16, 16)
			if low < 0xDC00 || low > 0xDFFF {
				return false
			}

			i += 6
		}
	}

	return true
}

// NewLocalRecoveryHandler supplies bounded, process-local abuse controls.
// Request: 5/IP/15m and one attempt/NIC/minute, including unknown accounts.
// Reset: 10/IP/15m and three valid-body attempts/token/15m. These counters are not
// distributed and reset on process restart. Public multi-instance rollout must
// enforce shared limits; never claim this local adapter supplies that guarantee.
func NewLocalRecoveryHandler(service RecoveryService, key []byte, cfg RecoveryHTTPConfig) (*RecoveryHandler, error) {
	requestLimit, err := NewLocalRegistrationLimiter(
		RegistrationLimitConfig{
			Key:           key,
			IPAttempts:    5,
			EmailAttempts: 1,
			IPWindow:      15 * time.Minute,
			EmailWindow:   time.Minute,
			MaxKeys:       4096,
			MaxConcurrent: 4,
		},
	)
	if err != nil {
		return nil, recovery.ErrUnavailable
	}

	resetLimit, err := NewLocalRegistrationLimiter(
		RegistrationLimitConfig{
			Key:           key,
			IPAttempts:    10,
			EmailAttempts: 3,
			IPWindow:      15 * time.Minute,
			EmailWindow:   15 * time.Minute,
			MaxKeys:       4096,
			MaxConcurrent: 4,
		},
	)
	if err != nil {
		return nil, recovery.ErrUnavailable
	}

	return NewRecoveryHandler(service, requestLimit, resetLimit, cfg)
}

func (h *RecoveryHandler) isPath(path string) bool {
	return path == "/api/v1/auth/forgot-password" || path == "/api/v1/auth/reset-password"
}

func (h *RecoveryHandler) cors(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Vary", "Origin")
	if c.Request.TLS == nil {
		recoveryFail(c, 426, "https_required")
		return
	}

	origin := c.GetHeader("Origin")
	if origin == "" {
		c.Next()
		return
	}

	allowed := false
	for _, candidate := range h.cfg.Origins {
		if origin == candidate {
			allowed = true
		}
	}

	if !allowed || len(c.Request.Header.Values("Origin")) != 1 {
		recoveryFail(c, 403, "origin_not_allowed")
		return
	}

	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Access-Control-Expose-Headers", "Retry-After, X-Request-ID")
	if c.Request.Method == http.MethodOptions {
		c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
		c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
		if c.GetHeader("Access-Control-Request-Method") != "POST" {
			recoveryFail(c, 403, "cors_method_not_allowed")
			return
		}

		for _, field := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
			switch strings.ToLower(strings.TrimSpace(field)) {
			case "", "content-type":
			default:
				recoveryFail(c, 403, "cors_header_not_allowed")
				return
			}
		}

		c.Header("Access-Control-Allow-Methods", "POST")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}
