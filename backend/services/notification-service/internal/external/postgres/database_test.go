package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		Host: "127.0.0.1", Port: 5432,
		Username: "notification_test", Password: "synthetic:@/?password",
		Database: "govai_notifications", TLSMode: "disable",
		Development: true, MaxConnections: 4, Timeout: time.Second,
	}
}

func TestPoolConfiguration(t *testing.T) {
	settings := testConfig()
	parsed, err := poolConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	connection := parsed.ConnConfig
	if connection.Password != settings.Password || connection.User != settings.Username ||
		connection.Database != settings.Database || parsed.MaxConns != 4 ||
		connection.ConnectTimeout != time.Second || connection.TLSConfig != nil ||
		connection.RuntimeParams["statement_timeout"] != "1000" {
		t.Fatal("connection settings were not preserved")
	}
	settings.Development = false
	settings.TLSMode = "verify-full"
	parsed, err = poolConfig(settings)
	if err != nil || parsed.ConnConfig.TLSConfig == nil {
		t.Fatal("verified TLS configuration missing")
	}
	if parsed.ConnConfig.TLSConfig.InsecureSkipVerify ||
		parsed.ConnConfig.TLSConfig.ServerName != settings.Host {
		t.Fatal("TLS must verify the configured database hostname")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	cases := map[string]func(*Config){
		"production plaintext":   func(c *Config) { c.Development = false },
		"remote plaintext":       func(c *Config) { c.Host = "remote.example" },
		"unverified TLS":         func(c *Config) { c.TLSMode = "require" },
		"fallback TLS":           func(c *Config) { c.TLSMode = "prefer" },
		"other service database": func(c *Config) { c.Database = "govai_auth" },
		"empty password":         func(c *Config) { c.Password = "" },
		"empty username":         func(c *Config) { c.Username = "" },
		"host injection":         func(c *Config) { c.Host = "host?sslmode=disable" },
		"bad port":               func(c *Config) { c.Port = 0 },
		"unbounded pool":         func(c *Config) { c.MaxConnections = 1000 },
		"unbounded timeout":      func(c *Config) { c.Timeout = time.Hour },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			settings := testConfig()
			change(&settings)
			if !errors.Is(Validate(settings), ErrConfiguration) {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
}

func TestRedactionAndCancellation(t *testing.T) {
	settings := testConfig()
	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{
		fmt.Sprintf("%+v", settings), fmt.Sprintf("%#v", settings),
		string(encoded), settings.LogValue().String(),
	} {
		if strings.Contains(output, settings.Password) ||
			strings.Contains(output, settings.Username) {
			t.Fatal("database settings leaked")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, settings); !errors.Is(err, ErrUnavailable) {
		t.Fatal("canceled connection should fail safely")
	}
}

// This opt-in probe uses a disposable test server, never a service database.

func TestConnectionIntegration(t *testing.T) {
	password := os.Getenv("NOTIFICATION_TEST_DB_PASSWORD")
	if password == "" {
		t.Skip("disposable PostgreSQL fixture not configured")
	}
	settings := testConfig()
	settings.Password = password
	database, err := Open(context.Background(), settings)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	var name, user string
	if err := database.pool.QueryRow(context.Background(),
		"SELECT current_database(), current_user").Scan(&name, &user); err != nil {
		t.Fatal("identity probe failed")
	}
	if name != settings.Database || user != settings.Username {
		t.Fatal("connected to unexpected database identity")
	}
	settings.Password = "intentionally-wrong-test-password"
	if _, err := Open(context.Background(), settings); !errors.Is(err, ErrUnavailable) {
		t.Fatal("invalid credentials accepted")
	}
	database.Close()
	if err := database.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("closed pool reported healthy")
	}
}
