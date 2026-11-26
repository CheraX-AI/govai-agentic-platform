package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/inbox"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/preferences"
	"github.com/gin-gonic/gin"
)

type citizenAPI struct {
	inbox       inbox.Store
	preferences preferences.Store
}

// RegisterCitizenRoutes requires a real JWT verifier and both storage adapters.
// The foundation does not call this until persistent storage is integrated.

func RegisterCitizenRoutes(
	router *gin.Engine, verifier *authn.Verifier,
	store inbox.Store, settings preferences.Store,
) error {
	if router == nil || verifier == nil || store == nil || settings == nil {
		return inbox.ErrUnavailable
	}
	api := &citizenAPI{inbox: store, preferences: settings}
	group := router.Group("/api/notifications",
		verifier.Authenticate(), authn.RequireVerifiedEmail(), authn.RequireRole("citizen"),
	)
	group.GET("", api.list)
	group.GET("/unread-count", api.unreadCount)
	group.PATCH("/:id/read", api.markRead)
	group.GET("/preferences", api.getPreferences)
	group.PUT("/preferences", api.replacePreferences)
	return nil
}

func requestOwner(c *gin.Context) (inbox.Owner, bool) {
	identity, ok := authn.Current(c)
	if !ok || identity.Issuer == "" || identity.Subject == "" ||
		!identity.Expires.After(time.Now()) {
		failCitizen(c, http.StatusUnauthorized, "unauthorized")
		return inbox.Owner{}, false
	}
	return inbox.Owner{Issuer: identity.Issuer, Subject: identity.Subject}, true
}

func requestContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 3*time.Second)
}

func failCitizen(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
