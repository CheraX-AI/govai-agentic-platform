package config

import (
	"testing"
)

func TestLoadPostgres(t *testing.T) {
	t.Setenv("NOTIFICATION_DATABASE_ENABLED", "false")
	if _, enabled, err := LoadPostgres(); err != nil || enabled {
		t.Fatal("database should remain disabled")
	}
	t.Setenv("NOTIFICATION_DATABASE_ENABLED", "true")
	t.Setenv("NOTIFICATION_ENV", "development")
	t.Setenv("NOTIFICATION_DB_HOST", "127.0.0.1")
	t.Setenv("NOTIFICATION_DB_PORT", "5432")
	t.Setenv("NOTIFICATION_DB_USER", "synthetic-user")
	t.Setenv("NOTIFICATION_DB_PASSWORD", "synthetic-password")
	t.Setenv("NOTIFICATION_DB_NAME", "govai_notifications")
	t.Setenv("NOTIFICATION_DB_SSLMODE", "disable")
	t.Setenv("NOTIFICATION_DB_CA_FILE", "")
	t.Setenv("NOTIFICATION_DB_MAX_CONNECTIONS", "4")
	t.Setenv("NOTIFICATION_DB_TIMEOUT", "1s")
	if _, enabled, err := LoadPostgres(); err != nil || !enabled {
		t.Fatal("valid local database settings rejected")
	}
	for _, name := range []string{
		"NOTIFICATION_DATABASE_ENABLED", "NOTIFICATION_DB_PORT",
		"NOTIFICATION_DB_MAX_CONNECTIONS", "NOTIFICATION_DB_TIMEOUT",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "invalid")
			if _, _, err := LoadPostgres(); err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}
