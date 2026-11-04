package chatgateway

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
)

var ErrInvalid = errors.New("invalid chat message")
var ErrTooLarge = errors.New("chat message too large")

const MaxRequestBytes = 16 << 10
const MaxMessageRunes = 4000
const MaxResponseBytes = 64 << 10

type Question struct{ message string }

func (q Question) Text() string               { return q.message }
func (Question) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (Question) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (Question) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
type Reply struct {
	Answer  string   `json:"answer"`
	Sources []Source `json:"sources"`
}

func (Reply) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }
func (Reply) LogValue() slog.Value       { return slog.StringValue("[REDACTED]") }
