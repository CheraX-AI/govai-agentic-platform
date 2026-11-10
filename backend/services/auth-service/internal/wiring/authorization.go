package wiring

import (
	"net/http"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
)

// Authorization connects JWT verification to protected routes.
func Authorization(c config.Authorization, store *postgres.Store) (*httpapi.Protected, func(), error) {
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

	protected, e := httpapi.NewProtected(verifier, store)
	if e != nil {
		verifier.Close()
		return nil, nil, e
	}

	return protected, verifier.Close, nil
}
