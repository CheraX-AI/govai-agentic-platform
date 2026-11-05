package application

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"os"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/websession"
	"github.com/jackc/pgx/v5/pgxpool"
)

func webProvider(c config.Web) (*login.Keycloak, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		f, err := os.Open(c.CAFile)
		if err != nil {
			return nil, websession.ErrUnavailable
		}

		pem, err := io.ReadAll(io.LimitReader(f, 65537))
		_ = f.Close()
		if err != nil || len(pem) > 65536 {
			return nil, websession.ErrUnavailable
		}

		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}

		if !roots.AppendCertsFromPEM(pem) {
			return nil, websession.ErrUnavailable
		}

		transport.TLSClientConfig.RootCAs = roots
	}

	provider, err := login.NewKeycloak(c.Provider, &http.Client{Transport: transport})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, websession.ErrUnavailable
	}

	return provider, nil
}

type maintenanceRevoker struct{ provider *login.Keycloak }

func (r maintenanceRevoker) RevokeNative(ctx context.Context, raw string) error {
	return r.provider.RevokeRefresh(ctx, raw)
}

func WebRecoveryQueue(ctx context.Context, c config.Web, pool *pgxpool.Pool) (*postgres.BoundWebSessionStore, error) {
	if !c.Enabled || postgres.WebSessionsReady(ctx, pool) != nil {
		return nil, websession.ErrUnavailable
	}

	store, err := postgres.NewWebSessionStore(pool, c.DataKey)
	if err != nil {
		return nil, websession.ErrUnavailable
	}

	return postgres.NewBoundWebSessionStore(store, c.Provider.Issuer, c.Provider.ClientID)
}

func ReconcileWeb(ctx context.Context, c config.Web, pool *pgxpool.Pool) (websession.Reconciliation, error) {
	queue, err := WebRecoveryQueue(ctx, c, pool)
	if err != nil {
		return websession.Reconciliation{}, websession.ErrUnavailable
	}

	provider, err := webProvider(c)
	if err != nil {
		return websession.Reconciliation{}, websession.ErrUnavailable
	}

	defer provider.Close()
	return websession.Reconcile(ctx, queue, maintenanceRevoker{provider}, 20)
}
