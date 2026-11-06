package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/chat"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if run() != nil {
		slog.Error("core server stopped; check configuration and availability")
		os.Exit(1)
	}
}
func run() error {
	if _, err := os.Stat(".env"); err == nil {
		if godotenv.Load(".env") != nil {
			return config.ErrConfig
		}
	} else if !os.IsNotExist(err) {
		return config.ErrConfig
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	certificate, err := tls.LoadX509KeyPair(c.Certificate, c.Key)
	if err != nil {
		return config.ErrConfig
	}
	transport, err := trustedTransport(c.CAFile)
	if err != nil {
		return err
	}
	defer transport.CloseIdleConnections()
	verifier, err := authn.New(authn.Config{Issuer: c.Issuer, Audience: c.Audience, Clients: c.Clients}, &http.Client{Transport: transport})
	if err != nil {
		return config.ErrConfig
	}
	defer verifier.Close()
	gin.SetMode(gin.ReleaseMode)
	agentConfig, err := config.LoadAgent(c.RequestTimeout)
	if err != nil {
		return err
	}
	var agent chat.Agent = chat.Unavailable{}
	if agentConfig.Enabled {
		agentTransport, err := trustedTransport(agentConfig.CAFile)
		if err != nil {
			return err
		}
		defer agentTransport.CloseIdleConnections()
		connected, err := chat.NewHTTP(agentConfig.HTTP, &http.Client{Transport: agentTransport})
		if err != nil {
			return config.ErrConfig
		}
		defer connected.Close()
		agent = connected
	}
	router, err := httpapi.NewRouter(verifier, agent, c.Origins, slog.Default(), c.RequestTimeout)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: ":" + c.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: c.RequestTimeout + 5*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() {
		slog.Info("starting core chat API", "port", c.Port, "agent_configured", agentConfig.Enabled)
		done <- server.ListenAndServeTLS("", "")
	}()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return err
	}
	err = <-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func trustedTransport(caFile string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		file, err := os.Open(caFile)
		if err != nil {
			return nil, config.ErrConfig
		}
		data, err := io.ReadAll(io.LimitReader(file, 65537))
		_ = file.Close()
		if err != nil || len(data) > 65536 {
			return nil, config.ErrConfig
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return nil, config.ErrConfig
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	return transport, nil
}
