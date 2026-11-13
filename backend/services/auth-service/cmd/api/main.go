package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/httpapi"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/wiring"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load local settings before validating enabled features.
	if err := config.LoadLocalEnv(".env.recovery"); err != nil {
		return err
	}

	if err := config.LoadLocalEnv(".env.web"); err != nil {
		return err
	}

	if err := config.LoadLocalEnv(".env.authorization"); err != nil {
		return err
	}

	if err := config.LoadLocalEnv(".env.login"); err != nil {
		return err
	}

	if err := config.LoadLocalEnv(".env.registration"); err != nil {
		return err
	}

	if err := config.LoadLocalEnv(".env.database"); err != nil {
		return err
	}

	if err := config.LoadLocalEnv(".env"); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Prepare shutdown handling and feature configuration.
	gin.SetMode(gin.ReleaseMode)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dbCfg, err := config.LoadDatabase()
	if err != nil {
		return err
	}

	regCfg, err := config.LoadRegistration()
	if err != nil {
		return err
	}

	loginCfg, err := config.LoadLogin()
	if err != nil {
		return err
	}

	authorizationCfg, err := config.LoadAuthorization()
	if err != nil {
		return err
	}

	webCfg, err := config.LoadWeb()
	if err != nil {
		return err
	}

	// Optional rate-limit overrides, shared by login, web sessions and registration.
	limits, err := config.LoadRateLimits()
	if err != nil {
		return err
	}

	regCfg.Limits = limits
	loginCfg.Limits = limits
	webCfg.Limits = limits

	recoveryCfg, err := config.LoadRecovery(regCfg, webCfg)
	if err != nil {
		return err
	}

	tlsEnabled := regCfg.Enabled ||
		loginCfg.Enabled ||
		authorizationCfg.Enabled ||
		webCfg.Enabled ||
		recoveryCfg.Enabled
	var apiTLS *tls.Config
	if tlsEnabled {
		cert, err := tls.LoadX509KeyPair(os.Getenv("API_TLS_CERT_FILE"), os.Getenv("API_TLS_KEY_FILE"))
		if err != nil {
			return config.ErrRegistrationConfig
		}

		apiTLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	}

	// Open shared persistence before composing HTTP handlers.
	var pool *pgxpool.Pool
	var store *postgres.Store
	if dbCfg.URL != "" {
		pool, err = postgres.Open(ctx, dbCfg.URL)
		if err != nil {
			return err
		}

		defer pool.Close()
		if err := postgres.Ready(ctx, pool); err != nil {
			return err
		}

		store, err = postgres.NewStore(pool, dbCfg.DataKey, dbCfg.NICKey)
		if err != nil {
			return err
		}

		slog.Info("authentication database connected")
	}

	// Resolve NIC login through the registered profile binding.
	var identifierResolver login.IdentifierResolver
	if regCfg.Enabled && (loginCfg.Enabled || webCfg.Enabled) {
		expectedIssuer := regCfg.Admin.BaseURL + "/realms/" + regCfg.Admin.Realm
		if (loginCfg.Enabled &&
			loginCfg.Provider.Issuer != expectedIssuer) ||
			(webCfg.Enabled &&
				webCfg.Provider.Issuer != expectedIssuer) {
			return login.ErrUnavailable
		}

		var closeIdentifiers func()
		identifierResolver, closeIdentifiers, err = wiring.NICLogin(regCfg, store)
		if err != nil {
			return err
		}

		defer closeIdentifiers()
	}

	// Build each enabled feature with its required dependencies.
	var registrationHandler *httpapi.RegistrationHandler
	if regCfg.Enabled {
		var closeRegistration func()
		registrationHandler, closeRegistration, err = wiring.Registration(regCfg, store)
		if err != nil {
			return err
		}

		defer closeRegistration()
		slog.Info("registration route enabled with HTTPS")
	}

	var loginHandler *httpapi.LoginHandler
	var sessionHandler *httpapi.NativeSessionHandler
	if loginCfg.Enabled {
		var closeLogin func()
		loginHandler, sessionHandler, closeLogin, err = wiring.LoginWithSessions(loginCfg, store, identifierResolver)
		if err != nil {
			return err
		}

		defer closeLogin()
		slog.Info("mobile login route enabled with HTTPS")
	}

	var protected *httpapi.Protected
	if authorizationCfg.Enabled {
		var closeAuthorization func()
		protected, closeAuthorization, err = wiring.Authorization(authorizationCfg, store)
		if err != nil {
			return err
		}

		defer closeAuthorization()
		slog.Info("JWT authorization routes enabled with HTTPS")
	}

	var webHandler *httpapi.WebSessionHandler
	if webCfg.Enabled {
		var closeWeb func()
		var fenceKeys [][]byte
		if len(recoveryCfg.FenceKey) > 0 {
			fenceKeys = append(fenceKeys, recoveryCfg.FenceKey)
		}

		webHandler, closeWeb, err = wiring.WebWithIdentifiers(ctx, webCfg, pool, store, identifierResolver, fenceKeys...)
		if err != nil {
			return err
		}

		defer closeWeb()
		slog.Info("web session routes enabled with HTTPS")
	}

	chatCfg, err := config.LoadWebChat(webCfg.Enabled)
	if err != nil {
		return err
	}

	closeChat, err := wiring.WebChat(chatCfg, webHandler)
	if err != nil {
		return err
	}

	defer closeChat()
	var recoveryHandler *httpapi.RecoveryHandler
	if recoveryCfg.Enabled {
		var closeRecovery func()
		recoveryHandler, closeRecovery, err = wiring.Recovery(ctx, recoveryCfg, regCfg, webCfg, pool, store)
		if err != nil {
			return err
		}

		defer closeRecovery()
		slog.Info("password recovery routes enabled with HTTPS and guarded web sessions")
	}

	// Mount the configured routes and database readiness check.
	router := httpapi.NewRouter(httpapi.RouterOptions{
		CORSOrigins:    cfg.CORSOrigins,
		Registration:   registrationHandler,
		Login:          loginHandler,
		Protected:      protected,
		NativeSessions: sessionHandler,
		WebSessions:    webHandler,
		Recovery:       recoveryHandler,
	})
	if pool != nil {
		router.GET("/api/ready", func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			if postgres.Ready(c.Request.Context(), pool) != nil ||
				(webCfg.Enabled &&
					postgres.WebSessionsReady(c.Request.Context(), pool) != nil) ||
				(len(recoveryCfg.FenceKey) > 0 &&
					postgres.RecoveryReady(c.Request.Context(), pool) != nil) {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": "ready"})
		})
	}

	// Set HTTP budgets before accepting requests.
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig:         apiTLS,
	}
	if tlsEnabled {
		server.WriteTimeout = 45 * time.Second
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("starting HTTP server", "port", cfg.Port, "tls", tlsEnabled)
		if tlsEnabled {
			serverErrors <- server.ListenAndServeTLS("", "")
		} else {
			serverErrors <- server.ListenAndServe()
		}
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		stop()
	}

	slog.Info("shutting down HTTP server")
	// Let active requests finish within the shutdown budget.
	shutdownTimeout := 5 * time.Second
	if tlsEnabled {
		shutdownTimeout = 30 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return err
	}

	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	slog.Info("HTTP server stopped")
	return nil
}
