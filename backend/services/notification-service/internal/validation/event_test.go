package validation

import (
	"encoding/json"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

// TestEventRouteIsolation keeps security and normal processing separate on replay.

func TestEventRouteIsolation(t *testing.T) {
	event := contract.Event{
		ID:         "123e4567-e89b-42d3-a456-426614174000",
		Version:    contract.Version,
		Issuer:     "https://identity.example/realms/govai",
		Subject:    "synthetic-subject",
		OccurredAt: time.Date(2026, 11, 21, 5, 0, 0, 0, time.UTC),
	}
	cases := []struct {
		eventType string
		key       string
		queue     string
		valid     bool
	}{
		{contract.LoginSucceeded, contract.LoginSucceeded, "notifications.security.v1", true},
		{contract.PasswordChanged, contract.PasswordChanged, "notifications.security.v1", true},
		{contract.EmailVerified, contract.EmailVerified, "notifications.account.v1", true},
		{contract.LoginSucceeded, "delivery.security.retry.v1", "notifications.security.v1", true},
		{contract.EmailVerified, "delivery.retry.v1", "notifications.account.v1", true},
		{contract.LoginSucceeded, contract.LoginSucceeded, "notifications.account.v1", false},
		{contract.EmailVerified, contract.EmailVerified, "notifications.security.v1", false},
		{contract.LoginSucceeded, contract.PasswordChanged, "notifications.security.v1", false},
		{contract.LoginSucceeded, "delivery.retry.v1", "notifications.security.v1", false},
		{contract.EmailVerified, "delivery.security.retry.v1", "notifications.account.v1", false},
		{contract.EmailVerified, contract.EmailVerified, "notifications.dead.v1", false},
	}
	for _, item := range cases {
		event.Type = item.eventType
		body, err := json.Marshal(event)
		if err != nil {
			t.Fatal("synthetic encoding failed")
		}
		_, err = DecodeEvent(body, event.Issuer, item.key, item.queue)
		if (err == nil) != item.valid {
			t.Fatal("incorrect queue/routing validation result")
		}
	}
	if _, err := DecodeEvent([]byte("invalid"), event.Issuer, "", ""); err == nil {
		t.Fatal("invalid payload accepted")
	}
}
