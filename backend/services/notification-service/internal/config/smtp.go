package config

import (
	"os"
	"strconv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/delivery"
)

// LoadSMTP validates optional email settings without connecting or sending.

func LoadSMTP() (delivery.Config, bool, error) {
	enabled := value("EMAIL_ENABLED", "false")
	if enabled == "false" {
		return delivery.Config{}, false, nil
	}
	if enabled != "true" {
		return delivery.Config{}, false, ErrConfig
	}
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		return delivery.Config{}, false, ErrConfig
	}
	timeout, err := time.ParseDuration(value("SMTP_TIMEOUT", "10s"))
	if err != nil {
		return delivery.Config{}, false, ErrConfig
	}
	settings := delivery.Config{
		Host:        os.Getenv("SMTP_HOST"),
		Port:        port,
		Username:    os.Getenv("SMTP_USERNAME"),
		Password:    os.Getenv("SMTP_PASSWORD"),
		From:        os.Getenv("SMTP_FROM"),
		TLSMode:     value("SMTP_TLS_MODE", "starttls"),
		Timeout:     timeout,
		Development: os.Getenv("NOTIFICATION_ENV") == "development",
	}
	if _, err := delivery.New(settings); err != nil {
		return delivery.Config{}, false, ErrConfig
	}
	return settings, true, nil
}
