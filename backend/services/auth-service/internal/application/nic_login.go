package application

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"io"
	"net/http"
	"os"
)

// nicLogin resolves only persisted, ready profiles in the fixed configured realm.
// It does not mutate identities or expose the resolved address to HTTP callers.
type nicLogin struct {
	profiles *postgres.Store
	admin    *keycloak.Admin
	issuer   string
}

func (r *nicLogin) ResolveNIC(ctx context.Context, nic registration.NIC) (login.NICAccount, error) {
	b, err := r.profiles.RecoveryByNIC(ctx, r.issuer, nic)
	if errors.Is(err, postgres.ErrRecoveryUnavailable) {
		return login.NICAccount{}, login.ErrFailed
	}
	if err != nil {
		return login.NICAccount{}, login.ErrUnavailable
	}
	recipient, err := r.admin.RecoveryRecipient(ctx, b.Subject, b.OperationID)
	if err != nil {
		var upstream *registration.AdminError
		if errors.As(err, &upstream) && (upstream.Code == registration.AdminInvalid || upstream.Code == registration.AdminNotFound || upstream.Code == registration.AdminOwnership) {
			return login.NICAccount{}, login.ErrFailed
		}
		return login.NICAccount{}, login.ErrUnavailable
	}
	return login.NICAccount{Email: recipient.Email(), Issuer: b.Issuer, Subject: b.Subject}, nil
}
func NICLogin(c config.Registration, profiles *postgres.Store) (login.IdentifierResolver, func(), error) {
	if !c.Enabled || profiles == nil {
		return nil, nil, login.ErrUnavailable
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		f, err := os.Open(c.CAFile)
		if err != nil {
			return nil, nil, login.ErrUnavailable
		}
		pem, err := io.ReadAll(io.LimitReader(f, 65537))
		_ = f.Close()
		if err != nil || len(pem) > 65536 {
			return nil, nil, login.ErrUnavailable
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, nil, login.ErrUnavailable
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	admin, err := keycloak.NewRegistrationAdmin(c.Admin, &http.Client{Transport: transport})
	transport.CloseIdleConnections()
	if err != nil {
		return nil, nil, login.ErrUnavailable
	}
	return &nicLogin{profiles: profiles, admin: admin, issuer: c.Admin.BaseURL + "/realms/" + c.Admin.Realm}, admin.Close, nil
}
