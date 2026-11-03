// Package handler defines the service's HTTP endpoints.
package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func NewRouter(origins ...string) *gin.Engine {
	return newRouter(slog.Default(), origins)
}

func newRouter(logger *slog.Logger, origins []string) *gin.Engine {
	return registrationRouter(logger, origins, nil)
}

// NewRouterWithRegistration enables only the explicitly composed HTTPS handler.
func NewRouterWithRegistration(h *RegistrationHandler, origins ...string) *gin.Engine {
	return registrationRouter(slog.Default(), origins, h)
}

func registrationRouter(logger *slog.Logger, origins []string, h *RegistrationHandler) *gin.Engine {
	return authRouter(logger, origins, h, nil)
}

func NewRouterWithAuth(registration *RegistrationHandler, login *LoginHandler, origins ...string) *gin.Engine {
	return authRouter(slog.Default(), origins, registration, login)
}

func authRouter(logger *slog.Logger, origins []string, h *RegistrationHandler, login *LoginHandler) *gin.Engine {
	return configuredRouter(logger, origins, h, login, nil)
}

func NewRouterWithProtected(registration *RegistrationHandler, login *LoginHandler, protected *Protected, origins ...string) *gin.Engine {
	return configuredRouter(slog.Default(), origins, registration, login, protected)
}

func NewRouterWithSessions(registration *RegistrationHandler, login *LoginHandler, protected *Protected, sessions *NativeSessionHandler, origins ...string) *gin.Engine {
	return configuredRouter(slog.Default(), origins, registration, login, protected, sessions)
}

func configuredRouter(logger *slog.Logger, origins []string, h *RegistrationHandler, login *LoginHandler, protected *Protected, sessionHandlers ...*NativeSessionHandler) *gin.Engine {
	return configuredWebRouter(logger, origins, h, login, protected, nil, sessionHandlers...)
}

func NewRouterWithWeb(registration *RegistrationHandler, login *LoginHandler, protected *Protected, sessions *NativeSessionHandler, web *WebSessionHandler, origins ...string) *gin.Engine {
	return configuredWebRouter(slog.Default(), origins, registration, login, protected, web, sessions)
}

func configuredWebRouter(logger *slog.Logger, origins []string, h *RegistrationHandler, login *LoginHandler, protected *Protected, web *WebSessionHandler, sessionHandlers ...*NativeSessionHandler) *gin.Engine {
	return configuredRecoveryRouter(logger, origins, h, login, protected, web, nil, sessionHandlers...)
}

func NewRouterWithRecovery(registration *RegistrationHandler, login *LoginHandler, protected *Protected, sessions *NativeSessionHandler, web *WebSessionHandler, recovery *RecoveryHandler, origins ...string) *gin.Engine {
	return configuredRecoveryRouter(slog.Default(), origins, registration, login, protected, web, recovery, sessions)
}

func configuredRecoveryRouter(logger *slog.Logger, origins []string, h *RegistrationHandler, login *LoginHandler, protected *Protected, web *WebSessionHandler, recovery *RecoveryHandler, sessionHandlers ...*NativeSessionHandler) *gin.Engine {
	var sessions *NativeSessionHandler
	if len(sessionHandlers) > 0 {
		sessions = sessionHandlers[0]
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(func(c *gin.Context) {
		if web != nil && web.isPath(c.Request.URL.Path) {
			c.Header("Cache-Control", "no-store")
			c.Header("Pragma", "no-cache")
		}
		if sessions != nil && (c.Request.URL.Path == "/api/v1/auth/refresh" || c.Request.URL.Path == "/api/v1/auth/revoke") {
			c.Header("Cache-Control", "no-store")
			c.Header("Pragma", "no-cache")
		}
		if (login != nil && c.Request.URL.Path == "/api/v1/auth/login") || (protected != nil && (c.Request.URL.Path == "/api/v1/me" || c.Request.URL.Path == "/api/v1/ai/chat")) {
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
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				} else {
					c.Abort()
				}
			}
			// Route templates exclude caller-supplied path, query, headers and bodies.
			logger.Info("http request", "request_id", requestID, "method", c.Request.Method,
				"route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
		}()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	})
	router.Use(func(c *gin.Context) {
		if recovery != nil && recovery.isPath(c.Request.URL.Path) {
			recovery.cors(c)
			return
		}
		if web != nil && web.isPath(c.Request.URL.Path) {
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
		for _, candidate := range origins {
			if origin == candidate {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		if c.Request.Method == http.MethodOptions {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
			method := c.GetHeader("Access-Control-Request-Method")
			allowedMethod := "GET"
			registrationPath := h != nil && c.Request.URL.Path == "/api/v1/registrations"
			chatPath := protected != nil && c.Request.URL.Path == "/api/v1/ai/chat"
			if registrationPath || chatPath {
				allowedMethod = "POST"
			}
			if method != allowedMethod {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CORS method not allowed"})
				return
			}
			for _, header := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(header)) {
				case "", "authorization", "content-type":
				case "idempotency-key":
					if !registrationPath {
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CORS header not allowed"})
						return
					}
				default:
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CORS header not allowed"})
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
	if web != nil {
		router.POST("/api/v1/web/login", web.Login)
		router.GET("/api/v1/web/session", web.Check)
		router.GET("/api/v1/web/csrf", web.CSRF)
		router.POST("/api/v1/web/refresh", web.Refresh)
		router.POST("/api/v1/web/logout", web.Logout)
	}
	if recovery != nil {
		router.POST("/api/v1/auth/forgot-password", recovery.Request)
		router.POST("/api/v1/auth/reset-password", recovery.Reset)
	}
	router.GET("/health", health)
	router.GET("/api/health", health)
	if protected != nil {
		protected.attach(router)
	}
	if login != nil {
		router.POST("/api/v1/auth/login", login.Handle)
	}
	if sessions != nil {
		router.POST("/api/v1/auth/refresh", sessions.Refresh)
		router.POST("/api/v1/auth/revoke", sessions.Revoke)
	}
	if h != nil {
		router.POST("/api/v1/registrations", func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			// Ignore Forwarded/X-Forwarded-Proto. Credentials require direct TLS;
			// proxy termination requires a separate reviewed deployment design.
			if c.Request.TLS == nil {
				c.AbortWithStatusJSON(http.StatusUpgradeRequired, gin.H{"error": "HTTPS required"})
				return
			}
			h.Handle(c)
		})
	}
	router.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "route not found"}) })
	router.NoMethod(func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"}) })
	return router
}
