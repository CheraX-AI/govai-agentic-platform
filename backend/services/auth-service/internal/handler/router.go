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
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(func(c *gin.Context) {
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
			if method != "GET" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CORS method not allowed"})
				return
			}
			for _, header := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(header)) {
				case "", "authorization", "content-type":
				default:
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CORS header not allowed"})
					return
				}
			}
			c.Header("Access-Control-Allow-Methods", "GET")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	health := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }
	router.GET("/health", health)
	router.GET("/api/health", health)
	router.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "route not found"}) })
	router.NoMethod(func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"}) })
	return router
}
