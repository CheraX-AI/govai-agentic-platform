package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Run serves requests until ctx is canceled, then lets active requests finish.
func (a *App) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("starting HTTP server", "port", a.port, "tls", a.tls)
		if a.tls {
			serverErrors <- a.server.ListenAndServeTLS("", "")
		} else {
			serverErrors <- a.server.ListenAndServe()
		}
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down HTTP server")
	shutdownTimeout := 5 * time.Second
	if a.tls {
		shutdownTimeout = 30 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		_ = a.server.Close()
		return err
	}

	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	slog.Info("HTTP server stopped")
	return nil
}
