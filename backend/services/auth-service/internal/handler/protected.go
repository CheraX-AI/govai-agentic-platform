package handler

import (
	"context"
	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type ReadyProfiles interface {
	HasReadyProfile(context.Context, string, string) (bool, error)
}
type Protected struct {
	verifier *authn.Verifier
	profiles ReadyProfiles
}

func NewProtected(verifier *authn.Verifier, profiles ReadyProfiles) (*Protected, error) {
	if verifier == nil || profiles == nil {
		return nil, authn.ErrConfig
	}
	return &Protected{verifier, profiles}, nil
}
func (p *Protected) readyProfile(c *gin.Context) {
	identity, ok := authn.Current(c)
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "Authentication required.", "code": "unauthorized"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	ready, e := p.profiles.HasReadyProfile(ctx, identity.Issuer, identity.Subject)
	cancel()
	if e != nil {
		c.AbortWithStatusJSON(503, gin.H{"error": "Profile service temporarily unavailable.", "code": "profile_unavailable"})
		return
	}
	if !ready {
		c.AbortWithStatusJSON(403, gin.H{"error": "Access denied.", "code": "forbidden"})
		return
	}
	c.Next()
}
func (p *Protected) attach(router *gin.Engine) {
	group := router.Group("/api/v1", p.verifier.Authenticate(), authn.RequireVerifiedEmail(), authn.RequireRole("citizen"), p.readyProfile)
	group.GET("/me", func(c *gin.Context) {
		identity, ok := authn.Current(c)
		if !ok {
			c.AbortWithStatus(401)
			return
		}
		c.JSON(200, gin.H{"subject": identity.Subject, "email_verified": identity.EmailVerified, "roles": identity.Roles})
	})
	group.POST("/ai/chat", func(c *gin.Context) {
		// No RAG contract/client exists. Authentication/authorization runs before this
		// placeholder; no Python request, generated answer or citizen data forwarding.
		c.JSON(http.StatusNotImplemented, gin.H{"error": "AI chat integration is not implemented.", "code": "chat_not_implemented"})
	})
}
