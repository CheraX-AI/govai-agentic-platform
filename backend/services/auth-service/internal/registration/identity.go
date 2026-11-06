// Package registration defines registration validation and persistence boundaries.
// It does not implement identity verification or expose an HTTP endpoint.
package registration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

var (
	ErrNICFormat   = errors.New("invalid NIC format")
	ErrNICConflict = errors.New("NIC number already reserved")
)

// NIC retains the supplied value privately. Syntax is not authenticity,
// ownership, checksum validation or old/new equivalence.
type NIC struct{ original string }

// ParseNIC accepts ASCII digits with an optional old-format V/X suffix.
// Only suffix case is normalized for lookup; no digits are inserted or removed.
// Whitespace, separators and Unicode lookalikes are rejected rather than repaired.
func ParseNIC(input string) (NIC, error) {
	digits := len(input)
	if digits == 10 {
		switch input[9] {
		case 'v', 'V', 'x', 'X':
		default:
			return NIC{}, ErrNICFormat
		}
		digits = 9
	} else if digits != 12 {
		return NIC{}, ErrNICFormat
	}
	for i := 0; i < digits; i++ {
		if input[i] < '0' || input[i] > '9' {
			return NIC{}, ErrNICFormat
		}
	}
	return NIC{original: input}, nil
}

// NormalizedNumber is sensitive: only registration/storage adapters may use it.
// It must never enter logs, tokens, metrics, queues or public responses.
// It identifies a self-declared number, not a verified person.
func (n NIC) NormalizedNumber() string {
	if len(n.original) == 10 {
		last := n.original[9]
		if last == 'v' || last == 'x' {
			last -= 'a' - 'A'
		}
		return n.original[:9] + string(last)
	}
	return n.original
}

// RestrictedOriginal is for an access-restricted, encrypted profile adapter only.
func (n NIC) RestrictedOriginal() string { return n.original }
func (NIC) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (NIC) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (NIC) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

// NICReservations is the database teammate's required persistence contract.
// Reserve must atomically claim NIC.NormalizedNumber() in the GovAI citizen
// namespace AND durably reserve its registration journal operation, before any
// Keycloak create. Same number+operation is idempotent; a different operation gets
// ErrNICConflict. Each operation may reserve only one number. Claims survive
// restarts and must not expire while an
// external identity outcome is uncertain. No check-then-insert implementation.
// Bind atomically attaches the reserved operation to a unique issuer+subject and
// profile/journal completion. Conflicting bindings fail without overwriting.
// Adapters must enforce database constraints, not rely on process-local locks.
// Both methods reject zero NIC values with ErrNICFormat, honor cancellation, and
// return sanitized errors. Neither operation verifies identity or maps old/new
// numbers. Different normalized numbers remain distinct, even for the same person.
type NICReservations interface {
	Reserve(ctx context.Context, nic NIC, operationID string) error
	Bind(ctx context.Context, nic NIC, operationID, issuer, subject string) error
}
