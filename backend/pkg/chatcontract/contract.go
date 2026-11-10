// Package chatcontract defines the shared Go chat request and reply format.
package chatcontract

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
)

var (
	ErrInvalid  = errors.New("invalid chat message")
	ErrTooLarge = errors.New("chat message too large")
	ErrResponse = errors.New("invalid chat response")
)

const MaxRequestBytes = 16 << 10

const MaxMessageRunes = 4000

const MaxResponseBytes = 64 << 10

const MaxSources = 20

const MaxSourceNumber = 100

const MaxSourcePage = 100000

type Question struct{ message string }

func (q Question) Text() string { return q.message }

func (Question) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func (Question) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func (Question) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

// Source is a document cited in the answer. Number matches the [n] marker in
// the answer text. Page (0 = unknown) and URL ("" = none) are optional.
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

func (Reply) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
