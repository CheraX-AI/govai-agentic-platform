package config

import (
	"os"
	"strconv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/rabbitmq"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
)

type NotificationEvents struct {
	Enabled   bool
	Publisher rabbitmq.Config
}

// LoadNotificationEvents is opt-in and never connects to RabbitMQ during loading.

func LoadNotificationEvents() (NotificationEvents, error) {
	var settings NotificationEvents
	switch os.Getenv("AUTH_NOTIFICATION_EVENTS_ENABLED") {
	case "", "false":
		return settings, nil
	case "true":
		settings.Enabled = true
	default:
		return settings, notificationevents.ErrConfiguration
	}
	port := 5671
	if raw := os.Getenv("AUTH_NOTIFICATION_RABBITMQ_PORT"); raw != "" {
		var err error
		port, err = strconv.Atoi(raw)
		if err != nil {
			return NotificationEvents{}, notificationevents.ErrConfiguration
		}
	}
	timeout := 5 * time.Second
	if raw := os.Getenv("AUTH_NOTIFICATION_TIMEOUT"); raw != "" {
		var err error
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return NotificationEvents{}, notificationevents.ErrConfiguration
		}
	}
	settings.Publisher = rabbitmq.Config{
		Host:     notificationValue("AUTH_NOTIFICATION_RABBITMQ_HOST", "127.0.0.1"),
		Port:     port,
		Username: notificationValue("AUTH_NOTIFICATION_RABBITMQ_USER", "govai_auth"),
		Password: os.Getenv("AUTH_NOTIFICATION_RABBITMQ_PASSWORD"),
		Scheme:   notificationValue("AUTH_NOTIFICATION_RABBITMQ_SCHEME", "amqps"),
		VHost:    "govai", CAFile: os.Getenv("AUTH_NOTIFICATION_RABBITMQ_CA_FILE"),
		Issuer: os.Getenv("AUTH_NOTIFICATION_ISSUER_URL"), Timeout: timeout,
		Development: os.Getenv("AUTH_NOTIFICATION_ENV") == "development",
	}
	if os.Getenv("DATABASE_URL") == "" {
		return NotificationEvents{}, notificationevents.ErrConfiguration
	}
	if _, err := rabbitmq.New(settings.Publisher); err != nil {
		return NotificationEvents{}, notificationevents.ErrConfiguration
	}
	return settings, nil
}

func notificationValue(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
