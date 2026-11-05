package config

import "testing"

func TestAuthorizationConfiguration(t *testing.T) {
	t.Setenv("API_AUTH_ENABLED", "")
	if c, e := LoadAuthorization(); e != nil || c.Enabled {
		t.Fatal("authorization enabled by default")
	}

	for k, v := range map[string]string{
		"API_AUTH_ENABLED":    "true",
		"DATABASE_URL":        "local",
		"API_TLS_CERT_FILE":   "cert",
		"API_TLS_KEY_FILE":    "key",
		"API_AUTH_ISSUER":     "https://identity.example.test/realms/test",
		"API_AUTH_AUDIENCE":   "govai-api",
		"API_AUTH_CLIENT_IDS": "mobile,web",
	} {
		t.Setenv(k, v)
	}

	c, e := LoadAuthorization()
	if e != nil || len(c.Clients) != 2 {
		t.Fatal("valid authorization configuration rejected")
	}

	t.Setenv("API_AUTH_CLIENT_IDS", "mobile,")
	if _, e := LoadAuthorization(); e == nil {
		t.Fatal("empty allowlist entry accepted")
	}
}
