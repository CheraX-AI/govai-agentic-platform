package handler

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/chatgateway"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/websession"
	"github.com/gin-gonic/gin"
)

type ChatForwarder interface {
	Ask(context.Context, string, []byte) (chatgateway.Payload, error)
}

type accessForwarder interface {
	WithAccess(context.Context, string, string, func(context.Context, string) error) error
}

type webChat struct {
	upstream ChatForwarder
	access   accessForwarder
	slots    chan struct{}
}

// EnableChat must run before serving; the upstream is fixed by server configuration.
func (h *WebSessionHandler) EnableChat(upstream ChatForwarder) error {
	a, ok := h.service.(accessForwarder)
	if !ok || upstream == nil {
		return chatgateway.ErrUnavailable
	}

	h.chat = &webChat{upstream, a, make(chan struct{}, 8)}
	return nil
}

// Chat forwards a checked browser session request to core.
func (h *WebSessionHandler) Chat(c *gin.Context) {
	if h.chat == nil {
		c.Status(404)
		return
	}

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

	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil ||
		media != "application/json" ||
		len(c.Request.Header.Values("Content-Type")) != 1 ||
		c.GetHeader("Content-Encoding") != "" {
		errorResponse(c, 415, "unsupported_media_type", "Use UTF-8 JSON.")
		return
	}

	for k, v := range params {
		if k != "charset" || (v != "utf-8" && v != "UTF-8") {
			errorResponse(c, 415, "unsupported_media_type", "Use UTF-8 JSON.")
			return
		}
	}

	controller := http.NewResponseController(c.Writer)
	if err := controller.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil &&
		!errors.Is(err, http.ErrNotSupported) {
		webError(c, websession.ErrUnavailable)
		return
	}

	defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
	q, err := chatcontract.Parse(c.Request.Body)
	if err != nil {
		status := 400
		if errors.Is(err, chatcontract.ErrTooLarge) {
			status = 413
		}

		errorResponse(c, status, "invalid_message", "Provide a nonempty question of at most 4000 characters.")
		return
	}

	select {
	case h.chat.slots <- struct{}{}:
		defer func() { <-h.chat.slots }()
	default:
		c.Header("Retry-After", "1")
		errorResponse(c, 503, "chat_busy", "Chat is busy. Try again later.")
		return
	}

	body, _ := json.Marshal(map[string]string{"message": q.Text()})
	var encoded []byte
	err = h.chat.access.WithAccess(ctx, raw, tokens[0], func(ctx context.Context, access string) error {
		payload, e := h.chat.upstream.Ask(ctx, access, body)
		if e != nil {
			return e
		}

		reply, e := chatcontract.DecodeReply(payload.Bytes())
		if e != nil {
			return e
		}

		encoded, e = chatcontract.Encode(reply)
		return e
	})
	if err != nil {
		switch {
		case errors.Is(err, websession.ErrDenied), errors.Is(err, chatgateway.ErrDenied):
			webFail(c, 401, "session_rejected")
		case errors.Is(err, chatgateway.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
			errorResponse(c, 504, "chat_timeout", "Chat request timed out.")
		case errors.Is(err, chatgateway.ErrResponse), errors.Is(err, chatcontract.ErrResponse):
			errorResponse(c, 502, "invalid_chat_response", "Unable to retrieve a valid answer.")
		default:
			errorResponse(c, 503, "chat_unavailable", "Chat is temporarily unavailable.")
		}

		return
	}

	if ctx.Err() != nil {
		errorResponse(c, 504, "chat_timeout", "Chat request timed out.")
		return
	}

	c.Data(200, "application/json; charset=utf-8", encoded)
}
