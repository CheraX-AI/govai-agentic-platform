package application

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/handler"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"io"
	"net/http"
	"os"
	"time"
)

func Login(c config.Login, store *postgres.Store) (*handler.LoginHandler, func(), error) {
	if !c.Enabled || store == nil {
		return nil, nil, login.ErrUnavailable
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAFile != "" {
		file, e := os.Open(c.CAFile)
		if e != nil {
			return nil, nil, login.ErrUnavailable
		}
		pem, e := io.ReadAll(io.LimitReader(file, 65537))
		_ = file.Close()
		if e != nil || len(pem) > 65536 {
			return nil, nil, login.ErrUnavailable
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, nil, login.ErrUnavailable
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	provider, e := login.NewKeycloak(c.Provider, &http.Client{Transport: transport})
	if e != nil {
		transport.CloseIdleConnections()
		return nil, nil, login.ErrUnavailable
	}
	fail := func() (*handler.LoginHandler, func(), error) { provider.Close(); return nil, nil, login.ErrUnavailable }
	service, e := login.NewService(provider, provider, store)
	if e != nil {
		return fail()
	}
	limiter, e := handler.NewLocalRegistrationLimiter(handler.RegistrationLimitConfig{Key: c.RateKey, IPAttempts: 5, EmailAttempts: 5, IPWindow: 15 * time.Minute, EmailWindow: 15 * time.Minute, MaxKeys: 4096, MaxConcurrent: 8})
	if e != nil {
		return fail()
	}
	h, e := handler.NewLoginHandler(service, limiter)
	if e != nil {
		return fail()
	}
	return h, provider.Close, nil
}
