package wiring

import (
	"crypto/x509"
	"os"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/chatgateway"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
)

// WebChat attaches the core chat client to browser sessions.
func WebChat(c config.WebChat, h *httpapi.WebSessionHandler) (func(), error) {
	if !c.Enabled {
		return func() {}, nil
	}

	if h == nil {
		return nil, chatgateway.ErrUnavailable
	}

	var roots *x509.CertPool
	if c.CAFile != "" {
		data, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, chatgateway.ErrUnavailable
		}

		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(data) {
			return nil, chatgateway.ErrUnavailable
		}
	}

	client, err := chatgateway.New(c.Endpoint, roots, c.Timeout)
	if err != nil {
		return nil, err
	}

	if err = h.EnableChat(client); err != nil {
		client.Close()
		return nil, err
	}

	return client.Close, nil
}
