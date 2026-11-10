// Package httpapi defines the service's HTTP endpoints.
package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RouterOptions supplies the handlers for enabled features. Health routes are always mounted.
type RouterOptions struct {
	Logger         *slog.Logger
	CORSOrigins    []string
	Registration   *RegistrationHandler
	Login          *LoginHandler
	Protected      *Protected
	NativeSessions *NativeSessionHandler
	WebSessions    *WebSessionHandler
	Recovery       *RecoveryHandler
}

// NewRouter registers the configured auth routes and shared HTTP middleware.
func NewRouter(options RouterOptions) *gin.Engine {
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(func(c *gin.Context) {
		if web := options.WebSessions; web != nil && web.isPath(c.Request.URL.Path) {
			c.Header("Cache-Control", "no-store")
			c.Header("Pragma", "no-cache")
		}

		if options.NativeSessions != nil &&
			(c.Request.URL.Path == "/api/v1/auth/refresh" ||
				c.Request.URL.Path == "/api/v1/auth/revoke") {
			c.Header("Cache-Control", "no-store")
			c.Header("Pragma", "no-cache")
		}

		if (options.Login != nil &&
			c.Request.URL.Path == "/api/v1/auth/login") ||
			(options.Protected != nil &&
				(c.Request.URL.Path == "/api/v1/me" ||
					c.Request.URL.Path == "/api/v1/ai/chat")) {
			c.Header("Cache-Control", "no-store")
			c.Header("Pragma", "no-cache")
		}

		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			panic("request ID generation failed")
		}

		requestID := hex.EncodeToString(id[:])
		c.Header("X-Request-ID", requestID)
		started := time.Now()
		defer func() {
			if recover() != nil {
				logger.Error("request panic", "request_id", requestID)
				if !c.Writer.Written() {
					abortJSON(c, http.StatusInternalServerError, gin.H{"error": "internal server error"})
				} else {
					c.Abort()
				}
			}
			// Route templates exclude caller-supplied path, query, headers and bodies.
			logger.Info("http request", "request_id", requestID, "method", c.Request.Method,
				"route",
				c.FullPath(),
				"status",
				c.Writer.Status(),
				"duration_ms",
				time.Since(started).Milliseconds())
		}()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	})
	router.Use(func(c *gin.Context) {
		if recovery := options.Recovery; recovery != nil && recovery.isPath(c.Request.URL.Path) {
			recovery.cors(c)
			return
		}

		if web := options.WebSessions; web != nil && web.isPath(c.Request.URL.Path) {
			web.cors(c)
			return
		}

		c.Header("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := false
		for _, candidate := range options.CORSOrigins {
			if origin == candidate {
				allowed = true
				break
			}
		}

		if !allowed {
			abortJSON(c, http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		if c.Request.Method == http.MethodOptions {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
			method := c.GetHeader("Access-Control-Request-Method")
			allowedMethod := "GET"
			registrationPath := options.Registration != nil && c.Request.URL.Path == "/api/v1/registrations"
			chatPath := options.Protected != nil && c.Request.URL.Path == "/api/v1/ai/chat"
			if registrationPath || chatPath {
				allowedMethod = "POST"
			}

			if method != allowedMethod {
				abortJSON(c, http.StatusForbidden, gin.H{"error": "CORS method not allowed"})
				return
			}

			for _, header := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(header)) {
				case "", "authorization", "content-type":
				case "idempotency-key":
					if !registrationPath {
						abortJSON(c, http.StatusForbidden, gin.H{"error": "CORS header not allowed"})
						return
					}
				default:
					abortJSON(c, http.StatusForbidden, gin.H{"error": "CORS header not allowed"})
					return
				}
			}

			c.Header("Access-Control-Allow-Methods", allowedMethod)
			allowedHeaders := "Authorization, Content-Type"
			if registrationPath {
				allowedHeaders += ", Idempotency-Key"
			}

			c.Header("Access-Control-Allow-Headers", allowedHeaders)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}

	health := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }
	if web := options.WebSessions; web != nil {
		if web.chat != nil {
			router.POST("/api/v1/web/chat", web.Chat)
		}

		router.POST("/api/v1/web/login", web.Login)
		router.GET("/api/v1/web/session", web.Check)
		router.GET("/api/v1/web/csrf", web.CSRF)
		router.POST("/api/v1/web/refresh", web.Refresh)
		router.POST("/api/v1/web/logout", web.Logout)
	}

	if recovery := options.Recovery; recovery != nil {
		router.POST("/api/v1/auth/forgot-password", recovery.Request)
		router.POST("/api/v1/auth/reset-password", recovery.Reset)
	}

	router.GET("/health", health)
	router.GET("/api/health", health)
	if protected := options.Protected; protected != nil {
		protected.attach(router)
	}

	if login := options.Login; login != nil {
		router.POST("/api/v1/auth/login", login.Handle)
	}

	if sessions := options.NativeSessions; sessions != nil {
		router.POST("/api/v1/auth/refresh", sessions.Refresh)
		router.POST("/api/v1/auth/revoke", sessions.Revoke)
	}

	if registration := options.Registration; registration != nil {
		router.POST("/api/v1/registrations", func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			// Ignore Forwarded/X-Forwarded-Proto. Credentials require direct TLS;
			// proxy termination requires a separate reviewed deployment design.
			if c.Request.TLS == nil {
				abortJSON(c, http.StatusUpgradeRequired, gin.H{"error": "HTTPS required"})
				return
			}

			registration.Handle(c)
		})
	}

	router.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "route not found"}) })
	router.NoMethod(
		func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"}) },
	)
	return router
}
