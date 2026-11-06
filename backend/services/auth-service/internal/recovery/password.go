package recovery

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// Password never crosses persistence, logging or job boundaries. Local checks
// only bound input; Keycloak remains authoritative for password policy.
type Password struct{ value string }

func ParsePassword(value string) (Password, error) {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) < 8 || len(value) > 1024 || strings.ContainsRune(value, 0) {
		return Password{}, errors.New("invalid new password")
	}
	return Password{value}, nil
}
func (p Password) Reveal() string             { return p.value }
func (Password) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (Password) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (Password) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }
