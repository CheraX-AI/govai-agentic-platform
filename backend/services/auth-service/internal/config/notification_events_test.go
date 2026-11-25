package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNotificationEventsSettings(t *testing.T) {
	t.Setenv("AUTH_NOTIFICATION_EVENTS_ENABLED", "false")
	if settings, err := LoadNotificationEvents(); err != nil || settings.Enabled {
		t.Fatal("notifications must be disabled by default")
	}
	t.Setenv("AUTH_NOTIFICATION_EVENTS_ENABLED", "true")
	t.Setenv("AUTH_NOTIFICATION_ISSUER_URL", "https://identity.test/realms/outbox-test")
	t.Setenv("AUTH_NOTIFICATION_RABBITMQ_HOST", "127.0.0.1")
	t.Setenv("AUTH_NOTIFICATION_RABBITMQ_PORT", "5672")
	t.Setenv("AUTH_NOTIFICATION_RABBITMQ_SCHEME", "amqp")
	t.Setenv("AUTH_NOTIFICATION_RABBITMQ_USER", "synthetic-publisher")
	t.Setenv("AUTH_NOTIFICATION_RABBITMQ_PASSWORD", "synthetic-private-password")
	t.Setenv("AUTH_NOTIFICATION_RABBITMQ_CA_FILE", "")
	t.Setenv("AUTH_NOTIFICATION_TIMEOUT", "1s")
	t.Setenv("AUTH_NOTIFICATION_ENV", "development")
	t.Setenv("DATABASE_URL", "synthetic-config-only")
	settings, err := LoadNotificationEvents()
	if err != nil || !settings.Enabled {
		t.Fatal("valid local notification settings rejected")
	}
	encoded, _ := json.Marshal(settings)
	if strings.Contains(fmt.Sprintf("%+v", settings), settings.Publisher.Password) ||
		strings.Contains(string(encoded), settings.Publisher.Password) {
		t.Fatal("notification settings leaked credentials")
	}
	for _, name := range []string{
		"AUTH_NOTIFICATION_EVENTS_ENABLED", "AUTH_NOTIFICATION_ISSUER_URL",
		"AUTH_NOTIFICATION_RABBITMQ_PORT", "AUTH_NOTIFICATION_RABBITMQ_SCHEME",
		"AUTH_NOTIFICATION_TIMEOUT", "AUTH_NOTIFICATION_ENV",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "invalid")
			if _, err := LoadNotificationEvents(); err == nil {
				t.Fatal("invalid notification settings accepted")
			}
		})
	}
}
