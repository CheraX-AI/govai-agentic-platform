package wiring

import (
	"context"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Profile(ctx context.Context, pool *pgxpool.Pool, store *postgres.Store, native *handler.Protected, web *handler.WebSessionHandler) error {
	if store == nil || (native == nil && web == nil) || postgres.CitizenDetailsReady(ctx, pool) != nil {
		return profile.ErrUnavailable
	}
	service, err := profile.NewService(store, profile.CitizenFieldRules{})
	if err != nil {
		return err
	}
	if native != nil {
		if err = native.EnableProfile(service); err != nil {
			return err
		}
	}
	if web != nil {
		if err = web.EnableProfile(service); err != nil {
			return err
		}
	}
	return nil
}
