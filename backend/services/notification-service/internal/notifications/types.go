package notifications

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

var (
	ErrConfiguration  = errors.New("notification processor is not configured")
	ErrStorage        = errors.New("notification storage unavailable")
	ErrConflict       = errors.New("notification event identity conflict")
	ErrTemplate       = errors.New("notification template unavailable")
	ErrAcknowledgment = errors.New("notification acknowledgment failed")
)

type SaveResult uint8

const (
	Saved SaveResult = iota + 1
	Duplicate
)

// Content is server-rendered English text, never message text supplied by an event.

type Content struct {
	Title  string
	Body   string
	Locale string
}

type Notification struct {
	Event    contract.Event
	Priority contract.Priority
	Content  Content
}

// Store returns only after atomic inbox/deduplication persistence commits.
// Duplicate requires a matching stored event; conflicting ID reuse returns ErrConflict.

type Store interface {
	Save(context.Context, Notification) (SaveResult, error)
}

// Renderer supplies approved templates; no email is sent during event processing.

type Renderer interface {
	Render(contract.Event) (Content, error)
}

// Delivery binds payload to its broker route and a single-message acknowledgment.

type Delivery struct {
	Body        []byte
	RoutingKey  string
	Queue       string
	Acknowledge func() error
}

func validContent(content Content) bool {
	return content.Locale == "en" &&
		utf8.ValidString(content.Title) && utf8.ValidString(content.Body) &&
		strings.TrimSpace(content.Title) != "" &&
		strings.TrimSpace(content.Body) != "" &&
		len(content.Title) <= 256 && len(content.Body) <= 8192
}
