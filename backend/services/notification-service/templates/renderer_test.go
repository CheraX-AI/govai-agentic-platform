package templates

import (
	"strings"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/notifications"
)

const testIssuer = "https://identity.example/realms/GovAI"

func eventFor(eventType string) contract.Event {
	return contract.Event{
		ID: "123e4567-e89b-42d3-a456-426614174000", Type: eventType,
		Version: contract.Version, Issuer: testIssuer, Subject: "private-test-owner",
		OccurredAt: time.Date(2026, 11, 25, 5, 0, 0, 0, time.UTC),
	}
}

func TestAccountCatalog(t *testing.T) {
	renderer, err := New(testIssuer)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		eventType string
		title     string
		subject   string
	}{
		{contract.LoginSucceeded, "New sign-in", "New sign-in to your GovAI account"},
		{contract.PasswordChanged, "Password changed", "Your GovAI password was changed"},
		{contract.EmailVerified, "Email verified", "Your GovAI email address is verified"},
	}
	for _, test := range cases {
		t.Run(test.eventType, func(t *testing.T) {
			event := eventFor(test.eventType)
			inbox, err := renderer.Render(event)
			if err != nil || inbox.Title != test.title || inbox.Locale != "en" ||
				!clean(inbox.Body, 8192, false) {
				t.Fatal("inbox catalog entry is invalid")
			}
			email, err := renderer.RenderEmail(event)
			if err != nil || email.Subject != test.subject || email.Locale != "en" ||
				!clean(email.Body, 64<<10, false) {
				t.Fatal("email catalog entry is invalid")
			}
			combined := inbox.Body + email.Body + inbox.Title + email.Subject
			for _, secret := range []string{event.ID, event.Subject, event.Issuer} {
				if strings.Contains(combined, secret) {
					t.Fatal("identity reference leaked into rendered content")
				}
			}

			// Identity and observation time do not change server-owned wording.

			event.Subject = "another-owner"
			event.OccurredAt = event.OccurredAt.Add(time.Hour)
			again, err := renderer.Render(event)
			if err != nil || again != inbox {
				t.Fatal("static text unexpectedly depends on event identity or time")
			}
		})
	}
}

func TestInvalidEventsProduceNoContent(t *testing.T) {
	renderer, err := New(testIssuer)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*contract.Event)
	}{
		{"unsupported_event", func(e *contract.Event) { e.Type = "citizen.login.failed" }},
		{"wrong_issuer", func(e *contract.Event) { e.Issuer = "https://other.example" }},
		{"wrong_version", func(e *contract.Event) { e.Version++ }},
		{"invalid_id", func(e *contract.Event) { e.ID = "untrusted-id" }},
		{"invalid_subject", func(e *contract.Event) { e.Subject = "owner\r\nInjected" }},
		{"missing_time", func(e *contract.Event) { e.OccurredAt = time.Time{} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			event := eventFor(contract.LoginSucceeded)
			test.change(&event)
			content, err := renderer.Render(event)
			if err != notifications.ErrTemplate || content != (notifications.Content{}) {
				t.Fatal("invalid event produced inbox text")
			}
			email, err := renderer.RenderEmail(event)
			if err != notifications.ErrTemplate || email != (EmailContent{}) {
				t.Fatal("invalid event produced email text")
			}
		})
	}
	if _, err := New("http://identity.example"); err != notifications.ErrConfiguration {
		t.Fatal("insecure issuer was accepted")
	}
	var missing *Renderer
	if _, err := missing.Render(eventFor(contract.LoginSucceeded)); err == nil {
		t.Fatal("missing renderer did not fail safely")
	}
	if _, err := missing.RenderEmail(eventFor(contract.LoginSucceeded)); err == nil {
		t.Fatal("missing email renderer did not fail safely")
	}
}

func TestUnsafeCatalogTextIsRejected(t *testing.T) {
	for _, text := range []string{"", "\t", "title\r\nInjected", "nul\x00", "\xff"} {
		if clean(text, 256, true) {
			t.Fatal("unsafe subject or title accepted")
		}
	}
	if clean(strings.Repeat("x", 257), 256, true) {
		t.Fatal("oversized subject accepted")
	}
	if valid(wording{Locale: "si"}) || valid(wording{Locale: "en"}) {
		t.Fatal("unsupported locale or incomplete catalog accepted")
	}
}
