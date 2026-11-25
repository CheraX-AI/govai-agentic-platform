package templates_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/notifications"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/templates"
)

type memoryStore struct {
	row *notifications.Notification
}

func (s *memoryStore) Save(_ context.Context, row notifications.Notification) (
	notifications.SaveResult, error,
) {
	if s.row != nil {
		return notifications.Duplicate, nil
	}
	s.row = &row
	return notifications.Saved, nil
}

// TestProcessorWithCatalog exercises the real renderer and save-before-ACK path.
// The in-memory store does not establish PostgreSQL deduplication guarantees.

func TestProcessorWithCatalog(t *testing.T) {
	issuer := "https://identity.example/realms/GovAI"
	renderer, err := templates.New(issuer)
	if err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{
		contract.LoginSucceeded, contract.PasswordChanged, contract.EmailVerified,
	} {
		t.Run(eventType, func(t *testing.T) {
			store := &memoryStore{}
			processor, err := notifications.New(issuer, store, renderer)
			if err != nil {
				t.Fatal(err)
			}
			event := contract.Event{
				ID: "123e4567-e89b-42d3-a456-426614174000", Type: eventType,
				Version: contract.Version, Issuer: issuer, Subject: "synthetic-owner",
				OccurredAt: time.Date(2026, 11, 25, 5, 0, 0, 0, time.UTC),
			}
			body, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			priority, _ := contract.PriorityFor(eventType)
			queue := "notifications.security.v1"
			if priority == contract.Normal {
				queue = "notifications.account.v1"
			}
			acks := 0
			delivery := notifications.Delivery{
				Body: body, RoutingKey: eventType, Queue: queue,
				Acknowledge: func() error {
					if store.row == nil || store.row.Content.Locale != "en" ||
						store.row.Content.Title == "" || store.row.Priority != priority {
						t.Fatal("ACK occurred before rendered content was saved")
					}
					acks++
					return nil
				},
			}
			for _, expected := range []notifications.SaveResult{
				notifications.Saved, notifications.Duplicate,
			} {
				result, err := processor.Handle(context.Background(), delivery)
				if err != nil || result != expected {
					t.Fatal("catalog processing failed")
				}
			}
			if acks != 2 {
				t.Fatal("saved event and matching replay were not acknowledged")
			}
		})
	}
}
