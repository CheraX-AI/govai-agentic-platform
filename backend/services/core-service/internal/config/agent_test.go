package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAgentConfig(t *testing.T) {
	t.Setenv("KNOWLEDGE_AGENT_ENABLED", "")
	if c, err := LoadAgent(30 * time.Second); err != nil || c.Enabled {
		t.Fatal("enabled by default")
	}
	for key, value := range map[string]string{"KNOWLEDGE_AGENT_ENABLED": "true", "KNOWLEDGE_AGENT_URL": "https://agent.example.test/chat", "KNOWLEDGE_AGENT_AUTH_MODE": "bearer", "KNOWLEDGE_AGENT_BEARER_TOKEN": "synthetic-service-secret", "KNOWLEDGE_AGENT_TIMEOUT": "20s", "KNOWLEDGE_AGENT_ALLOW_LOOPBACK_HTTP": "false"} {
		t.Setenv(key, value)
	}
	c, err := LoadAgent(30 * time.Second)
	if err != nil || !c.Enabled {
		t.Fatal("valid agent config rejected")
	}
	if strings.Contains(fmt.Sprintf("%+v", c), "synthetic-service-secret") {
		t.Fatal("secret leaked")
	}
	for _, tc := range []struct{ key, value string }{{"KNOWLEDGE_AGENT_ENABLED", "yes"}, {"KNOWLEDGE_AGENT_AUTH_MODE", ""}, {"KNOWLEDGE_AGENT_BEARER_TOKEN", ""}, {"KNOWLEDGE_AGENT_TIMEOUT", "31s"}, {"KNOWLEDGE_AGENT_TIMEOUT", "0s"}, {"KNOWLEDGE_AGENT_ALLOW_LOOPBACK_HTTP", "yes"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := LoadAgent(30 * time.Second); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}
