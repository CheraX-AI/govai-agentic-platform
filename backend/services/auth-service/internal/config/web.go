package config

import (
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/websession"
)

type Web struct {
	Enabled          bool
	Provider         login.KeycloakConfig
	Origin, CAFile   string
	DataKey, RateKey []byte
	Lifetime         time.Duration
	Limits           RateLimits // set in cmd/api from *_RATE_LIMIT
}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Web) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (Web) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (Web) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

// LoadWeb validates browser sessions, cookie settings and trusted origin.
func LoadWeb() (Web, error) {
	var c Web
	switch os.Getenv("WEB_SESSION_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, websession.ErrUnavailable
	}

	if os.Getenv("DATABASE_URL") == "" ||
		os.Getenv("API_TLS_CERT_FILE") == "" ||
		os.Getenv("API_TLS_KEY_FILE") == "" {
		return Web{}, websession.ErrUnavailable
	}

	c.Provider = login.KeycloakConfig{
		Issuer:        os.Getenv("WEB_ISSUER_URL"),
		ClientID:      os.Getenv("WEB_CLIENT_ID"),
		ClientSecret:  os.Getenv("WEB_CLIENT_SECRET"),
		Audience:      os.Getenv("WEB_AUDIENCE"),
		EnableRefresh: true,
	}
	if c.Provider.ClientID == "" ||
		c.Provider.ClientSecret == "" ||
		c.Provider.Audience == "" ||
		c.Provider.ClientID == os.Getenv("LOGIN_CLIENT_ID") ||
		c.Provider.ClientID == os.Getenv("REGISTRATION_CLIENT_ID") {
		return Web{}, websession.ErrUnavailable
	}

	c.Origin = os.Getenv("WEB_ORIGIN")
	u, err := url.Parse(c.Origin)
	if err != nil ||
		u.Scheme != "https" ||
		u.Host == "" ||
		u.User != nil ||
		u.Path != "" ||
		u.RawQuery != "" ||
		u.ForceQuery ||
		u.Fragment != "" {
		return Web{}, websession.ErrUnavailable
	}

	c.CAFile = os.Getenv("WEB_CA_FILE")
	c.DataKey, err = base64.StdEncoding.DecodeString(os.Getenv("WEB_SESSION_DATA_KEY"))
	if err != nil || len(c.DataKey) != 32 {
		return Web{}, websession.ErrUnavailable
	}

	c.RateKey, err = base64.StdEncoding.DecodeString(os.Getenv("WEB_RATE_KEY"))
	if err != nil || len(c.RateKey) != 32 || hmac.Equal(c.DataKey, c.RateKey) {
		return Web{}, websession.ErrUnavailable
	}

	for _, name := range []string{"PROFILE_DATA_KEY", "NIC_INDEX_KEY", "REGISTRATION_RATE_KEY", "LOGIN_RATE_KEY"} {
		other, _ := base64.StdEncoding.DecodeString(os.Getenv(name))
		if hmac.Equal(c.DataKey, other) || hmac.Equal(c.RateKey, other) {
			return Web{}, websession.ErrUnavailable
		}
	}

	value := os.Getenv("WEB_SESSION_LIFETIME")
	if value == "" {
		value = "30m"
	}

	c.Lifetime, err = time.ParseDuration(value)
	if err != nil || c.Lifetime < time.Minute || c.Lifetime > 24*time.Hour {
		return Web{}, websession.ErrUnavailable
	}

	return c, nil
}
