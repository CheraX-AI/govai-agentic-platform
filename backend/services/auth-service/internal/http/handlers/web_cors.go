package handler

import (
	"github.com/gin-gonic/gin"
	"strings"
)

func (h *WebSessionHandler) isPath(path string) bool {
	if path == "/api/v1/web/chat" {
		return h.chat != nil
	}
	switch path {
	case "/api/v1/web/login", "/api/v1/web/session", "/api/v1/web/csrf", "/api/v1/web/refresh", "/api/v1/web/logout":
		return true
	}
	return false
}
func (h *WebSessionHandler) cors(c *gin.Context) {
	c.Header("Vary", "Origin")
	values := c.Request.Header.Values("Origin")
	if len(values) > 1 || (len(values) == 1 && values[0] != h.origin) {
		webFail(c, 403, "origin_rejected")
		return
	}
	if len(values) == 1 {
		c.Header("Access-Control-Allow-Origin", h.origin)
		c.Header("Access-Control-Allow-Credentials", "true")
	}
	if c.Request.Method != "OPTIONS" {
		c.Next()
		return
	}
	if c.Request.TLS == nil || len(values) != 1 {
		webFail(c, 403, "preflight_rejected")
		return
	}
	method := "POST"
	if c.Request.URL.Path == "/api/v1/web/session" || c.Request.URL.Path == "/api/v1/web/csrf" {
		method = "GET"
	}
	c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
	c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
	if c.GetHeader("Access-Control-Request-Method") != method {
		webFail(c, 403, "preflight_rejected")
		return
	}
	for _, value := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "", "content-type", "x-govai-web", "x-csrf-token":
		default:
			webFail(c, 403, "preflight_rejected")
			return
		}
	}
	c.Header("Access-Control-Allow-Methods", method)
	c.Header("Access-Control-Allow-Headers", "Content-Type, X-GovAI-Web, X-CSRF-Token")
	c.AbortWithStatus(204)
}
