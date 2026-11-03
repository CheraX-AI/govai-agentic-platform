package application

import (
	"context"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/websession"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func Web(ctx context.Context, c config.Web, pool *pgxpool.Pool, profiles *postgres.Store, fenceKeys ...[]byte) (*handler.WebSessionHandler, func(), error) {
	if !c.Enabled || profiles == nil || !c.Provider.EnableRefresh || postgres.WebSessionsReady(ctx, pool) != nil {
		return nil, nil, websession.ErrUnavailable
	}
	provider, err := webProvider(c)
	if err != nil {
		return nil, nil, websession.ErrUnavailable
	}
	fail := func() (*handler.WebSessionHandler, func(), error) {
		provider.Close()
		return nil, nil, websession.ErrUnavailable
	}
	l, err := login.NewService(provider, provider, profiles)
	if err != nil {
		return fail()
	}
	rotation, err := login.NewSessionService(provider, provider, profiles)
	if err != nil {
		return fail()
	}
	store, err := postgres.NewWebSessionStore(pool, c.DataKey)
	if err != nil {
		return fail()
	}
	bound, err := postgres.NewBoundWebSessionStore(store, c.Provider.Issuer, c.Provider.ClientID)
	if err != nil {
		return fail()
	}
	var sessionStore websession.Store = bound
	if len(fenceKeys) == 0 {
		var installed bool
		if pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM public.govai_auth_migrations WHERE version=6)").Scan(&installed) != nil || installed {
			return fail()
		}
	}
	if len(fenceKeys) > 1 {
		return fail()
	}
	if len(fenceKeys) == 1 {
		if postgres.RecoveryReady(ctx, pool) != nil {
			return fail()
		}
		guarded, err := postgres.NewRecoveryWebStore(bound, fenceKeys[0])
		if err != nil {
			return fail()
		}
		sessionStore = guarded
	}
	service, err := websession.New(l, rotation, provider, profiles, sessionStore, c.Lifetime)
	if err != nil {
		return fail()
	}
	limiter, err := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: c.RateKey, IPAttempts: 60, EmailAttempts: 1, IPWindow: time.Minute, EmailWindow: time.Minute, MaxKeys: 4096, MaxConcurrent: 8})
	if err != nil {
		return fail()
	}
	h, err := handler.NewWebSessionHandler(service, limiter, c.Origin)
	if err != nil {
		return fail()
	}
	loginLimiter, err := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: c.RateKey, IPAttempts: 5, EmailAttempts: 5, IPWindow: 15 * time.Minute, EmailWindow: 15 * time.Minute, MaxKeys: 4096, MaxConcurrent: 8})
	if err != nil {
		return fail()
	}
	h.SetLoginLimiter(loginLimiter)
	return h, provider.Close, nil
}
