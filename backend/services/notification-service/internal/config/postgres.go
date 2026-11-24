package config

import (
	"os"
	"strconv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/external/postgres"
)

// LoadPostgres prepares optional database settings without enabling consumption.

func LoadPostgres() (postgres.Config, bool, error) {
	enabled := value("NOTIFICATION_DATABASE_ENABLED", "false")
	if enabled == "false" {
		return postgres.Config{}, false, nil
	}
	if enabled != "true" {
		return postgres.Config{}, false, ErrConfig
	}
	port, err := strconv.Atoi(value("NOTIFICATION_DB_PORT", "5432"))
	if err != nil {
		return postgres.Config{}, false, ErrConfig
	}
	limit, err := strconv.ParseInt(value("NOTIFICATION_DB_MAX_CONNECTIONS", "8"), 10, 32)
	if err != nil {
		return postgres.Config{}, false, ErrConfig
	}
	timeout, err := time.ParseDuration(value("NOTIFICATION_DB_TIMEOUT", "5s"))
	if err != nil {
		return postgres.Config{}, false, ErrConfig
	}
	settings := postgres.Config{
		Host:           value("NOTIFICATION_DB_HOST", "127.0.0.1"),
		Port:           port,
		Username:       os.Getenv("NOTIFICATION_DB_USER"),
		Password:       os.Getenv("NOTIFICATION_DB_PASSWORD"),
		Database:       value("NOTIFICATION_DB_NAME", "govai_notifications"),
		TLSMode:        value("NOTIFICATION_DB_SSLMODE", "verify-full"),
		CAFile:         os.Getenv("NOTIFICATION_DB_CA_FILE"),
		Development:    os.Getenv("NOTIFICATION_ENV") == "development",
		MaxConnections: int32(limit),
		Timeout:        timeout,
	}
	if postgres.Validate(settings) != nil {
		return postgres.Config{}, false, ErrConfig
	}
	return settings, true, nil
}
