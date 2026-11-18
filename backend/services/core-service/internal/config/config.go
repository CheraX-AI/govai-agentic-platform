package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrConfig = errors.New("invalid core chat server configuration")

type Config struct {
	RequestTimeout                                   time.Duration
	MaxConcurrent                                    int // chats answered at the same time (all users)
	Port, Certificate, Key, Issuer, Audience, CAFile string
	Clients, Origins                                 []string
	DatabaseURL                                      string // runtime login for govai_core; required

	// MessageKey encrypts stored message content (AES-256). Never log or return it.
	MessageKey []byte
}

// Load validates core HTTP, TLS and token-verification settings.
func Load() (Config, error) {
	c := Config{
		Port:        os.Getenv("CORE_PORT"),
		Certificate: os.Getenv("CORE_TLS_CERT_FILE"),
		Key:         os.Getenv("CORE_TLS_KEY_FILE"),
		Issuer:      os.Getenv("CHAT_ISSUER_URL"),
		Audience:    os.Getenv("CHAT_AUDIENCE"),
		CAFile:      os.Getenv("CHAT_CA_FILE"),
		DatabaseURL: os.Getenv("CORE_DATABASE_URL"),
	}
	c.RequestTimeout = 30 * time.Second
	if value := os.Getenv("CHAT_REQUEST_TIMEOUT"); value != "" {
		var err error
		c.RequestTimeout, err = time.ParseDuration(value)
		if err != nil || c.RequestTimeout < time.Second || c.RequestTimeout > time.Minute {
			return Config{}, ErrConfig
		}
	}

	c.MaxConcurrent = 8
	if value := os.Getenv("CHAT_MAX_CONCURRENT"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 64 || strconv.Itoa(n) != value {
			return Config{}, ErrConfig
		}

		c.MaxConcurrent = n
	}

	if c.Port == "" {
		c.Port = "5002"
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil ||
		port < 1 ||
		port > 65535 ||
		strconv.Itoa(port) != c.Port ||
		c.Certificate == "" ||
		c.Key == "" ||
		c.Audience == "" ||
		c.DatabaseURL == "" {
		return Config{}, ErrConfig
	}

	// Required: a base64 32-byte key. An all-zero key is a placeholder, not a secret.
	key, err := base64.StdEncoding.Strict().DecodeString(os.Getenv("CORE_CONVERSATION_DATA_KEY"))
	if err != nil || len(key) != 32 || bytes.Equal(key, make([]byte, 32)) {
		return Config{}, ErrConfig
	}

	c.MessageKey = key

	issuer, err := url.Parse(c.Issuer)
	if err != nil ||
		issuer.Scheme != "https" ||
		issuer.Hostname() == "" ||
		issuer.User != nil ||
		issuer.RawQuery != "" ||
		issuer.ForceQuery ||
		issuer.Fragment != "" ||
		strings.HasSuffix(c.Issuer, "/") {
		return Config{}, ErrConfig
	}

	for _, value := range strings.Split(os.Getenv("CHAT_ALLOWED_CLIENTS"), ",") {
		v := strings.TrimSpace(value)
		if v == "" {
			return Config{}, ErrConfig
		}

		c.Clients = append(c.Clients, v)
	}

	if len(c.Clients) > 10 {
		return Config{}, ErrConfig
	}

	if origins := os.Getenv("CHAT_ALLOWED_ORIGINS"); origins != "" {
		for _, value := range strings.Split(origins, ",") {
			origin := strings.TrimSpace(value)
			u, err := url.Parse(origin)
			if err != nil ||
				u.Scheme != "https" ||
				u.Hostname() == "" ||
				u.User != nil ||
				u.Path != "" ||
				u.RawQuery != "" ||
				u.ForceQuery ||
				u.Fragment != "" ||
				u.String() != origin {
				return Config{}, ErrConfig
			}

			c.Origins = append(c.Origins, origin)
		}
	}

	return c, nil
}
