package config

import "testing"

func TestConfig(t *testing.T) {
	for key, value := range map[string]string{
		"CORE_PORT":            "5002",
		"CORE_TLS_CERT_FILE":   "synthetic-cert",
		"CORE_TLS_KEY_FILE":    "synthetic-key",
		"CHAT_ISSUER_URL":      "https://identity.test/realms/GovAI",
		"CHAT_AUDIENCE":        "govai-api",
		"CHAT_ALLOWED_CLIENTS": "mobile,web",
		"CHAT_ALLOWED_ORIGINS": "https://web.test",
	} {
		t.Setenv(key, value)
	}

	if c, err := Load(); err != nil || c.Port != "5002" || len(c.Clients) != 2 || c.MaxConcurrent != 8 {
		t.Fatal("valid config rejected")
	}

	t.Run("CHAT_MAX_CONCURRENT set", func(t *testing.T) {
		t.Setenv("CHAT_MAX_CONCURRENT", "20")
		if c, err := Load(); err != nil || c.MaxConcurrent != 20 {
			t.Fatal("concurrency setting ignored")
		}
	})

	for _, tc := range []struct{ key, value string }{
		{"CHAT_MAX_CONCURRENT", "0"},
		{"CHAT_MAX_CONCURRENT", "65"},
		{"CHAT_MAX_CONCURRENT", "08"},
		{"CHAT_MAX_CONCURRENT", "many"},
		{"CORE_PORT", "0"},
		{"CORE_PORT", "05002"},
		{"CORE_TLS_KEY_FILE", ""},
		{"CHAT_ISSUER_URL", "http://identity.test"},
		{"CHAT_ALLOWED_CLIENTS", ""},
		{"CHAT_ALLOWED_ORIGINS", "*"},
		{"CHAT_ALLOWED_ORIGINS", "https://web.test/"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}
