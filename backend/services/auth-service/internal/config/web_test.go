package config

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestWebConfiguration(t *testing.T) {
	t.Setenv("WEB_SESSION_ENABLED", "")
	if c, e := LoadWeb(); e != nil || c.Enabled {
		t.Fatal("web enabled by default")
	}

	for k, v := range map[string]string{
		"WEB_SESSION_ENABLED":    "true",
		"DATABASE_URL":           "synthetic",
		"API_TLS_CERT_FILE":      "synthetic",
		"API_TLS_KEY_FILE":       "synthetic",
		"WEB_CLIENT_ID":          "web",
		"WEB_CLIENT_SECRET":      "synthetic",
		"WEB_AUDIENCE":           "api",
		"WEB_ISSUER_URL":         "https://identity.test/realms/test",
		"WEB_ORIGIN":             "https://govai.test",
		"WEB_SESSION_DATA_KEY":   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{11}, 32)),
		"WEB_RATE_KEY":           base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{12}, 32)),
		"WEB_SESSION_LIFETIME":   "30m",
		"LOGIN_CLIENT_ID":        "mobile",
		"REGISTRATION_CLIENT_ID": "registration",
	} {
		t.Setenv(k, v)
	}

	if c, e := LoadWeb(); e != nil || !c.Provider.EnableRefresh {
		t.Fatal("valid web config rejected")
	}

	for _, tc := range []struct{ k, v string }{
		{"WEB_ORIGIN", "https://govai.test/"},
		{"WEB_ORIGIN", "http://govai.test"},
		{"WEB_CLIENT_ID", "mobile"},
		{"WEB_SESSION_LIFETIME", "0s"},
		{"WEB_SESSION_DATA_KEY", "invalid"},
		{"PROFILE_DATA_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{11}, 32))},
	} {
		t.Run(tc.k+tc.v, func(t *testing.T) {
			t.Setenv(tc.k, tc.v)
			if _, e := LoadWeb(); e == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}
