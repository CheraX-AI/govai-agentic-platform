package config

import "testing"

func TestProfileConfiguration(t *testing.T) {
	t.Setenv("PROFILE_API_ENABLED", "")
	if cfg, err := LoadProfile(Authorization{}, Web{}); err != nil || cfg.Enabled {
		t.Fatal("profile not optional")
	}
	t.Setenv("PROFILE_API_ENABLED", "true")
	for _, name := range []string{"DATABASE_URL", "API_TLS_CERT_FILE", "API_TLS_KEY_FILE"} {
		t.Setenv(name, "synthetic")
	}
	if _, err := LoadProfile(Authorization{}, Web{}); err == nil {
		t.Fatal("unauthenticated profile enabled")
	}
	if cfg, err := LoadProfile(Authorization{Enabled: true}, Web{}); err != nil || !cfg.Enabled {
		t.Fatal("JWT profile rejected")
	}
	if cfg, err := LoadProfile(Authorization{}, Web{Enabled: true}); err != nil || !cfg.Enabled {
		t.Fatal("web profile rejected")
	}
	web := Web{Enabled: true}
	web.Provider.Issuer = "https://other.test/realms/govai"
	if _, err := LoadProfile(Authorization{Enabled: true, Issuer: "https://issuer.test/realms/govai"}, web); err == nil {
		t.Fatal("web/native issuer mismatch accepted")
	}
	t.Setenv("DATABASE_URL", "")
	if _, err := LoadProfile(Authorization{Enabled: true}, Web{}); err == nil {
		t.Fatal("missing database accepted")
	}
	t.Setenv("PROFILE_API_ENABLED", "yes")
	if _, err := LoadProfile(Authorization{}, Web{}); err == nil {
		t.Fatal("invalid flag accepted")
	}
}
