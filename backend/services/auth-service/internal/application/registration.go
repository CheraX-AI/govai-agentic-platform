// Package application composes real registration dependencies without modifying
// a realm, applying migrations or making startup Keycloak requests.
package application

import (
	"crypto/hmac"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/handler"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

func Registration(c config.Registration, store *postgres.Store) (*handler.RegistrationHandler, func(), error) {
	if !c.Enabled || store == nil || len(c.FingerprintKey) != 32 || len(c.RateKey) != 32 || hmac.Equal(c.FingerprintKey, c.RateKey) {
		return nil, nil, config.ErrRegistrationConfig
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		file, e := os.Open(c.CAFile)
		if e != nil {
			return nil, nil, config.ErrRegistrationConfig
		}
		pem, e := io.ReadAll(io.LimitReader(file, 65537))
		_ = file.Close()
		if e != nil || len(pem) > 65536 {
			return nil, nil, config.ErrRegistrationConfig
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, nil, config.ErrRegistrationConfig
		}
		transport.TLSClientConfig.RootCAs = roots
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
	service, e := registration.NewService(registration.ServiceConfig{Issuer: c.Admin.BaseURL + "/realms/" + c.Admin.Realm, FingerprintKey: c.FingerprintKey, Timeout: 25 * time.Second, EmailRetryBackoff: time.Minute, MaxEmailAttempts: 3}, observedAdmin{admin}, store, store)
	if e != nil {
		return fail()
	}
	limiter, e := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: c.RateKey, IPAttempts: 5, EmailAttempts: 3, IPWindow: 15 * time.Minute, EmailWindow: time.Hour, MaxKeys: 4096, MaxConcurrent: 4})
	if e != nil {
		return fail()
	}
	h, e := handler.NewRegistrationHandler(service, limiter, handler.RegistrationHTTPConfig{Timeout: 28 * time.Second, UnavailableRetryAfter: 30 * time.Second})
	if e != nil {
		return fail()
	}
	return h, admin.Close, nil
}
