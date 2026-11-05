package config

import (
	"errors"
	"os"
	"strings"
)

type Authorization struct {
	Enabled                  bool
	Issuer, Audience, CAFile string
	Clients                  []string
}

var ErrAuthorizationConfig = errors.New("invalid authorization runtime configuration")

// LoadAuthorization validates the trusted issuer, audience and clients.
func LoadAuthorization() (Authorization, error) {
	var c Authorization
	switch os.Getenv("API_AUTH_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, ErrAuthorizationConfig
	}

	if os.Getenv("DATABASE_URL") == "" ||
		os.Getenv("API_TLS_CERT_FILE") == "" ||
		os.Getenv("API_TLS_KEY_FILE") == "" {
		return Authorization{}, ErrAuthorizationConfig
	}

	c.Issuer = os.Getenv("API_AUTH_ISSUER")
	c.Audience = os.Getenv("API_AUTH_AUDIENCE")
	c.CAFile = os.Getenv("API_AUTH_CA_FILE")
	clients := os.Getenv("API_AUTH_CLIENT_IDS")
	for _, raw := range strings.Split(clients, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			return Authorization{}, ErrAuthorizationConfig
		}

		c.Clients = append(c.Clients, id)
	}

	if c.Issuer == "" || c.Audience == "" {
		return Authorization{}, ErrAuthorizationConfig
	}

	return c, nil
}
