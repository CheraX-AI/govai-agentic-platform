package config

import "testing"

// TestSMTPSettings checks disabled defaults and explicit secure configuration.

func TestSMTPSettings(t *testing.T) {
	base := map[string]string{
		"EMAIL_ENABLED":    "true",
		"SMTP_HOST":        "smtp.example",
		"SMTP_PORT":        "587",
		"SMTP_USERNAME":    "synthetic",
		"SMTP_PASSWORD":    "synthetic-secret",
		"SMTP_FROM":        "noreply@govai.test",
		"SMTP_TLS_MODE":    "starttls",
		"SMTP_TIMEOUT":     "10s",
		"NOTIFICATION_ENV": "production",
	}
	cases := []struct {
		name    string
		changes map[string]string
		enabled bool
		valid   bool
	}{
		{"secure settings", nil, true, true},
		{"disabled", map[string]string{
			"EMAIL_ENABLED": "false", "SMTP_HOST": "", "SMTP_PASSWORD": "",
		}, false, true},
		{"unset", map[string]string{"EMAIL_ENABLED": ""}, false, true},
		{"invalid flag", map[string]string{"EMAIL_ENABLED": "yes"}, false, false},
		{"invalid port", map[string]string{"SMTP_PORT": "bad"}, false, false},
		{"invalid timeout", map[string]string{"SMTP_TIMEOUT": "forever"}, false, false},
		{"missing sender", map[string]string{"SMTP_FROM": ""}, false, false},
		{"remote plaintext", map[string]string{"SMTP_TLS_MODE": "plain"}, false, false},
		{"partial credentials", map[string]string{"SMTP_USERNAME": ""}, false, false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			for name, value := range base {
				t.Setenv(name, value)
			}
			for name, value := range item.changes {
				t.Setenv(name, value)
			}
			_, enabled, err := LoadSMTP()
			if (err == nil) != item.valid || enabled != item.enabled {
				t.Fatal("unexpected SMTP settings validation")
			}
		})
	}
}
