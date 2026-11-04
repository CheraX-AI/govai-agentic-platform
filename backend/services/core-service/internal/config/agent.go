package config

import (
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/chat"
	"os"
	"time"
)

type Agent struct {
	Enabled bool
	HTTP    chat.HTTPConfig
	CAFile  string
}

func LoadAgent(requestTimeout time.Duration) (Agent, error) {
	var c Agent
	switch os.Getenv("KNOWLEDGE_AGENT_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, ErrConfig
	}
	timeout := 20 * time.Second
	if value := os.Getenv("KNOWLEDGE_AGENT_TIMEOUT"); value != "" {
		var err error
		timeout, err = time.ParseDuration(value)
		if err != nil {
			return Agent{}, ErrConfig
		}
	}
	if timeout < time.Second || timeout >= requestTimeout {
		return Agent{}, ErrConfig
	}
	allowHTTP := false
	switch os.Getenv("KNOWLEDGE_AGENT_ALLOW_LOOPBACK_HTTP") {
	case "", "false":
	case "true":
		allowHTTP = true
	default:
		return Agent{}, ErrConfig
	}
	c.HTTP = chat.HTTPConfig{URL: os.Getenv("KNOWLEDGE_AGENT_URL"), AuthMode: os.Getenv("KNOWLEDGE_AGENT_AUTH_MODE"), BearerToken: os.Getenv("KNOWLEDGE_AGENT_BEARER_TOKEN"), Timeout: timeout, AllowLoopbackHTTP: allowHTTP}
	c.CAFile = os.Getenv("KNOWLEDGE_AGENT_CA_FILE")
	if chat.ValidateHTTPConfig(c.HTTP) != nil {
		return Agent{}, ErrConfig
	}
	return c, nil
}
