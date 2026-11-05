// Package chat owns the shared chat contract. It has no Python, database or
// identity-provider implementation; agents are injected through a small port.
package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

var (
	ErrInvalid     = errors.New("invalid chat message")
	ErrTooLarge    = errors.New("chat message too large")
	ErrUnavailable = errors.New("knowledge agent unavailable")
	ErrResponse    = errors.New("invalid knowledge agent response")
)

const MaxRequestBytes = 16 << 10
const MaxMessageRunes = 4000
const MaxResponseBytes = 64 << 10
const MaxSources = 20
const MaxSourceNumber = 100
const MaxSourcePage = 100000

type Question struct{ message string }

func (q Question) Text() string               { return q.message }
func (Question) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (Question) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (Question) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

// Source is a document cited in the answer. Number matches the [n] marker in the
// answer text. Page (0 = unknown) and URL ("" = none) are optional: the agent's
// documents have no public citizen-facing address yet.
type Source struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Page   int    `json:"page,omitempty"`
	URL    string `json:"url,omitempty"`
}
type Reply struct {
	Answer  string   `json:"answer"`
	Sources []Source `json:"sources"`
}

func (Reply) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }
func (Reply) LogValue() slog.Value       { return slog.StringValue("[REDACTED]") }

type Agent interface {
	Ask(context.Context, Question) (Reply, error)
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

// Unavailable is the production default until GA-66 supplies a real adapter.
// It never fabricates an answer, citation or successful integration result.
type Unavailable struct{}

func (Unavailable) Ask(context.Context, Question) (Reply, error) { return Reply{}, ErrUnavailable }
