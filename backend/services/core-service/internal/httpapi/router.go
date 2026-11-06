package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/chat"
	"github.com/gin-gonic/gin"
)

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message, "code": code})
}
func NewRouter(verifier *authn.Verifier, agent chat.Agent, origins []string, logger *slog.Logger, budgets ...time.Duration) (*gin.Engine, error) {
	if verifier == nil || agent == nil {
		return nil, errors.New("invalid chat router dependencies")
	}
	if logger == nil {
		logger = slog.Default()
	}
	budget := 30 * time.Second
	if len(budgets) > 1 {
		return nil, errors.New("invalid chat timeout")
	}
	if len(budgets) == 1 {
		budget = budgets[0]
	}
	if budget < time.Second || budget > time.Minute {
		return nil, errors.New("invalid chat timeout")
	}
	origins = append([]string(nil), origins...)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	if router.SetTrustedProxies(nil) != nil {
		return nil, errors.New("invalid proxy configuration")
	}
	router.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Header("X-Content-Type-Options", "nosniff")
		var requestID string
		started := time.Now()
		defer func() {
			if recover() != nil {
				logger.Error("request panic", "request_id", requestID)
				if !c.Writer.Written() {
					fail(c, 500, "internal_error", "Unable to process this request.")
				} else {
					c.Abort()
				}
			}
			logger.Info("http request", "request_id", requestID, "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
		}()
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			fail(c, 503, "request_unavailable", "Service temporarily unavailable.")
			return
		}
		requestID = hex.EncodeToString(id[:])
		c.Header("X-Request-ID", requestID)
		ctx, cancel := context.WithTimeout(c.Request.Context(), budget)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(func(c *gin.Context) {
		c.Header("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		allowed := false
		for _, candidate := range origins {
			if candidate == origin {
				allowed = true
			}
		}
		if !allowed || len(c.Request.Header.Values("Origin")) != 1 {
			fail(c, 403, "origin_not_allowed", "Origin is not allowed.")
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
		if c.Request.Method == http.MethodOptions && c.Request.URL.Path == "/api/chat/message" {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
			if c.GetHeader("Access-Control-Request-Method") != "POST" {
				fail(c, 403, "cors_method_not_allowed", "Method is not allowed.")
				return
			}
			for _, field := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(field)) {
				case "", "authorization", "content-type":
				default:
					fail(c, 403, "cors_header_not_allowed", "Header is not allowed.")
					return
				}
			}
			c.Header("Access-Control-Allow-Methods", "POST")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	health := func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "govai-core"}) }
	router.GET("/health", health)
	router.GET("/api/health", health)
	slots := make(chan struct{}, 8)
	router.POST("/api/chat/message", verifier.Authenticate(), authn.RequireVerifiedEmail(), authn.RequireRole("citizen"), func(c *gin.Context) {
		if _, ok := authn.Current(c); !ok {
			fail(c, 401, "unauthorized", "Authentication required.")
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			c.Header("Retry-After", "1")
			fail(c, 503, "chat_busy", "Chat is busy. Try again shortly.")
			return
		}
		media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || media != "application/json" || len(c.Request.Header.Values("Content-Type")) != 1 || c.GetHeader("Content-Encoding") != "" {
			fail(c, 415, "unsupported_media_type", "Use application/json.")
			return
		}
		for key, value := range params {
			if key != "charset" || !strings.EqualFold(value, "utf-8") {
				fail(c, 415, "unsupported_media_type", "Use UTF-8 JSON.")
				return
			}
		}
		controller := http.NewResponseController(c.Writer)
		deadline := time.Now().Add(10 * time.Second)
		if parent, ok := c.Request.Context().Deadline(); ok && parent.Before(deadline) {
			deadline = parent
		}
		if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			fail(c, 503, "chat_unavailable", "Chat temporarily unavailable.")
			return
		}
		defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
		question, err := chat.Parse(c.Request.Body)
		if err != nil {
			if errors.Is(err, chat.ErrTooLarge) {
				fail(c, 413, "request_too_large", "Question exceeds the request limit.")
			} else {
				fail(c, 400, "invalid_message", "Provide a nonempty message of at most 4000 characters.")
			}
			return
		}
		ctx := c.Request.Context()
		if ctx.Err() != nil {
			fail(c, 504, "chat_timeout", "Chat request timed out.")
			return
		}
		reply, err := agent.Ask(ctx, question)
		if err != nil || ctx.Err() != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fail(c, 504, "chat_timeout", "Chat request timed out.")
			} else if errors.Is(err, chat.ErrResponse) {
				fail(c, 502, "invalid_agent_response", "Unable to retrieve a valid answer.")
			} else {
				fail(c, 503, "chat_unavailable", "Chat temporarily unavailable. Try again later.")
			}
			return
		}
		encoded, err := chat.Encode(reply)
		if err != nil {
			fail(c, 502, "invalid_agent_response", "Unable to retrieve a valid answer.")
			return
		}
		c.Data(200, "application/json; charset=utf-8", encoded)
	})
	router.NoRoute(func(c *gin.Context) { fail(c, 404, "route_not_found", "Route not found.") })
	router.NoMethod(func(c *gin.Context) { fail(c, 405, "method_not_allowed", "Method not allowed.") })
	return router, nil
}
