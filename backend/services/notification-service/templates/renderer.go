// Package templates provides embedded, server-owned account notification text.

package templates

import (
	"embed"
	"encoding/json"
	"strings"
	"unicode/utf8"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/notifications"
)

//go:embed en/*.json

var files embed.FS

type emailText struct {
	Subject string   `json:"subject"`
	Body    []string `json:"body"`
}

type inboxText struct {
	Title string   `json:"title"`
	Body  []string `json:"body"`
}

type wording struct {
	Locale string    `json:"locale"`
	Email  emailText `json:"email"`
	InApp  inboxText `json:"in_app"`
}

// EmailContent excludes recipient details; the durable worker resolves those.

type EmailContent struct {
	Subject string
	Body    string
	Locale  string
}

type Renderer struct {
	issuer string
	text   map[string]wording
}

var _ notifications.Renderer = (*Renderer)(nil)

// New loads the complete English catalog before any event can be rendered.

func New(issuer string) (*Renderer, error) {
	if contract.ValidateIssuer(issuer) != nil {
		return nil, notifications.ErrConfiguration
	}
	catalog := map[string]string{
		contract.LoginSucceeded:  "en/login_succeeded.json",
		contract.PasswordChanged: "en/password_changed.json",
		contract.EmailVerified:   "en/email_verified.json",
	}
	renderer := &Renderer{issuer: issuer, text: make(map[string]wording, len(catalog))}
	for eventType, path := range catalog {
		data, err := files.ReadFile(path)
		if err != nil {
			return nil, notifications.ErrTemplate
		}
		var text wording
		if json.Unmarshal(data, &text) != nil || !valid(text) {
			return nil, notifications.ErrTemplate
		}
		renderer.text[eventType] = text
	}
	return renderer, nil
}

// Render implements the inbox renderer without interpolating event data.

func (r *Renderer) Render(event contract.Event) (notifications.Content, error) {
	text, err := r.lookup(event)
	if err != nil {
		return notifications.Content{}, err
	}
	return notifications.Content{
		Title: text.InApp.Title, Body: strings.Join(text.InApp.Body, " "), Locale: text.Locale,
	}, nil
}

// RenderEmail supplies text only; it never connects to SMTP or sends a message.

func (r *Renderer) RenderEmail(event contract.Event) (EmailContent, error) {
	text, err := r.lookup(event)
	if err != nil {
		return EmailContent{}, err
	}
	return EmailContent{
		Subject: text.Email.Subject, Body: strings.Join(text.Email.Body, "\n"), Locale: text.Locale,
	}, nil
}

func (r *Renderer) lookup(event contract.Event) (wording, error) {
	if r == nil || event.Validate(r.issuer) != nil {
		return wording{}, notifications.ErrTemplate
	}
	text, ok := r.text[event.Type]
	if !ok {
		return wording{}, notifications.ErrTemplate
	}
	return text, nil
}

func valid(text wording) bool {
	return text.Locale == "en" && clean(text.InApp.Title, 256, true) &&
		clean(strings.Join(text.InApp.Body, " "), 8192, false) &&
		clean(text.Email.Subject, 256, true) &&
		clean(strings.Join(text.Email.Body, "\n"), 64<<10, false)
}

func clean(text string, limit int, singleLine bool) bool {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > limit {
		return false
	}
	for _, character := range text {
		if character == 127 || (character < 32 &&
			(singleLine || (character != '\n' && character != '\t'))) {
			return false
		}
	}
	return true
}
