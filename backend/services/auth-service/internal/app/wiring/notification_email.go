package wiring

import (
	"context"
	"net/http"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reuse the existing ownership-checked admin adapter, with no extra realm grants.

func NotificationEmailObserver(
	ctx context.Context, cfg config.Registration, pool *pgxpool.Pool, issuer string,
) (*notificationevents.EmailObserver, func(), error) {
	if !cfg.Enabled || issuer != cfg.Admin.BaseURL+"/realms/"+cfg.Admin.Realm {
		return nil, nil, notificationevents.ErrConfiguration
	}
	transport, err := newKeycloakTransport(cfg.CAFile)
	if err != nil {
		return nil, nil, notificationevents.ErrConfiguration
	}
	admin, err := keycloak.NewRegistrationAdmin(cfg.Admin, &http.Client{Transport: transport})
	transport.CloseIdleConnections()
	if err != nil {
		return nil, nil, notificationevents.ErrConfiguration
	}
	store, err := postgres.NewEmailObservationStore(pool, issuer)
	if err != nil || store.Initialize(ctx) != nil {
		admin.Close()
		return nil, nil, notificationevents.ErrConfiguration
	}
	observer, err := notificationevents.NewEmailObserver(store, admin)
	if err != nil {
		admin.Close()
		return nil, nil, err
	}
	return observer, admin.Close, nil
}
