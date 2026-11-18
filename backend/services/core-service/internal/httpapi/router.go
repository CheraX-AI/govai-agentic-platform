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
	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/chat"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation"
	"github.com/gin-gonic/gin"
)

// Conversations resolves the conversation ID for a citizen's message
// (conversation.Service). owner is always the verified token subject.
type Conversations interface {
	Resolve(ctx context.Context, owner, requested string) (string, error)
}

// Messages keeps the encrypted record of each exchange (conversation.Messages).
type Messages interface {
	Start(ctx context.Context, owner, conversationID, text string) (conversation.Turn, error)
	Complete(ctx context.Context, turn conversation.Turn, answer string) error
	Fail(ctx context.Context, turn conversation.Turn) error
}

// recordTimeout bounds saving a turn's outcome after the agent call.
const recordTimeout = 5 * time.Second

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message, "code": code})
}

// Settings are the router's deploy-time limits. Zero values use the defaults.
type Settings struct {
	Origins       []string      // exact browser origins allowed by CORS; empty = none
	Timeout       time.Duration // whole chat request budget (default 30s, 1s-60s)
	MaxConcurrent int           // chats answered at the same time, all users (default 8, 1-64)
}

// NewRouter protects chat requests and applies shared request limits.
func NewRouter(
	verifier *authn.Verifier,
	agent chat.Agent,
	conversations Conversations,
	messages Messages,
	logger *slog.Logger,
	settings Settings,
) (*gin.Engine, error) {
	if verifier == nil || agent == nil || conversations == nil || messages == nil {
		return nil, errors.New("invalid chat router dependencies")
	}

	if logger == nil {
		logger = slog.Default()
	}

	budget := settings.Timeout
	if budget == 0 {
		budget = 30 * time.Second
	}

	if budget < time.Second || budget > time.Minute {
		return nil, errors.New("invalid chat timeout")
	}

	maxConcurrent := settings.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = 8
	}

	if maxConcurrent < 1 || maxConcurrent > 64 {
		return nil, errors.New("invalid chat concurrency")
	}

	origins := append([]string(nil), settings.Origins...)
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

			logger.Info(
				"http request",
				"request_id",
				requestID,
				"method",
				c.Request.Method,
				"route",
				c.FullPath(),
				"status",
				c.Writer.Status(),
				"duration_ms",
				time.Since(started).Milliseconds(),
			)
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
		c.Request = c.Request.WithContext(chat.WithRequestID(ctx, requestID))
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
	slots := make(chan struct{}, maxConcurrent)
	router.POST("/api/chat/message",
		verifier.Authenticate(),
		authn.RequireVerifiedEmail(),
		authn.RequireRole("citizen"),
		func(c *gin.Context) {
			identity, ok := authn.Current(c)
			if !ok {
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
			if err != nil ||
				media != "application/json" ||
				len(c.Request.Header.Values("Content-Type")) != 1 ||
				c.GetHeader("Content-Encoding") != "" {
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
			question, err := chatcontract.Parse(c.Request.Body)
			if err != nil {
				switch {
				case errors.Is(err, chatcontract.ErrTooLarge):
					fail(c, 413, "request_too_large", "Question exceeds the request limit.")
				case errors.Is(err, chatcontract.ErrInvalidConversationID):
					fail(c, 400, "invalid_conversation_id", "conversation_id must be a lowercase UUID.")
				default:
					fail(c, 400, "invalid_message", "Provide a nonempty message of at most 4000 characters.")
				}

				return
			}

			ctx := c.Request.Context()
			if ctx.Err() != nil {
				fail(c, 504, "chat_timeout", "Chat request timed out.")
				return
			}

			// The owner is the verified token subject, never a client-supplied value.
			// Unknown and other citizens' IDs both get 404, so IDs cannot be probed.
			conversationID, err := conversations.Resolve(ctx, identity.Subject, question.ConversationID())
			if err != nil {
				switch {
				case errors.Is(err, conversation.ErrNotFound):
					fail(c, 404, "conversation_not_found", "Conversation not found.")
				case errors.Is(ctx.Err(), context.DeadlineExceeded):
					fail(c, 504, "chat_timeout", "Chat request timed out.")
				default:
					logger.Warn("conversation storage unavailable", "request_id", c.Writer.Header().Get("X-Request-ID"))
					fail(c, 503, "chat_unavailable", "Chat temporarily unavailable. Try again later.")
				}

				return
			}

			// The question is saved (encrypted, pending) before the agent is asked.
			requestID := c.Writer.Header().Get("X-Request-ID")
			turn, err := messages.Start(ctx, identity.Subject, conversationID, question.Text())
			if err != nil {
				switch {
				case errors.Is(err, conversation.ErrNotFound):
					fail(c, 404, "conversation_not_found", "Conversation not found.")
				case errors.Is(err, conversation.ErrInProgress):
					c.Header("Retry-After", "2")
					fail(c, 503, "chat_busy", "This message is still being answered. Try again shortly.")
				case errors.Is(ctx.Err(), context.DeadlineExceeded):
					fail(c, 504, "chat_timeout", "Chat request timed out.")
				default:
					logger.Warn("message storage unavailable", "request_id", requestID)
					fail(c, 503, "chat_unavailable", "Chat temporarily unavailable. Try again later.")
				}

				return
			}

			// A retry of an answered question gets the stored answer; the agent is not asked again.
			if answer, ok := turn.Answered(); ok {
				encoded, err := chatcontract.Encode(chatcontract.Reply{Answer: answer, ConversationID: conversationID})
				if err != nil {
					fail(c, 503, "chat_unavailable", "Chat temporarily unavailable. Try again later.")
					return
				}

				c.Data(200, "application/json; charset=utf-8", encoded)
				return
			}

			// Outcomes are saved even if the client has gone, within their own short budget.
			failTurn := func() {
				record, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
				defer cancel()
				if messages.Fail(record, turn) != nil {
					logger.Warn("message status not saved", "request_id", requestID)
				}
			}

			ctx = chat.WithConversationID(ctx, conversationID)
			reply, err := agent.Ask(ctx, question)
			if err != nil || ctx.Err() != nil {
				failTurn()
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
					fail(c, 504, "chat_timeout", "Chat request timed out.")
				} else if errors.Is(err, chat.ErrResponse) {
					fail(c, 502, "invalid_agent_response", "Unable to retrieve a valid answer.")
				} else {
					fail(c, 503, "chat_unavailable", "Chat temporarily unavailable. Try again later.")
				}

				return
			}

			// The agent does not choose the conversation; core-service does. An agent
			// that echoes a different ID is broken, so its answer is not trusted.
			if reply.ConversationID != "" && reply.ConversationID != conversationID {
				failTurn()
				fail(c, 502, "invalid_agent_response", "Unable to retrieve a valid answer.")
				return
			}

			reply.ConversationID = conversationID
			encoded, err := chatcontract.Encode(reply)
			if err != nil {
				failTurn()
				fail(c, 502, "invalid_agent_response", "Unable to retrieve a valid answer.")
				return
			}

			// The answer is returned only once it is saved: no success without a record.
			record, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
			err = messages.Complete(record, turn, reply.Answer)
			cancel()
			if err != nil {
				logger.Warn("answer not saved", "request_id", requestID)
				fail(c, 503, "chat_unavailable", "Chat temporarily unavailable. Try again later.")
				return
			}

			c.Data(200, "application/json; charset=utf-8", encoded)
		})
	router.NoRoute(func(c *gin.Context) { fail(c, 404, "route_not_found", "Route not found.") })
	router.NoMethod(func(c *gin.Context) { fail(c, 405, "method_not_allowed", "Method not allowed.") })
	return router, nil
}
