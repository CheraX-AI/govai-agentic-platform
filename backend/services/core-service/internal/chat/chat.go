// Package chat connects the core service to the knowledge agent.
package chat

import (
	"context"
	"errors"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
)

var (
	ErrUnavailable = errors.New("knowledge agent unavailable")
	ErrResponse    = errors.New("invalid knowledge agent response")
)

type Agent interface {
	Ask(context.Context, chatcontract.Question) (chatcontract.Reply, error)
}

type requestIDKey struct{}

// WithRequestID lets the HTTP adapter forward the core request ID upstream, so
// one question can be followed through core, agent and knowledge-service logs.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	if len(id) == 0 || len(id) > 64 {
		return ""
	}

	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
			return ""
		}
	}

	return id
}

// Unavailable is used when no knowledge agent is configured.
// It never fabricates an answer, citation or successful integration result.
type Unavailable struct{}

func (Unavailable) Ask(context.Context, chatcontract.Question) (chatcontract.Reply, error) {
	return chatcontract.Reply{}, ErrUnavailable
}
