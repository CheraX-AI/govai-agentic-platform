package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/app"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/config"
	"github.com/joho/godotenv"
)

// main keeps dependency creation and runtime work in focused internal packages.

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck())
	}
	if run() != nil {
		slog.Error("notification service stopped; check configuration and availability")
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return config.ErrConfig
	}
	settings, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx, settings)
}

func healthcheck() int {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:5003/health")
	if err != nil {
		return 1
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
