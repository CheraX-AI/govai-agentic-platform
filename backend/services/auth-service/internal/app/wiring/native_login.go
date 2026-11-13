package wiring

import (
	"net/http"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
)

// Login builds the native login handler.
func Login(c config.Login, store *postgres.Store) (*handler.LoginHandler, func(), error) {
	h, _, close, e := LoginWithSessions(c, store)
	return h, close, e
}

// LoginWithSessions connects native login, refresh and revocation handlers.
func LoginWithSessions(c config.Login, store *postgres.Store, resolvers ...login.IdentifierResolver) (*handler.LoginHandler, *handler.NativeSessionHandler, func(), error) {
	if !c.Enabled || store == nil {
		return nil, nil, nil, login.ErrUnavailable
	}

	transport, e := newKeycloakTransport(c.CAFile)
	if e != nil {
		return nil, nil, nil, login.ErrUnavailable
	}

	provider, e := login.NewKeycloak(c.Provider, &http.Client{Transport: transport})
	if e != nil {
		transport.CloseIdleConnections()
		return nil, nil, nil, login.ErrUnavailable
	}

	fail := func() (*handler.LoginHandler, *handler.NativeSessionHandler, func(), error) {
		provider.Close()
		return nil, nil, nil, login.ErrUnavailable
	}
	service, e := login.NewService(provider, provider, store, resolvers...)
	if e != nil {
		return fail()
	}

	limiter, e := handler.NewLocalRegistrationLimiter(
		handler.RegistrationLimitConfig{
			Key:           c.RateKey,
			IPAttempts:    orDefault(c.Limits.Login, 5),
			EmailAttempts: orDefault(c.Limits.Login, 5),
			IPWindow:      15 * time.Minute,
			EmailWindow:   15 * time.Minute,
			MaxKeys:       4096,
			MaxConcurrent: 8,
		},
	)
	if e != nil {
		return fail()
	}

	h, e := handler.NewLoginHandler(service, limiter)
	if e != nil {
		return fail()
	}

	var sessions *handler.NativeSessionHandler
	if c.Provider.EnableRefresh {
		sessionService, e := login.NewSessionService(provider, provider, store)
		if e != nil {
			return fail()
		}
		// Separate counters: normal refresh traffic must not consume login limits.
		sessionLimiter, e := handler.NewLocalRegistrationLimiter(
			handler.RegistrationLimitConfig{
				Key:           c.RateKey,
				IPAttempts:    orDefault(c.Limits.Refresh, 60),
				EmailAttempts: 1,
				IPWindow:      time.Minute,
				EmailWindow:   time.Minute,
				MaxKeys:       4096,
				MaxConcurrent: 8,
			},
		)
		if e != nil {
			return fail()
		}

		sessions, e = handler.NewNativeSessionHandler(sessionService, sessionLimiter)
		if e != nil {
			return fail()
		}
	}

	return h, sessions, provider.Close, nil
}
