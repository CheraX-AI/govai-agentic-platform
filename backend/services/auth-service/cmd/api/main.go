package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/app"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load the service's single local settings file before validating enabled features.
	// Process environment settings win; the migration owner login (.env.migrate) is never loaded here.
	cfg, err := app.LoadConfig(".env")
	if err != nil {
		return err
	}

	// Ctrl+C or SIGTERM starts a graceful shutdown; a second signal stops immediately.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}

	defer a.Close()
	return a.Run(ctx)
}
