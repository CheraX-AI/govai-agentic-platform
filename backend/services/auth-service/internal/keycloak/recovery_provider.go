package keycloak

import (
	"context"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
)

// RecoveryProvider fixes issuer to the configured realm. Public callers cannot
// select a realm, subject or operation; the orchestrator supplies saved bindings.
type RecoveryProvider struct {
	admin  *Admin
	issuer string
}

// NewRecoveryProvider adapts the admin client for password recovery.
func NewRecoveryProvider(admin *Admin) (*RecoveryProvider, error) {
	if admin == nil {
		return nil, recovery.ErrUnavailable
	}

	return &RecoveryProvider{admin: admin, issuer: admin.cfg.BaseURL + "/realms/" + admin.cfg.Realm}, nil
}

var _ recovery.Provider = (*RecoveryProvider)(nil)

func (p *RecoveryProvider) Recipient(ctx context.Context, b recovery.Binding) (string, error) {
	if b.Issuer != p.issuer {
		return "", recovery.ErrUnavailable
	}

	r, err := p.admin.RecoveryRecipient(ctx, b.Subject, b.OperationID)
	if err != nil {
		return "", recovery.ErrUnavailable
	}

	return r.Email(), nil
}

func (p *RecoveryProvider) Reset(ctx context.Context, b recovery.Binding, password recovery.Password) error {
	if b.Issuer != p.issuer {
		return recovery.ErrUnavailable
	}

	if p.admin.ResetOwnedPassword(ctx, b.Subject, b.OperationID, password) != nil {
		return recovery.ErrUnavailable
	}

	return nil
}

func (p *RecoveryProvider) Logout(ctx context.Context, b recovery.Binding) error {
	if b.Issuer != p.issuer {
		return recovery.ErrUnavailable
	}

	if p.admin.LogoutOwnedSessions(ctx, b.Subject, b.OperationID) != nil {
		return recovery.ErrUnavailable
	}

	return nil
}
