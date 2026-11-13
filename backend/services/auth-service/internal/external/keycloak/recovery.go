package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

// RecoveryRecipient may be revealed only to the trusted mail adapter. Never
// serialize it in the public masked-email response or accept it from a caller.
type RecoveryRecipient struct{ email string }

func (r RecoveryRecipient) Email() string { return r.email }

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (RecoveryRecipient) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (RecoveryRecipient) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (RecoveryRecipient) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

// Recovery uses persisted subject/operation bindings, not email/NIC user searches.
// This reuses accepted group-wide credentials but exposes no route or new grant.
func (a *Admin) RecoveryRecipient(ctx context.Context, subject, operation string) (RecoveryRecipient, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	if !segment.MatchString(subject) || !segment.MatchString(operation) {
		return RecoveryRecipient{}, failure(registration.AdminInvalid, false)
	}

	body, _, err := a.call(ctx, http.MethodGet, a.users+"/"+subject, nil, http.StatusOK, false)
	if err != nil {
		return RecoveryRecipient{}, err
	}

	var user struct {
		userRepresentation
		Email string `json:"email"`
	}
	if json.Unmarshal(body, &user) != nil {
		return RecoveryRecipient{}, failure(registration.AdminUpstream, false)
	}

	if user.ID != subject || !owned(user.userRepresentation, operation) {
		return RecoveryRecipient{}, failure(registration.AdminOwnership, false)
	}

	if !user.Enabled || !user.EmailVerified {
		return RecoveryRecipient{}, failure(registration.AdminInvalid, false)
	}

	address, err := mail.ParseAddress(user.Email)
	if err != nil ||
		address.Address != user.Email ||
		len(user.Email) > 254 ||
		strings.ContainsAny(user.Email, "\r\n") {
		return RecoveryRecipient{}, failure(registration.AdminUpstream, false)
	}

	return RecoveryRecipient{email: user.Email}, nil
}

// ResetOwnedPassword is only for an orchestrator that has irreversibly claimed
// a valid recovery token and checked the persisted identity binding. Do not retry
// this mutation after an unknown outcome, and never persist its password argument.
func (a *Admin) ResetOwnedPassword(ctx context.Context, subject, operation string, password recovery.Password) error {
	if _, err := recovery.ParsePassword(password.Reveal()); err != nil {
		return failure(registration.AdminInvalid, false)
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	if _, err := a.RecoveryRecipient(ctx, subject, operation); err != nil {
		return err
	}

	body, err := json.Marshal(struct {
		Type      string `json:"type"`
		Value     string `json:"value"`
		Temporary bool   `json:"temporary"`
	}{"password", password.Reveal(), false})
	if err != nil {
		return failure(registration.AdminInvalid, false)
	}

	_, _, err = a.call(
		ctx,
		http.MethodPut,
		a.users+"/"+subject+"/reset-password",
		body,
		http.StatusNoContent,
		false,
	)
	return err
}

// LogoutOwnedSessions is separate from password mutation. Orchestration must
// track failures and also revoke Go-owned browser sessions; offline JWTs can
// remain valid until expiry. No automatic retry is performed here.
func (a *Admin) LogoutOwnedSessions(ctx context.Context, subject, operation string) error {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	if _, err := a.RecoveryRecipient(ctx, subject, operation); err != nil {
		return err
	}

	_, _, err := a.call(ctx, http.MethodPost, a.users+"/"+subject+"/logout", nil, http.StatusNoContent, false)
	return err
}
