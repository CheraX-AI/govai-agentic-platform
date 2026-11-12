package authn

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// Authenticate requires a valid Bearer access token over HTTPS.
// Cookies, query parameters and forwarded headers never count as identity.
func (v *Verifier) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		noStore(c)

		if c.Request.TLS == nil {
			c.AbortWithStatusJSON(http.StatusUpgradeRequired, gin.H{"error": "HTTPS required.", "code": "https_required"})
			return
		}

		token, ok := bearerToken(c.Request)
		if !ok {
			unauthorized(c)
			return
		}

		identity, err := v.Verify(c.Request.Context(), token)
		if err != nil {
			unauthorized(c)
			return
		}

		c.Set(identityKey, identity)
		c.Next()
	}
}

// RequireVerifiedEmail allows only users whose email is verified.
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

// RequireRole allows only users with the given Keycloak realm role.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := Current(c)
		if !ok {
			unauthorized(c)
			return
		}

		if role == "" || !slices.Contains(identity.Roles, role) {
			forbidden(c)
			return
		}

		c.Next()
	}
}

// bearerToken reads the token from exactly one "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, bool) {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 {
		return "", false
	}

	scheme, token, found := strings.Cut(headers[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.Contains(token, " ") {
		return "", false
	}

	return token, true
}

// noStore stops browsers and proxies from caching authenticated responses.
func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

// unauthorized answers 401: the token is missing or invalid.
func unauthorized(c *gin.Context) {
	noStore(c)
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required.", "code": "unauthorized"})
}

// forbidden answers 403: the token is valid but the user lacks permission.
func forbidden(c *gin.Context) {
	noStore(c)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied.", "code": "forbidden"})
}
