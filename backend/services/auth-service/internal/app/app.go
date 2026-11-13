// Package app starts and stops the authentication service: settings, features, routes and server.
package app

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// App is the assembled authentication service, ready to Run.
type App struct {
	server  *http.Server
	port    string
	tls     bool
	closers []func()
}

// New connects the database, builds every enabled feature and mounts the routes.
// On failure it releases whatever it had already opened.
func New(ctx context.Context, cfg Config) (_ *App, err error) {
	a := &App{port: cfg.Base.Port, tls: cfg.tlsEnabled()}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()

	gin.SetMode(gin.ReleaseMode)

	var apiTLS *tls.Config
	if a.tls {
		cert, err := tls.LoadX509KeyPair(os.Getenv("API_TLS_CERT_FILE"), os.Getenv("API_TLS_KEY_FILE"))
		if err != nil {
			return nil, config.ErrRegistrationConfig
		}

		apiTLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	}

	pool, store, err := a.openDatabase(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	f, err := a.buildFeatures(ctx, cfg, pool, store)
	if err != nil {
		return nil, err
	}

	router := handler.NewRouter(handler.RouterOptions{
		CORSOrigins:    cfg.Base.CORSOrigins,
		Registration:   f.registration,
		Login:          f.login,
		Protected:      f.protected,
		NativeSessions: f.sessions,
		WebSessions:    f.web,
		Recovery:       f.recovery,
	})
	if pool != nil {
		router.GET("/api/ready", readyHandler(cfg, pool))
	}

	a.server = &http.Server{
		Addr:              ":" + cfg.Base.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig:         apiTLS,
	}
	if a.tls {
		a.server.WriteTimeout = 45 * time.Second
	}

	return a, nil
}

// Close releases every resource New opened, newest first. Safe to call twice.
func (a *App) Close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}

	a.closers = nil
}

// onClose registers a cleanup step for Close.
func (a *App) onClose(close func()) {
	a.closers = append(a.closers, close)
}

// openDatabase connects to PostgreSQL when DATABASE_URL is set; otherwise it returns nils.
func (a *App) openDatabase(ctx context.Context, c config.Database) (*pgxpool.Pool, *postgres.Store, error) {
	if c.URL == "" {
		return nil, nil, nil
	}

	pool, err := postgres.Open(ctx, c.URL)
	if err != nil {
		return nil, nil, err
	}

	a.onClose(pool.Close)
	if err := postgres.Ready(ctx, pool); err != nil {
		return nil, nil, err
	}

	store, err := postgres.NewStore(pool, c.DataKey, c.NICKey)
	if err != nil {
		return nil, nil, err
	}

	slog.Info("authentication database connected")
	return pool, store, nil
}

// readyHandler reports whether the database and every enabled feature's schema are ready.
func readyHandler(cfg Config, pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		ctx := c.Request.Context()
		if postgres.Ready(ctx, pool) != nil ||
			(cfg.Web.Enabled && postgres.WebSessionsReady(ctx, pool) != nil) ||
			(len(cfg.Recovery.FenceKey) > 0 && postgres.RecoveryReady(ctx, pool) != nil) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}
