package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func registrationEnv(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"REGISTRATION_ENABLED": "true", "DATABASE_URL": "postgres://app@127.0.0.1/db?sslmode=disable",
		"API_TLS_CERT_FILE": "private-cert.pem", "API_TLS_KEY_FILE": "private-key.pem",
		"REGISTRATION_FINGERPRINT_KEY": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)),
		"REGISTRATION_RATE_KEY":        base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)),
		"REGISTRATION_KEYCLOAK_URL":    "https://identity.example.test", "REGISTRATION_REALM": "GovAI",
		"REGISTRATION_CLIENT_ID":              "registration-service",
		"REGISTRATION_CLIENT_SECRET":          "never-print-this-secret",
		"REGISTRATION_GROUP_PATH":             "/ga48-registration",
		"REGISTRATION_VERIFICATION_CLIENT_ID": "web-verification",
		"REGISTRATION_VERIFICATION_REDIRECT":  "https://app.example.test/verified",
	}
	for k, v := range values {
		t.Setenv(k, v)
	}
}

func TestRegistrationConfiguration(t *testing.T) {
	registrationEnv(t)
	c, e := LoadRegistration()
	if e != nil || !c.Enabled {
		t.Fatal("valid configuration rejected")
	}

	if strings.Contains(fmt.Sprintf("%+v", c), "never-print") {
		t.Fatal("configuration leaked secret")
	}

	for key, value := range map[string]string{
		"REGISTRATION_ENABLED":         "yes",
		"DATABASE_URL":                 "",
		"API_TLS_KEY_FILE":             "",
		"REGISTRATION_KEYCLOAK_URL":    "http://localhost:8180",
		"REGISTRATION_CLIENT_SECRET":   "",
		"REGISTRATION_FINGERPRINT_KEY": "bad",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, e := LoadRegistration(); e == nil {
				t.Fatal("incomplete/unsafe configuration accepted")
			}
		})
	}

	t.Run("disabled", func(t *testing.T) {
		t.Setenv("REGISTRATION_ENABLED", "false")
		t.Setenv("REGISTRATION_KEYCLOAK_URL", "bad")
		if c, e := LoadRegistration(); e != nil || c.Enabled {
			t.Fatal("disabled mode changed")
		}
	})
	t.Run("independent-keys", func(t *testing.T) {
		t.Setenv("REGISTRATION_RATE_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
		if _, e := LoadRegistration(); e == nil {
			t.Fatal("shared keys accepted")
		}
	})
}
