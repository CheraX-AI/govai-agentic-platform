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

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Question) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (Question) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (Question) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

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

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Reply) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// LogValue hides the value when it is written to a slog log.
func (Reply) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}
