package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Routes exposes foundation health only; no citizen data endpoints exist yet.

func Routes(connected func() bool) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	_ = router.SetTrustedProxies(nil)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/api/health", func(c *gin.Context) {
		broker := connected()
		status := http.StatusOK
		if !broker {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"stage":              "foundation",
			"broker_connected":   broker,
			"processing_enabled": false,
		})
	})

	// A broker connection alone is not evidence of a usable notification pipeline.

	router.GET("/api/ready", func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"reason": "persistence_not_implemented",
		})
	})
	return router
}
