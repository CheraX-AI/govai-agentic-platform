package authn

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const identityKey = "govai.authn.validated-identity.v1"

// Current returns a copy of the identity established by middleware.
func Current(c *gin.Context) (Identity, bool) {
	value, ok := c.Get(identityKey)
	identity, typed := value.(Identity)
	if !ok || !typed {
		return Identity{}, false
	}

	identity.Roles = append([]string(nil), identity.Roles...)
	return identity, true
}

func noStore(c *gin.Context) {

	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func unauthorized(c *gin.Context) {
	noStore(c)
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(401, gin.H{"error": "Authentication required.", "code": "unauthorized"})
}

func forbidden(c *gin.Context) {
	noStore(c)
	c.AbortWithStatusJSON(403, gin.H{"error": "Access denied.", "code": "forbidden"})
}

// Authenticate reads only one Authorization: Bearer header. Cookies, query
// parameters, forwarding headers and caller-supplied identity/role fields do not
// establish identity. A token is never stored in Gin context or logs.
func (v *Verifier) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		noStore(c)
		if c.Request.TLS == nil {
			c.AbortWithStatusJSON(http.StatusUpgradeRequired, gin.H{"error": "HTTPS required.", "code": "https_required"})
			return
		}

		headers := c.Request.Header.Values("Authorization")
		if len(headers) != 1 {
			unauthorized(c)
			return
		}

		parts := strings.Split(headers[0], " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			unauthorized(c)
			return
		}

		select {
		case v.gate <- struct{}{}:
		default:
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(
				503,
				gin.H{"error": "Authentication temporarily unavailable.", "code": "authentication_unavailable"},
			)
			return
		}

		identity, err := func() (Identity, error) {

			defer func() { <-v.gate }()
			return v.Verify(c.Request.Context(), parts[1])
		}()
		if err != nil {
			unauthorized(c)
			return
		}

		c.Set(identityKey, identity)
		c.Next()
	}
}

// RequireVerifiedEmail rejects accounts without verified email.
func RequireVerifiedEmail() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := Current(c)
		if !ok {
			unauthorized(c)
			return
		}

		if !identity.EmailVerified {
			forbidden(c)
			return
		}

		c.Next()
	}
}

// RequireRole checks the verified identity for the required role.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := Current(c)
		if !ok {
			unauthorized(c)
			return
		}

		for _, actual := range identity.Roles {
			if role != "" && actual == role {
				c.Next()
				return
			}
		}

		forbidden(c)
	}
}
