package config

import (
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
)

type Registration struct {
	Enabled                         bool
	Admin                           keycloak.RegistrationConfig
	FingerprintKey, RateKey         []byte
	CAFile, TLSCertFile, TLSKeyFile string
	Limits                          RateLimits // set in cmd/api from *_RATE_LIMIT
}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Registration) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (Registration) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (Registration) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

var registrationSegment = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

var ErrRegistrationConfig = errors.New("invalid registration runtime configuration")

// LoadRegistration validates registration settings before enabling the route.
func LoadRegistration() (Registration, error) {
	var c Registration
	switch os.Getenv("REGISTRATION_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, ErrRegistrationConfig
	}

	c.TLSCertFile = os.Getenv("API_TLS_CERT_FILE")
	c.TLSKeyFile = os.Getenv("API_TLS_KEY_FILE")
	c.CAFile = os.Getenv("REGISTRATION_CA_FILE")
	if c.TLSCertFile == "" || c.TLSKeyFile == "" || os.Getenv("DATABASE_URL") == "" {
		return Registration{}, ErrRegistrationConfig
	}

	var err error
	c.FingerprintKey, err = base64.StdEncoding.DecodeString(os.Getenv("REGISTRATION_FINGERPRINT_KEY"))
	if err != nil || len(c.FingerprintKey) != 32 {
		return Registration{}, ErrRegistrationConfig
	}

	c.RateKey, err = base64.StdEncoding.DecodeString(os.Getenv("REGISTRATION_RATE_KEY"))
	if err != nil || len(c.RateKey) != 32 || hmac.Equal(c.RateKey, c.FingerprintKey) {
		return Registration{}, ErrRegistrationConfig
	}

	c.Admin = keycloak.RegistrationConfig{
		BaseURL: os.Getenv("REGISTRATION_KEYCLOAK_URL"), Realm: os.Getenv("REGISTRATION_REALM"),
		ClientID: os.Getenv("REGISTRATION_CLIENT_ID"), ClientSecret: os.Getenv("REGISTRATION_CLIENT_SECRET"),
		RegistrationGroupPath: os.Getenv("REGISTRATION_GROUP_PATH"),
		VerificationClientID:  os.Getenv("REGISTRATION_VERIFICATION_CLIENT_ID"),
		VerificationRedirect:  os.Getenv("REGISTRATION_VERIFICATION_REDIRECT"),
		VerificationLifespan:  15 * time.Minute,
		CallTimeout:           5 * time.Second,
		MaxResponseBytes:      64 << 10,
		TokenSkew:             5 * time.Second,
	}
	u, e := url.Parse(c.Admin.BaseURL)
	if e != nil ||
		u.Scheme != "https" ||
		u.Hostname() == "" ||
		u.User != nil ||
		u.RawQuery != "" ||
		u.ForceQuery ||
		u.Fragment != "" ||
		strings.HasSuffix(c.Admin.BaseURL, "/") ||
		!registrationSegment.MatchString(c.Admin.Realm) ||
		c.Admin.ClientID == "" ||
		c.Admin.ClientSecret == "" ||
		c.Admin.VerificationClientID == "" {
		return Registration{}, ErrRegistrationConfig
	}
	// The adapter also validates URL paths, group and verification callback before
	// any request. These settings are server-only and never taken from request JSON.
	return c, nil
}
