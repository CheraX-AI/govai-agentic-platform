package application

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"log/slog"
)

// Report only fixed dependency outcomes. Never log err.Error(), subjects,
// markers, URLs, request representations or administrative configuration.
type observedAdmin struct{ registration.Admin }

func observeAdmin(operation string, err error) {
	if err == nil {
		return
	}
	code := "unclassified"
	uncertain := true
	var safe *registration.AdminError
	if errors.As(err, &safe) {
		switch safe.Code {
		case registration.AdminInvalid, registration.AdminDuplicate, registration.AdminPolicyRejected, registration.AdminPermission, registration.AdminUpstream, registration.AdminAmbiguous, registration.AdminNotFound, registration.AdminOwnership, registration.AdminCanceled:
			code = string(safe.Code)
			uncertain = safe.UnknownOutcome
		}
	}
	slog.Warn("registration identity dependency failed", "operation", operation, "code", code, "uncertain", uncertain)
}
func (a observedAdmin) CreateDisabled(ctx context.Context, r registration.Request, op string) (registration.Identity, error) {
	u, e := a.Admin.CreateDisabled(ctx, r, op)
	observeAdmin("create", e)
	return u, e
}
func (a observedAdmin) ReadOwned(ctx context.Context, subject, op string) (registration.Identity, error) {
	u, e := a.Admin.ReadOwned(ctx, subject, op)
	observeAdmin("read", e)
	return u, e
}
func (a observedAdmin) FindOwned(ctx context.Context, op string) (registration.Identity, error) {
	u, e := a.Admin.FindOwned(ctx, op)
	observeAdmin("find", e)
	return u, e
}
func (a observedAdmin) EnableOwned(ctx context.Context, subject, op string) error {
	e := a.Admin.EnableOwned(ctx, subject, op)
	observeAdmin("enable", e)
	return e
}
func (a observedAdmin) SendVerification(ctx context.Context, subject, op string) error {
	e := a.Admin.SendVerification(ctx, subject, op)
	observeAdmin("verification_email", e)
	return e
}
