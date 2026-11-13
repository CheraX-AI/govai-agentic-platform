// Package wiring composes real registration dependencies without modifying
// a realm, applying migrations or making startup Keycloak requests.
package wiring

import (
	"crypto/hmac"
	"net/http"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
)

// Registration connects the handler, registration service and persistent stores.
func Registration(c config.Registration, store *postgres.Store) (*handler.RegistrationHandler, func(), error) {
	if !c.Enabled ||
		store == nil ||
		len(c.FingerprintKey) != 32 ||
		len(c.RateKey) != 32 ||
		hmac.Equal(c.FingerprintKey, c.RateKey) {
		return nil, nil, config.ErrRegistrationConfig
	}

	transport, e := newKeycloakTransport(c.CAFile)
	if e != nil {
		return nil, nil, config.ErrRegistrationConfig
	}

	admin, e := keycloak.NewRegistrationAdmin(c.Admin, &http.Client{Transport: transport})
	if e != nil {
		transport.CloseIdleConnections()
		return nil, nil, config.ErrRegistrationConfig
	}

	fail := func() (*handler.RegistrationHandler, func(), error) {
		admin.Close()
		return nil, nil, config.ErrRegistrationConfig
	}
	service, e := registration.NewService(
		registration.ServiceConfig{
			Issuer:            c.Admin.BaseURL + "/realms/" + c.Admin.Realm,
			FingerprintKey:    c.FingerprintKey,
			Timeout:           25 * time.Second,
			EmailRetryBackoff: time.Minute,
			MaxEmailAttempts:  3,
		},
		observedAdmin{admin},
		store,
		store,
	)
	if e != nil {
		return fail()
	}

	limiter, e := handler.NewLocalRegistrationLimiter(
		handler.RegistrationLimitConfig{
			Key:           c.RateKey,
			IPAttempts:    orDefault(c.Limits.RegistrationIP, 5),
			EmailAttempts: orDefault(c.Limits.RegistrationEmail, 3),
			IPWindow:      15 * time.Minute,
			EmailWindow:   time.Hour,
			MaxKeys:       4096,
			MaxConcurrent: 4,
		},
	)
	if e != nil {
		return fail()
	}

	h, e := handler.NewRegistrationHandler(
		service,
		limiter,
		handler.RegistrationHTTPConfig{Timeout: 28 * time.Second, UnavailableRetryAfter: 30 * time.Second},
	)
	if e != nil {
		return fail()
	}

	return h, admin.Close, nil
}
