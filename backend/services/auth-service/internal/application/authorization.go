package application

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"os"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
)

// Authorization connects JWT verification to protected routes.
func Authorization(c config.Authorization, store *postgres.Store) (*handler.Protected, func(), error) {
	if !c.Enabled || store == nil {
		return nil, nil, config.ErrAuthorizationConfig
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		file, e := os.Open(c.CAFile)
		if e != nil {
			return nil, nil, config.ErrAuthorizationConfig
		}

		pem, e := io.ReadAll(io.LimitReader(file, 65537))
		_ = file.Close()
		if e != nil || len(pem) > 65536 {
			return nil, nil, config.ErrAuthorizationConfig
		}

		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}

		if !roots.AppendCertsFromPEM(pem) {
			return nil, nil, config.ErrAuthorizationConfig
		}

		transport.TLSClientConfig.RootCAs = roots
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
