package notificationevents

import (
	"fmt"
	"io"
	"log/slog"
)

// A job contains an internal lease capability; keep it out of diagnostics.

func (Job) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED outbox job]")
}

func (Job) LogValue() slog.Value {
	return slog.StringValue("[REDACTED outbox job]")
}

func (Job) MarshalJSON() ([]byte, error) {
	return []byte("\"[REDACTED outbox job]\""), nil
}
