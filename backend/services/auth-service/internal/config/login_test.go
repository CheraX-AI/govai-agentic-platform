package config

import (
	"encoding/base64"
	"testing"
)

func TestLoginConfiguration(t *testing.T) {
	t.Setenv("LOGIN_REFRESH_ENABLED", "")
	t.Setenv("LOGIN_ENABLED", "")
	if c, e := LoadLogin(); e != nil || c.Enabled {
		t.Fatal("default enabled")
	}

	for k, v := range map[string]string{
		"LOGIN_ENABLED":          "true",
		"DATABASE_URL":           "local",
		"API_TLS_CERT_FILE":      "cert",
		"API_TLS_KEY_FILE":       "key",
		"LOGIN_ISSUER_URL":       "https://identity.example.test/realms/test",
		"LOGIN_CLIENT_ID":        "mobile",
		"REGISTRATION_CLIENT_ID": "registration",
		"LOGIN_CLIENT_SECRET":    "private",
		"LOGIN_AUDIENCE":         "govai-api",
		"LOGIN_RATE_KEY":         base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"REGISTRATION_RATE_KEY":  "",
	} {
		t.Setenv(k, v)
	}

	if _, e := LoadLogin(); e != nil {
		t.Fatal(e)
	}

	t.Setenv("LOGIN_CLIENT_ID", "registration")
	if _, e := LoadLogin(); e == nil {
		t.Fatal("administrative client reused")
	}
}

func TestRefreshRequiresLogin(t *testing.T) {
	t.Setenv("LOGIN_ENABLED", "false")
	for _, value := range []string{"true", "yes", "1"} {
		t.Setenv("LOGIN_REFRESH_ENABLED", value)
		if _, err := LoadLogin(); err == nil {
			t.Fatal("invalid refresh configuration accepted")
		}
	}
}
