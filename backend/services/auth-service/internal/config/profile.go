package config

import (
	"errors"
	"os"
)

type Profile struct{ Enabled bool }

var ErrProfileConfig = errors.New("invalid profile runtime configuration")

func LoadProfile(auth Authorization, web Web) (Profile, error) {
	switch os.Getenv("PROFILE_API_ENABLED") {
	case "", "false":
		return Profile{}, nil
	case "true":
		if os.Getenv("DATABASE_URL") == "" || os.Getenv("API_TLS_CERT_FILE") == "" || os.Getenv("API_TLS_KEY_FILE") == "" || (!auth.Enabled && !web.Enabled) {
			return Profile{}, ErrProfileConfig
		}
		if auth.Enabled && web.Enabled && auth.Issuer != web.Provider.Issuer {
			return Profile{}, ErrProfileConfig
		}
		return Profile{Enabled: true}, nil
	default:
		return Profile{}, ErrProfileConfig
	}
}
