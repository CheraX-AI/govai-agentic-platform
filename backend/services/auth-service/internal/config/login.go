package config

import (
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
)

type Login struct {
	Enabled  bool
	Provider login.KeycloakConfig
	RateKey  []byte
	CAFile   string
	Limits   RateLimits // set in cmd/api from *_RATE_LIMIT
}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Login) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (Login) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (Login) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

// LoadLogin validates native login and refresh settings.
func LoadLogin() (Login, error) {
	var c Login
	switch os.Getenv("LOGIN_REFRESH_ENABLED") {
	case "", "false":
	case "true":
		c.Provider.EnableRefresh = true
	default:
		return c, login.ErrUnavailable
	}

	switch os.Getenv("LOGIN_ENABLED") {
	case "", "false":
		if c.Provider.EnableRefresh {
			return Login{}, login.ErrUnavailable
		}
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, login.ErrUnavailable
	}

	if os.Getenv("DATABASE_URL") == "" ||
		os.Getenv("API_TLS_CERT_FILE") == "" ||
		os.Getenv("API_TLS_KEY_FILE") == "" {
		return Login{}, login.ErrUnavailable
	}

	c.Provider = login.KeycloakConfig{
		Issuer:        os.Getenv("LOGIN_ISSUER_URL"),
		ClientID:      os.Getenv("LOGIN_CLIENT_ID"),
		ClientSecret:  os.Getenv("LOGIN_CLIENT_SECRET"),
		Audience:      os.Getenv("LOGIN_AUDIENCE"),
		EnableRefresh: c.Provider.EnableRefresh,
	}
	if c.Provider.ClientID == "" ||
		c.Provider.ClientID == os.Getenv("REGISTRATION_CLIENT_ID") ||
		c.Provider.ClientSecret == "" ||
		c.Provider.Audience == "" {
		return Login{}, login.ErrUnavailable
	}

	var e error
	c.RateKey, e = base64.StdEncoding.DecodeString(os.Getenv("LOGIN_RATE_KEY"))
	if e != nil || len(c.RateKey) != 32 {
		return Login{}, login.ErrUnavailable
	}

	other, _ := base64.StdEncoding.DecodeString(os.Getenv("REGISTRATION_RATE_KEY"))
	if hmac.Equal(c.RateKey, other) {
		return Login{}, login.ErrUnavailable
	}

	c.CAFile = os.Getenv("LOGIN_CA_FILE")
	return c, nil
}
