// Package inbox defines owner-scoped notification queries for the citizen API.

package inbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

var ErrUnavailable = errors.New("notification inbox unavailable")

// Owner is populated only from verified authentication, never request JSON.

type Owner struct {
	Issuer  string
	Subject string
}

func (Owner) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED notification owner]")
}

func (Owner) LogValue() slog.Value {
	return slog.StringValue("[REDACTED notification owner]")
}

func (Owner) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED notification owner]"`), nil
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

type Page struct {
	Limit  int
	Before *Cursor
}

// Notification deliberately excludes owner, delivery errors and SMTP metadata.

type Notification struct {
	ID        string     `json:"id"`
	EventType string     `json:"event_type"`
	Priority  string     `json:"priority"`
	Locale    string     `json:"locale"`
	Title     string     `json:"title"`
	Body      string     `json:"message_body"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

type Result struct {
	Items   []Notification
	HasMore bool
}

// Store must scope every SQL statement by issuer AND subject. List uses descending
// (created_at, id) keyset order. Missing and other-owner IDs are indistinguishable.
// MarkRead is idempotent and preserves the first server-assigned read timestamp.

type Store interface {
	List(context.Context, Owner, Page) (Result, error)
	MarkRead(context.Context, Owner, string) (bool, error)
	UnreadCount(context.Context, Owner) (int64, error)
}
