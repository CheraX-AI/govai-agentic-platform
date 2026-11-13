package wiring

import (
	"net/http"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
)

// Authorization connects JWT verification to the protected route (/api/v1/me).
func Authorization(c config.Authorization, store *postgres.Store) (*handler.Protected, func(), error) {
	if !c.Enabled || store == nil {
		return nil, nil, config.ErrAuthorizationConfig
	}

	transport, e := newKeycloakTransport(c.CAFile)
	if e != nil {
		return nil, nil, config.ErrAuthorizationConfig
	}

	verifier, e := authn.New(
		authn.Config{Issuer: c.Issuer, Audience: c.Audience, Clients: c.Clients},
		&http.Client{Transport: transport},
	)
	if e != nil {
		transport.CloseIdleConnections()
		return nil, nil, config.ErrAuthorizationConfig
	}

	protected, e := handler.NewProtected(verifier, store)
	if e != nil {
		verifier.Close()
		return nil, nil, e
	}

	return protected, verifier.Close, nil
}
