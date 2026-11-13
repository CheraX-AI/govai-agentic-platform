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
	"strconv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
)

var ErrRecoveryConfig = errors.New("invalid recovery runtime configuration")

type Recovery struct {
	Enabled           bool
	Service           recovery.Config
	SMTP              recovery.SMTPConfig
	FenceKey, RateKey []byte
	Origin            string
}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Recovery) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (Recovery) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (Recovery) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

// LoadRecovery validates password-reset and email settings.
func LoadRecovery(reg Registration, web Web) (Recovery, error) {
	var c Recovery
	switch os.Getenv("RECOVERY_ENABLED") {
	case "", "false":
		// Disabling reset routes must not disable revocation of existing cookies.
		if value := os.Getenv("RECOVERY_FENCE_KEY"); value != "" {
			key, err := base64.StdEncoding.DecodeString(value)
			if err != nil || len(key) != 32 {
				return c, ErrRecoveryConfig
			}

			c.FenceKey = key
		}
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, ErrRecoveryConfig
	}

	if !reg.Enabled ||
		!web.Enabled ||
		os.Getenv("DATABASE_URL") == "" ||
		os.Getenv("API_TLS_CERT_FILE") == "" ||
		os.Getenv("API_TLS_KEY_FILE") == "" {
		return Recovery{}, ErrRecoveryConfig
	}

	issuer := reg.Admin.BaseURL + "/realms/" + reg.Admin.Realm
	if issuer != web.Provider.Issuer {
		return Recovery{}, ErrRecoveryConfig
	}

	c.Origin = web.Origin
	for _, value := range []string{issuer, os.Getenv("RECOVERY_RESET_URL"), c.Origin} {
		u, err := url.Parse(value)
		if err != nil ||
			u.Scheme != "https" ||
			u.Hostname() == "" ||
			u.User != nil ||
			u.RawQuery != "" ||
			u.ForceQuery ||
			u.Fragment != "" ||
			len(value) > 2048 {
			return Recovery{}, ErrRecoveryConfig
		}
	}

	origin, _ := url.Parse(c.Origin)
	if origin.Path != "" {
		return Recovery{}, ErrRecoveryConfig
	}

	keys := make([][]byte, 0, 3)
	for _, name := range []string{"RECOVERY_BINDING_KEY", "RECOVERY_FENCE_KEY", "RECOVERY_RATE_KEY"} {
		key, err := base64.StdEncoding.DecodeString(os.Getenv(name))
		if err != nil || len(key) != 32 {
			return Recovery{}, ErrRecoveryConfig
		}

		for _, previous := range keys {
			if hmac.Equal(previous, key) {
				return Recovery{}, ErrRecoveryConfig
			}
		}

		for _, otherName := range []string{
			"PROFILE_DATA_KEY",
			"NIC_INDEX_KEY",
			"WEB_SESSION_DATA_KEY",
			"WEB_RATE_KEY",
			"REGISTRATION_FINGERPRINT_KEY",
			"REGISTRATION_RATE_KEY",
			"LOGIN_RATE_KEY",
		} {
			other, _ := base64.StdEncoding.DecodeString(os.Getenv(otherName))
			if hmac.Equal(other, key) {
				return Recovery{}, ErrRecoveryConfig
			}
		}

		keys = append(keys, key)
	}

	ttl := 15 * time.Minute
	if value := os.Getenv("RECOVERY_LIFETIME"); value != "" {
		var err error
		ttl, err = time.ParseDuration(value)
		if err != nil {
			return Recovery{}, ErrRecoveryConfig
		}
	}

	if ttl < time.Minute || ttl > 30*time.Minute {
		return Recovery{}, ErrRecoveryConfig
	}

	c.Service = recovery.Config{
		Issuer:     issuer,
		ResetURL:   os.Getenv("RECOVERY_RESET_URL"),
		BindingKey: keys[0],
		Lifetime:   ttl,
		Timeout:    25 * time.Second,
	}
	c.FenceKey = keys[1]
	c.RateKey = keys[2]
	port, err := strconv.Atoi(os.Getenv("RECOVERY_SMTP_PORT"))
	if err != nil {
		return Recovery{}, ErrRecoveryConfig
	}

	dev := false
	switch os.Getenv("RECOVERY_SMTP_DEV_PLAINTEXT") {
	case "", "false":
	case "true":
		dev = true
	default:
		return Recovery{}, ErrRecoveryConfig
	}

	c.SMTP = recovery.SMTPConfig{
		Host:                 os.Getenv("RECOVERY_SMTP_HOST"),
		Port:                 port,
		From:                 os.Getenv("RECOVERY_SMTP_FROM"),
		Username:             os.Getenv("RECOVERY_SMTP_USERNAME"),
		Password:             os.Getenv("RECOVERY_SMTP_PASSWORD"),
		DevelopmentPlaintext: dev,
		Timeout:              5 * time.Second,
	}
	if _, err = recovery.NewSMTPMailer(c.SMTP); err != nil {
		return Recovery{}, ErrRecoveryConfig
	}

	return c, nil
}
