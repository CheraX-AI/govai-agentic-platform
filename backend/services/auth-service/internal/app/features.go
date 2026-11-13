package app

import (
	"context"
	"log/slog"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/app/wiring"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

// features holds the HTTP handler of each enabled feature; nil means disabled.
type features struct {
	registration *handler.RegistrationHandler
	login        *handler.LoginHandler
	sessions     *handler.NativeSessionHandler
	protected    *handler.Protected
	web          *handler.WebSessionHandler
	recovery     *handler.RecoveryHandler
}

// buildFeatures builds each enabled feature with its dependencies.
func (a *App) buildFeatures(ctx context.Context, cfg Config, pool *pgxpool.Pool, store *postgres.Store) (features, error) {
	var f features

	identifiers, err := a.nicLogin(cfg, store)
	if err != nil {
		return f, err
	}

	if cfg.Registration.Enabled {
		h, closeRegistration, err := wiring.Registration(cfg.Registration, store)
		if err != nil {
			return f, err
		}

		a.onClose(closeRegistration)
		f.registration = h
		slog.Info("registration route enabled with HTTPS")
	}

	if cfg.Login.Enabled {
		h, sessions, closeLogin, err := wiring.LoginWithSessions(cfg.Login, store, identifiers)
		if err != nil {
			return f, err
		}

		a.onClose(closeLogin)
		f.login, f.sessions = h, sessions
		slog.Info("mobile login route enabled with HTTPS")
	}

	if cfg.Authorization.Enabled {
		protected, closeAuthorization, err := wiring.Authorization(cfg.Authorization, store)
		if err != nil {
			return f, err
		}

		a.onClose(closeAuthorization)
		f.protected = protected
		slog.Info("JWT authorization routes enabled with HTTPS")
	}

	if cfg.Web.Enabled {
		var fenceKeys [][]byte
		if len(cfg.Recovery.FenceKey) > 0 {
			fenceKeys = append(fenceKeys, cfg.Recovery.FenceKey)
		}

		web, closeWeb, err := wiring.WebWithIdentifiers(ctx, cfg.Web, pool, store, identifiers, fenceKeys...)
		if err != nil {
			return f, err
		}

		a.onClose(closeWeb)
		f.web = web
		slog.Info("web session routes enabled with HTTPS")
	}

	closeChat, err := wiring.WebChat(cfg.WebChat, f.web)
	if err != nil {
		return f, err
	}

	a.onClose(closeChat)

	if cfg.Recovery.Enabled {
		recovery, closeRecovery, err := wiring.Recovery(ctx, cfg.Recovery, cfg.Registration, cfg.Web, pool, store)
		if err != nil {
			return f, err
		}

		a.onClose(closeRecovery)
		f.recovery = recovery
		slog.Info("password recovery routes enabled with HTTPS and guarded web sessions")
	}

	return f, nil
}

// nicLogin lets users log in with their NIC when registration and a login feature are on.
// Every feature must use the same Keycloak realm, or startup stops.
func (a *App) nicLogin(cfg Config, store *postgres.Store) (login.IdentifierResolver, error) {
	if !cfg.Registration.Enabled || (!cfg.Login.Enabled && !cfg.Web.Enabled) {
		return nil, nil
	}

	expectedIssuer := cfg.Registration.Admin.BaseURL + "/realms/" + cfg.Registration.Admin.Realm
	if (cfg.Login.Enabled && cfg.Login.Provider.Issuer != expectedIssuer) ||
		(cfg.Web.Enabled && cfg.Web.Provider.Issuer != expectedIssuer) {
		return nil, login.ErrUnavailable
	}

	resolver, closeIdentifiers, err := wiring.NICLogin(cfg.Registration, store)
	if err != nil {
		return nil, err
	}

	a.onClose(closeIdentifiers)
	return resolver, nil
}
