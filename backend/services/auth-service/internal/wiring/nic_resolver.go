package wiring

import (
	"context"
	"errors"
	"net/http"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

// nicLogin resolves only persisted, ready profiles in the fixed configured realm.
// It does not mutate identities or expose the resolved address to HTTP callers.
type nicLogin struct {
	profiles *postgres.Store
	admin    *keycloak.Admin
	issuer   string
}

func (r *nicLogin) ResolveNIC(ctx context.Context, nic registration.NIC) (login.NICAccount, error) {
	b, err := r.profiles.RecoveryByNIC(ctx, r.issuer, nic)
	if errors.Is(err, postgres.ErrRecoveryUnavailable) {
		return login.NICAccount{}, login.ErrFailed
	}

	if err != nil {
		return login.NICAccount{}, login.ErrUnavailable
	}

	recipient, err := r.admin.RecoveryRecipient(ctx, b.Subject, b.OperationID)
	if err != nil {
		var upstream *registration.AdminError
		if errors.As(err, &upstream) &&
			(upstream.Code == registration.AdminInvalid ||
				upstream.Code == registration.AdminNotFound ||
				upstream.Code == registration.AdminOwnership) {
			return login.NICAccount{}, login.ErrFailed
		}

		return login.NICAccount{}, login.ErrUnavailable
	}

	return login.NICAccount{Email: recipient.Email(), Issuer: b.Issuer, Subject: b.Subject}, nil
}

func NICLogin(c config.Registration, profiles *postgres.Store) (login.IdentifierResolver, func(), error) {
	if !c.Enabled || profiles == nil {
		return nil, nil, login.ErrUnavailable
	}

	transport, err := newKeycloakTransport(c.CAFile)
	if err != nil {
		return nil, nil, login.ErrUnavailable
	}

	admin, err := keycloak.NewRegistrationAdmin(c.Admin, &http.Client{Transport: transport})
	transport.CloseIdleConnections()
	if err != nil {
		return nil, nil, login.ErrUnavailable
	}

	return &nicLogin{profiles: profiles, admin: admin, issuer: c.Admin.BaseURL + "/realms/" + c.Admin.Realm}, admin.Close, nil
}
