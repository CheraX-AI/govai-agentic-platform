package wiring

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func TestDependencyLoggingRedactsUnknownErrors(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	observeAdmin("create", errors.New("private-password-token-body"))
	observeAdmin(
		"read",
		&registration.AdminError{Code: registration.AdminPermission, UnknownOutcome: false},
	)
	observeAdmin("read", nil)
	if strings.Contains(output.String(), "private-password-token-body") ||
		!strings.Contains(output.String(), "unclassified") ||
		!strings.Contains(output.String(), "admin_permission_denied") ||
		strings.Count(output.String(), "registration identity dependency failed") != 2 {
		t.Fatal("dependency diagnostics missing or unsafe")
	}
}
