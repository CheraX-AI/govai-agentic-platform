package delivery

import (
	"bytes"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/templates"
)

// TestCatalogMIME checks real template text through the existing email encoder.
// This constructs messages locally and never sends email or opens a connection.

func TestCatalogMIME(t *testing.T) {
	issuer := "https://identity.example/realms/GovAI"
	renderer, err := templates.New(issuer)
	if err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{
		contract.LoginSucceeded, contract.PasswordChanged, contract.EmailVerified,
	} {
		t.Run(eventType, func(t *testing.T) {
			event := contract.Event{
				ID: "123e4567-e89b-42d3-a456-426614174000", Type: eventType,
				Version: contract.Version, Issuer: issuer, Subject: "synthetic-owner",
				OccurredAt: time.Date(2026, 11, 25, 5, 0, 0, 0, time.UTC),
			}
			text, err := renderer.RenderEmail(event)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := encodeMessage("notifications@govai.test", Message{
				ID: event.ID, Recipient: "synthetic-citizen@govai.test",
				Subject: text.Subject, Body: text.Body,
			})
			if err != nil {
				t.Fatal("rendered email was rejected by SMTP encoding")
			}
			message, err := mail.ReadMessage(bytes.NewReader(encoded))
			if err != nil {
				t.Fatal(err)
			}
			subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
			if err != nil || subject != text.Subject {
				t.Fatal("encoded subject changed template wording")
			}
			body, err := io.ReadAll(quotedprintable.NewReader(message.Body))
			if err != nil || !bytes.Equal(
				bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n")), []byte(text.Body),
			) {
				t.Fatal("encoded body changed template wording")
			}
		})
	}
}
