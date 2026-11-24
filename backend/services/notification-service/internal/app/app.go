package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/handlers"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/messaging"
	"github.com/gin-gonic/gin"
)

// Run owns HTTP and broker lifecycles, draining both before returning.

func Run(ctx context.Context, settings config.Config) error {
	broker, err := messaging.New(messaging.Settings{
		URL:              settings.BrokerURL,
		CAFile:           settings.BrokerCA,
		SecurityPrefetch: settings.SecurityPrefetch,
		NormalPrefetch:   settings.NormalPrefetch,
	}, slog.Default())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	brokerDone := make(chan struct{})
	go func() {
		defer close(brokerDone)
		broker.Run(ctx)
	}()
	defer func() {
		cancel()
		<-brokerDone
	}()

	gin.SetMode(gin.ReleaseMode)
	server := &http.Server{
		Addr:              settings.Address,
		Handler:           handlers.Routes(broker.Connected),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	serverDone := make(chan error, 1)
	go func() {
		slog.Info("notification foundation starting", "processing_enabled", false)
		serverDone <- server.ListenAndServe()
	}()

	select {
	case err = <-serverDone:
		return err
	case <-ctx.Done():
	}
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if err = server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return err
	}
	err = <-serverDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
