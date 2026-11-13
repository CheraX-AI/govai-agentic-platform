package config

import "testing"

func TestLoadRateLimits(t *testing.T) {
	c, err := LoadRateLimits()
	if err != nil || c != (RateLimits{}) {
		t.Fatal("unset limits must keep the defaults (zero)")
	}

	t.Setenv("LOGIN_RATE_LIMIT", "30")
	t.Setenv("REFRESH_RATE_LIMIT", "120")
	t.Setenv("REGISTRATION_RATE_LIMIT_IP", "50")
	t.Setenv("REGISTRATION_RATE_LIMIT_EMAIL", "5")
	c, err = LoadRateLimits()
	if err != nil || c != (RateLimits{Login: 30, Refresh: 120, RegistrationIP: 50, RegistrationEmail: 5}) {
		t.Fatal("configured limits not loaded")
	}

	for _, tc := range []struct{ key, value string }{
		{"LOGIN_RATE_LIMIT", "0"},
		{"LOGIN_RATE_LIMIT", "1001"},
		{"LOGIN_RATE_LIMIT", "05"},
		{"REFRESH_RATE_LIMIT", "-1"},
		{"REGISTRATION_RATE_LIMIT_IP", "many"},
		{"REGISTRATION_RATE_LIMIT_EMAIL", "101"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := LoadRateLimits(); err == nil {
				t.Fatal("invalid limit accepted")
			}
		})
	}
}
