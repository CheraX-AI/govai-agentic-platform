package config

import (
	"errors"
	"os"
	"strconv"
)

// RateLimits are per-process request limits that can be tuned at deploy time.
// A zero field means "use the built-in default" (see wiring/limits.go).
type RateLimits struct {
	Login             int // password attempts per IP and per account in 15 minutes (default 5)
	Refresh           int // refresh, session and logout calls per IP per minute (default 60)
	RegistrationIP    int // sign-ups per IP in 15 minutes (default 5)
	RegistrationEmail int // sign-ups per email address per hour (default 3)
}

var ErrRateLimits = errors.New("invalid rate limit settings")

// LoadRateLimits reads the optional *_RATE_LIMIT settings. Unset keeps the default.
func LoadRateLimits() (RateLimits, error) {
	var c RateLimits
	for _, setting := range []struct {
		key    string
		max    int
		target *int
	}{
		{"LOGIN_RATE_LIMIT", 1000, &c.Login},
		{"REFRESH_RATE_LIMIT", 10000, &c.Refresh},
		{"REGISTRATION_RATE_LIMIT_IP", 1000, &c.RegistrationIP},
		{"REGISTRATION_RATE_LIMIT_EMAIL", 100, &c.RegistrationEmail},
	} {
		value := os.Getenv(setting.key)
		if value == "" {
			continue
		}

		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > setting.max || strconv.Itoa(n) != value {
			return RateLimits{}, ErrRateLimits
		}

		*setting.target = n
	}

	return c, nil
}
