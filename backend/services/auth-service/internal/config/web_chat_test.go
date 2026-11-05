package config

import (
	"testing"
	"time"
)

func TestWebChatConfiguration(t *testing.T) {
	for _, flag := range []string{"", "false"} {
		t.Setenv("WEB_CHAT_ENABLED", flag)
		c, e := LoadWebChat(false)
		if e != nil || c.Enabled {
			t.Fatal("enabled by default")
		}
	}

	t.Setenv("WEB_CHAT_ENABLED", "true")
	if _, e := LoadWebChat(false); e == nil {
		t.Fatal("enabled without sessions")
	}

	t.Setenv("WEB_CHAT_TIMEOUT", "31s")
	if _, e := LoadWebChat(true); e == nil {
		t.Fatal("timeout exceeds route budget")
	}

	t.Setenv("WEB_CHAT_TIMEOUT", "")
	t.Setenv("WEB_CHAT_CORE_URL", "https://localhost:5002/api/chat/message")
	c, e := LoadWebChat(true)
	if e != nil || !c.Enabled || c.Timeout != 30*time.Second {
		t.Fatal("valid config rejected")
	}

	t.Setenv("WEB_CHAT_ENABLED", "yes")
	if _, e := LoadWebChat(true); e == nil {
		t.Fatal("unknown enable flag accepted")
	}
}
