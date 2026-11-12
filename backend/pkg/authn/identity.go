package authn

import (
	"time"

	"github.com/gin-gonic/gin"
)

// identityKey is where Authenticate stores the verified user in the Gin context.
const identityKey = "govai.authn.identity"

// Identity is the verified caller. It never holds the raw token.
type Identity struct {
	Issuer        string
	Subject       string // Keycloak user ID
	Client        string // app that requested the token
	Roles         []string
	EmailVerified bool
	Expires       time.Time
}

// Current returns the identity verified by Authenticate for this request.
func Current(c *gin.Context) (Identity, bool) {
	value, found := c.Get(identityKey)
	if !found {
		return Identity{}, false
	}

	identity, ok := value.(Identity)
	if !ok {
		return Identity{}, false
	}

	// Copy the roles so a handler cannot change what later handlers see.
	identity.Roles = append([]string(nil), identity.Roles...)
	return identity, true
}
