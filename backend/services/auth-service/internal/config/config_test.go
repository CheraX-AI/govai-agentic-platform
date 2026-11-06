package config

import (
	"os"
	"testing"
)

func TestLoadDefault(t *testing.T) {
	original, exists := os.LookupEnv("PORT")
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv("PORT", original)
		} else {
			_ = os.Unsetenv("PORT")
		}
	})
	cfg, err := Load()
	if err != nil || cfg.Port != "5000" {
		t.Fatalf("Load() = %+v, %v; want port 5000", cfg, err)
	}
}

func TestLoadPort(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		valid bool
	}{
		{"custom", "8080", true},
		{"minimum", "1", true},
		{"maximum", "65535", true},
		{"empty", "", false},
		{"text", "hello", false},
		{"zero", "0", false},
		{"negative", "-1", false},
		{"too large", "65536", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PORT", tc.value)
			cfg, err := Load()
			if (err == nil) != tc.valid {
				t.Fatalf("Load() error = %v, valid = %v", err, tc.valid)
			}
			if tc.valid && cfg.Port != tc.value {
				t.Fatalf("port = %q, want %q", cfg.Port, tc.value)
			}
		})
	}
}

func TestCORSConfig(t *testing.T) {
	for _, value := range []string{"*", "null", "https://example.com/path", "https://user@example.com", "https://example.com?x=1", "https://example.com,"} {
		t.Setenv("CORS_ALLOWED_ORIGINS", value)
		if _, err := Load(); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com,http://localhost:5173")
	cfg, err := Load()
	if err != nil || len(cfg.CORSOrigins) != 2 {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	cfg, err = Load()
	if err != nil || len(cfg.CORSOrigins) != 0 {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
}

func TestOIDCConfiguration(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CHECK_TIMEOUT", "5s")
	cfg, err := Load()
	if err != nil || cfg.OIDCIssuerURL != "" || cfg.OIDCCheckTimeout.String() != "5s" {
		t.Fatalf("optional config: %v", err)
	}
	for _, issuer := range []string{"http://localhost:8180/realms/GovAI", "https://id.example.test/realms/GovAI", "http://127.0.0.1:8180/realms/GovAI"} {
		t.Setenv("OIDC_ISSUER_URL", issuer)
		cfg, err := Load()
		if err != nil || cfg.OIDCIssuerURL != issuer {
			t.Fatalf("issuer config: %v", err)
		}
	}
	for _, issuer := range []string{"http://id.example.test", "https://user:secret@example.test", "https://example.test?secret=1", "relative"} {
		t.Setenv("OIDC_ISSUER_URL", issuer)
		if _, err := Load(); err == nil {
			t.Fatal("invalid issuer accepted")
		}
	}
	t.Setenv("OIDC_ISSUER_URL", "")
	for _, timeout := range []string{"", "oops", "0s", "-1s", "31s"} {
		t.Setenv("OIDC_CHECK_TIMEOUT", timeout)
		if _, err := Load(); err == nil {
			t.Fatal("invalid timeout accepted")
		}
	}
}
