// Package authn verifies Keycloak access tokens and provides Gin middleware for them.
package authn

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Verifier checks access tokens. Create one at startup and reuse it.
type Verifier struct {
	config Config
	client *http.Client
	oidc   *oidc.IDTokenVerifier
}

// New creates a verifier. The client must verify TLS certificates normally;
// a private CA may be added to its root pool.
func New(config Config, client *http.Client) (*Verifier, error) {
	if client == nil || config.validate() != nil || skipsTLSChecks(client) {
		return nil, ErrConfig
	}

	if config.Now == nil {
		config.Now = time.Now
	}

	config.Clients = slices.Clone(config.Clients)

	// Copy the caller's client so our timeout and redirect rule don't change theirs.
	keysClient := *client
	keysClient.Timeout = 5 * time.Second
	keysClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Keycloak publishes its public signing keys at this fixed address.
	// go-oidc downloads them on first use, caches them and refetches on key rotation.
	keysURL := config.Issuer + "/protocol/openid-connect/certs"
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), &keysClient), keysURL)

	verifier := oidc.NewVerifier(config.Issuer, keys, &oidc.Config{
		ClientID:             config.Audience,
		SupportedSigningAlgs: []string{oidc.RS256},
		Now:                  config.Now,
	})

	return &Verifier{config: config, client: &keysClient, oidc: verifier}, nil
}

// Close releases idle connections to Keycloak.
func (v *Verifier) Close() {
	v.client.CloseIdleConnections()
}

// keycloakClaims are the extra token fields Keycloak adds.
type keycloakClaims struct {
	Type          string `json:"typ"`
	Client        string `json:"azp"`
	EmailVerified bool   `json:"email_verified"`
	NotBefore     int64  `json:"nbf"`
	RealmAccess   struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Verify checks a raw access token and returns the caller's identity.
func (v *Verifier) Verify(ctx context.Context, rawToken string) (Identity, error) {
	if rawToken == "" {
		return Identity{}, ErrToken
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// go-oidc checks the RS256 signature, exact issuer, audience and expiry.
	token, err := v.oidc.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, ErrToken
	}

	var claims keycloakClaims
	if err := token.Claims(&claims); err != nil {
		return Identity{}, ErrToken
	}

	// Only access tokens; ID and refresh tokens use other "typ" values.
	if claims.Type != "Bearer" || token.Subject == "" {
		return Identity{}, ErrToken
	}

	if !slices.Contains(v.config.Clients, claims.Client) {
		return Identity{}, ErrToken
	}

	// Reject tokens that are not valid yet, allowing 30 seconds of clock difference.
	if claims.NotBefore > v.config.Now().Add(30*time.Second).Unix() {
		return Identity{}, ErrToken
	}

	return Identity{
		Issuer:        token.Issuer,
		Subject:       token.Subject,
		Client:        claims.Client,
		Roles:         claims.RealmAccess.Roles,
		EmailVerified: claims.EmailVerified,
		Expires:       token.Expiry,
	}, nil
}

// skipsTLSChecks reports whether the client turns off certificate verification.
func skipsTLSChecks(client *http.Client) bool {
	transport, ok := client.Transport.(*http.Transport)
	return ok && transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify
}
