package config

import (
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/chatgateway"
	"os"
	"time"
)

type WebChat struct {
	Enabled          bool
	Endpoint, CAFile string
	Timeout          time.Duration
}

func LoadWebChat(webEnabled bool) (WebChat, error) {
	c := WebChat{}
	switch os.Getenv("WEB_CHAT_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, chatgateway.ErrUnavailable
	}
	if !webEnabled {
		return WebChat{}, chatgateway.ErrUnavailable
	}
	c.Endpoint = os.Getenv("WEB_CHAT_CORE_URL")
	c.CAFile = os.Getenv("WEB_CHAT_CA_FILE")
	value := os.Getenv("WEB_CHAT_TIMEOUT")
	if value == "" {
		value = "30s"
	}
	var err error
	c.Timeout, err = time.ParseDuration(value)
	if err != nil || c.Timeout < time.Second || c.Timeout > 30*time.Second {
		return WebChat{}, chatgateway.ErrUnavailable
	}
	return c, nil
}
