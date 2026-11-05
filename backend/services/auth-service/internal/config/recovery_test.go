package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
)

func recoveryEnv(t *testing.T) (Registration, Web) {
	t.Helper()
	registrationEnv(t)
	reg, err := LoadRegistration()
	if err != nil {
		t.Fatal(err)
	}

	for k, v := range map[string]string{
		"RECOVERY_ENABLED":            "true",
		"RECOVERY_RESET_URL":          "https://app.example.test/reset",
		"RECOVERY_BINDING_KEY":        base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{51}, 32)),
		"RECOVERY_FENCE_KEY":          base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{52}, 32)),
		"RECOVERY_RATE_KEY":           base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{53}, 32)),
		"RECOVERY_SMTP_HOST":          "127.0.0.1",
		"RECOVERY_SMTP_PORT":          "1025",
		"RECOVERY_SMTP_FROM":          "noreply@govai.test",
		"RECOVERY_SMTP_DEV_PLAINTEXT": "true",
		"RECOVERY_SMTP_USERNAME":      "",
		"RECOVERY_SMTP_PASSWORD":      "",
		"RECOVERY_LIFETIME":           "15m",
	} {
		t.Setenv(k, v)
	}

	web := Web{
		Enabled:  true,
		Origin:   "https://app.example.test",
		Provider: login.KeycloakConfig{Issuer: reg.Admin.BaseURL + "/realms/" + reg.Admin.Realm},
	}
	return reg, web
}

func TestRecoveryConfiguration(t *testing.T) {
	t.Setenv("RECOVERY_ENABLED", "")
	if c, err := LoadRecovery(Registration{}, Web{}); err != nil || c.Enabled {
		t.Fatal("enabled by default")
	}

	reg, web := recoveryEnv(t)
	c, err := LoadRecovery(reg, web)
	if err != nil || !c.Enabled {
		t.Fatal("valid recovery config rejected")
	}

	raw, _ := json.Marshal(c)
	output := string(raw) + fmt.Sprintf("%+v", c)
	if strings.Contains(output, base64.StdEncoding.EncodeToString(c.FenceKey)) {
		t.Fatal("config redaction failed")
	}

	for _, tc := range []struct{ k, v string }{
		{"RECOVERY_ENABLED", "yes"},
		{"RECOVERY_RESET_URL", "http://app.test/reset"},
		{"RECOVERY_RESET_URL", "https://app.test/reset?token=caller"},
		{"RECOVERY_RESET_URL", "https://app.test/reset#token=caller"},
		{"RECOVERY_BINDING_KEY", "bad"},
		{"RECOVERY_FENCE_KEY", base64.StdEncoding.EncodeToString(c.RateKey)},
		{"RECOVERY_LIFETIME", "31m"},
		{"RECOVERY_SMTP_HOST", "smtp.example.test"},
		{"RECOVERY_SMTP_PORT", "0"},
		{"RECOVERY_SMTP_USERNAME", "unpaired"},
	} {
		t.Run(tc.k+tc.v, func(t *testing.T) {
			t.Setenv(tc.k, tc.v)
			if _, err := LoadRecovery(reg, web); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}

	reg.Enabled = false
	if _, err = LoadRecovery(reg, web); err == nil {
		t.Fatal("missing admin config accepted")
	}

	reg.Enabled = true
	web.Provider.Issuer = "https://other.test/realms/test"
	if _, err = LoadRecovery(reg, web); err == nil {
		t.Fatal("wrong web issuer accepted")
	}
}

func TestRecoveryDisabledRetainsFence(t *testing.T) {
	reg, web := recoveryEnv(t)
	t.Setenv("RECOVERY_ENABLED", "false")
	c, err := LoadRecovery(reg, web)
	if err != nil || c.Enabled || len(c.FenceKey) != 32 {
		t.Fatal("disabled routes removed guard")
	}

	t.Setenv("RECOVERY_FENCE_KEY", "bad")
	if _, err = LoadRecovery(reg, web); err == nil {
		t.Fatal("invalid retained guard key accepted")
	}
}
