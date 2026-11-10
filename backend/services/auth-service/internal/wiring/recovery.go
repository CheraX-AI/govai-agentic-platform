package wiring

import (
	"context"
	"net/http"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Recovery connects account lookup, reset challenges, Keycloak and email.
func Recovery(
	ctx context.Context,
	c config.Recovery,
	reg config.Registration,
	web config.Web,
	pool *pgxpool.Pool,
	profiles *postgres.Store,
) (*httpapi.RecoveryHandler, func(), error) {
	if !c.Enabled ||
		!reg.Enabled ||
		!web.Enabled ||
		profiles == nil ||
		postgres.RecoveryReady(ctx, pool) != nil {
		return nil, nil, config.ErrRecoveryConfig
	}

	if c.Service.Issuer != reg.Admin.BaseURL+"/realms/"+reg.Admin.Realm ||
		c.Service.Issuer != web.Provider.Issuer ||
		c.Origin != web.Origin {
		return nil, nil, config.ErrRecoveryConfig
	}

	transport, err := newKeycloakTransport(reg.CAFile)
	if err != nil {
		return nil, nil, config.ErrRecoveryConfig
	}

	admin, err := keycloak.NewRegistrationAdmin(reg.Admin, &http.Client{Transport: transport})
	transport.CloseIdleConnections()
	if err != nil {
		return nil, nil, config.ErrRecoveryConfig
	}

	fail := func() (*httpapi.RecoveryHandler, func(), error) {
		admin.Close()
		return nil, nil, config.ErrRecoveryConfig
	}
	provider, err := keycloak.NewRecoveryProvider(admin)
	if err != nil {
		return fail()
	}

	accounts, err := postgres.NewRecoveryAccounts(profiles)
	if err != nil {
		return fail()
	}

	raw, err := postgres.NewRecoveryStore(pool)
	if err != nil {
		return fail()
	}

	tokens, err := postgres.NewRecoveryChallenges(raw)
	if err != nil {
		return fail()
	}

	mail, err := recovery.NewSMTPMailer(c.SMTP)
	if err != nil {
		return fail()
	}

	webStore, err := postgres.NewWebSessionStore(pool, web.DataKey)
	if err != nil {
		return fail()
	}

	bound, err := postgres.NewBoundWebSessionStore(webStore, web.Provider.Issuer, web.Provider.ClientID)
	if err != nil {
		return fail()
	}

	sessions, err := postgres.NewRecoveryWebStore(bound, c.FenceKey)
	if err != nil {
		return fail()
	}

	service, err := recovery.NewService(c.Service, accounts, provider, tokens, mail, sessions)
	if err != nil {
		return fail()
	}

	h, err := httpapi.NewLocalRecoveryHandler(
		service,
		c.RateKey,
		httpapi.RecoveryHTTPConfig{Origins: []string{c.Origin}, Timeout: 28 * time.Second},
	)
	if err != nil {
		return fail()
	}

	return h, admin.Close, nil
}
