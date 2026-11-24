package config

import (
	"net/url"
	"strings"
	"testing"
)

// TestBrokerConfiguration covers URI escaping and plaintext deployment boundaries.

func TestBrokerConfiguration(t *testing.T) {
	base := map[string]string{
		"NOTIFICATION_ENV":               "development",
		"NOTIFICATION_LISTEN_ADDRESS":    "127.0.0.1:5003",
		"NOTIFICATION_SECURITY_PREFETCH": "10",
		"NOTIFICATION_NORMAL_PREFETCH":   "2",
		"RABBITMQ_SCHEME":                "amqp",
		"RABBITMQ_HOST":                  "127.0.0.1",
		"RABBITMQ_PORT":                  "5672",
		"RABBITMQ_VHOST":                 "govai",
		"RABBITMQ_CONSUMER_USER":         "consumer",
		"RABBITMQ_CONSUMER_PASSWORD":     "private+/value:@",
		"RABBITMQ_CA_FILE":               "",
	}
	cases := []struct {
		name    string
		changes map[string]string
		valid   bool
	}{
		{"local development", nil, true},
		{"encrypted remote", map[string]string{
			"RABBITMQ_SCHEME": "amqps", "RABBITMQ_HOST": "broker.example",
		}, true},
		{"plaintext remote", map[string]string{"RABBITMQ_HOST": "broker.example"}, false},
		{"plaintext production", map[string]string{"NOTIFICATION_ENV": "production"}, false},
		{"missing password", map[string]string{"RABBITMQ_CONSUMER_PASSWORD": ""}, false},
		{"bad vhost", map[string]string{"RABBITMQ_VHOST": "unapproved"}, false},
		{"bad host", map[string]string{"RABBITMQ_HOST": "broker/path"}, false},
		{"bad port", map[string]string{"RABBITMQ_PORT": "70000"}, false},
		{"bad scheme", map[string]string{"RABBITMQ_SCHEME": "http"}, false},
		{"bad prefetch", map[string]string{"NOTIFICATION_SECURITY_PREFETCH": "0"}, false},
		{"bad listen address", map[string]string{"NOTIFICATION_LISTEN_ADDRESS": ":bad"}, false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			for key, value := range base {
				t.Setenv(key, value)
			}
			for key, value := range item.changes {
				t.Setenv(key, value)
			}
			settings, err := Load()
			if (err == nil) != item.valid {
				t.Fatal("unexpected validation result")
			}
			if err != nil {
				if strings.Contains(err.Error(), base["RABBITMQ_CONSUMER_PASSWORD"]) {
					t.Fatal("configuration error exposed credentials")
				}
				return
			}
			endpoint, err := url.Parse(settings.BrokerURL)
			if err != nil {
				t.Fatal("invalid generated broker URI")
			}
			password, _ := endpoint.User.Password()
			if password != base["RABBITMQ_CONSUMER_PASSWORD"] {
				t.Fatal("credentials changed during URI encoding")
			}
			if endpoint.Path != "/govai" {
				t.Fatal("unexpected virtual host")
			}
		})
	}
}
