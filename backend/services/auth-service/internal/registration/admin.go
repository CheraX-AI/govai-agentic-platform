package registration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// Admin is registration-only. The caller must durably reserve the operation/NIC
// before CreateDisabled and complete the profile before EnableOwned. No method
// retries mutations. Ambiguous outcomes require durable reconciliation, not replay.
// operationID is server-generated, never a caller-controlled attribute.
type Admin interface {
	CreateDisabled(context.Context, Request, string) (Identity, error)
	FindOwned(context.Context, string) (Identity, error)
	ReadOwned(context.Context, string, string) (Identity, error)
	EnableOwned(context.Context, string, string) error
	SendVerification(context.Context, string, string) error
}

// Identity contains only registration state, not the upstream representation.
// Subject is sensitive; only profile/journal adapters may persist it.
type Identity struct {
	Subject       string
	Enabled       bool
	EmailVerified bool
}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (Identity) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (Identity) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (Identity) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

type AdminCode string

const (
	AdminInvalid   AdminCode = "admin_invalid_input"
	AdminDuplicate AdminCode = "admin_duplicate"
	// PolicyRejected means an upstream 400 validation/policy rejection, not proof
	// of a particular password rule or that no partial creation occurred.
	AdminPolicyRejected AdminCode = "admin_policy_rejected"
	AdminPermission     AdminCode = "admin_permission_denied"
	AdminUpstream       AdminCode = "admin_upstream_failure"
	AdminAmbiguous      AdminCode = "admin_ambiguous_outcome"
	AdminNotFound       AdminCode = "admin_not_found"
	AdminOwnership      AdminCode = "admin_ownership_mismatch"
	AdminCanceled       AdminCode = "admin_canceled"
)

// AdminError deliberately contains no URL, body, identity, secret or underlying
// transport error. UnknownOutcome prevents callers treating a failure as rollback.
// A rejected create may still need reconciliation: live atomicity is unproven.
type AdminError struct {
	Code           AdminCode
	UnknownOutcome bool
}

func (e *AdminError) Error() string { return string(e.Code) }
