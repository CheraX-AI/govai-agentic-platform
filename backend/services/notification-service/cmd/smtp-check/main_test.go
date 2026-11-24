package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/delivery"
)

// TestSandboxCommand checks opt-in sending and host restriction without network access.

func TestSandboxCommand(t *testing.T) {
	cases := []struct {
		name      string
		host      string
		enabled   string
		args      []string
		wantExit  int
		wantCalls int
		outcome   string
	}{
		{"validate only", sandboxHost, "true", nil, 0, 0, ""},
		{"explicit send", sandboxHost, "true", []string{"-send"}, 0, 1, "submitted"},
		{"live host blocked", "live.smtp.mailtrap.io", "true", []string{"-send"}, 2, 0, ""},
		{"disabled", sandboxHost, "false", []string{"-send"}, 2, 0, ""},
		{"unknown outcome", sandboxHost, "true", []string{"-send"}, 1, 1, "unknown"},
		{"extra arguments", sandboxHost, "true", []string{"unexpected"}, 2, 0, ""},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			values := map[string]string{
				"EMAIL_ENABLED": item.enabled, "SMTP_HOST": item.host,
				"SMTP_PORT": "2525", "SMTP_USERNAME": "synthetic-user",
				"SMTP_PASSWORD": "synthetic-secret", "SMTP_FROM": "noreply@govai.test",
				"SMTP_TLS_MODE": "starttls", "SMTP_TIMEOUT": "2s",
			}
			for key, value := range values {
				t.Setenv(key, value)
			}
			calls := 0
			var output bytes.Buffer
			sender := func(ctx context.Context, settings delivery.Config, message delivery.Message) (
				delivery.Result, error,
			) {
				calls++
				if message.Recipient != "synthetic-citizen@govai.test" || len(message.ID) != 36 {
					t.Fatal("unexpected synthetic test message")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("submission is not bounded")
				}
				if item.outcome == "unknown" {
					return delivery.Result{Status: "unknown", Code: "submission_unknown"},
						errors.New("synthetic-secret provider response")
				}
				return delivery.Result{Status: "submitted", Code: "smtp_accepted"}, nil
			}
			code := run(item.args, &output, sender)
			if code != item.wantExit || calls != item.wantCalls {
				t.Fatal("incorrect command guard or submission count")
			}
			if strings.Contains(output.String(), "synthetic-secret") {
				t.Fatal("credentials or raw provider response leaked")
			}
			if item.outcome == "unknown" &&
				!strings.Contains(output.String(), "Do not resend automatically") {
				t.Fatal("uncertain acceptance was not surfaced")
			}
		})
	}
}
