# Shared Keycloak access-token middleware

Importable Go module for GovAI services, outside any service's `internal` tree.
It provides a maintained-library RS256 JWT verifier and Gin `Authenticate`,
`RequireVerifiedEmail`, `RequireRole` and `Current` helpers. It never holds an
administrative secret, password, refresh token or application profile store.

## Use from a sibling service

For a Go module under `backend/services/<service>`:

```powershell
go mod edit '-require=github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn@v0.0.0'
go mod edit '-replace=github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn=../../pkg/authn'
go mod tidy
```

The committed local replace resolves from the service's go.mod. Clone/build with
the backend shared module included; a Docker build context containing only the
service directory will need adjustment. No repository-wide go.work was added.
Publish/version this module separately if services later move to independent repos.

```go
verifier, err := authn.New(authn.Config{
    Issuer: "https://identity.example/realms/GovAI",
    Audience: "govai-api",
    Clients: []string{"govai-login-mobile-dev"},
}, trustedTLSClient)
if err != nil { return err }
defer verifier.Close()
router.GET("/protected", verifier.Authenticate(),
    authn.RequireVerifiedEmail(), authn.RequireRole("citizen"), handler)
```

Create a long-lived verifier per fixed configuration, not one per request.
`trustedTLSClient` must use the normal Go HTTP transport with certificate and
hostname validation enabled; a private CA can be added to its roots. Do not use
an opaque/custom TLS bypass transport. Put this middleware before protected
handlers and add each service's own ownership/business policy after it.

## Guarantees and limits

- Only a single Authorization Bearer header over direct TLS establishes identity.
  Cookies, query tokens and forwarding/user/role headers are ignored.
- Verifies signature/RS256, exact issuer, API audience, configured authorized
  parties, expiry, Keycloak `typ=Bearer`, subject and bounded iat/nbf/lifetime.
  ID/unsigned/wrong-party tokens are rejected. Roles and verified-email are checked
  separately after authentication, giving 403 for those policy failures.
- Claims are stored as a redacted identity; the raw token is never put in Gin
  context. `Current` returns a copy of roles. Services must deliberately allowlist
  any identity fields returned to the authenticated caller.
- Fixed issuer-origin `/protocol/openid-connect/certs` only; no caller URLs,
  discovery redirects or token introspection. TLS/redirect/header/64 KiB body
  bounds and a five-second client timeout protect JWKS retrieval.
- Valid keys are cached by go-oidc. Unknown-key refreshes are coalesced and limited
  to one attempt per five seconds by default, including failed attempts. Genuine
  rotation during this cooldown can temporarily return 401 until the next fetch
  is eligible; existing cached signing keys continue to work. This is deliberate
  bounded-provider-load behavior, not immediate key rotation propagation.
- Verification has a five-second caller deadline and a 32-request per-verifier
  concurrency cap; overload returns 503/Retry-After. Shared JWKS fetches may finish
  after an individual caller cancels, bounded by the HTTP client timeout.
- Invalid credentials and failed JWKS validation fail closed with generic 401;
  this API does not distinguish malformed tokens from an unavailable key provider.
- No live role/session/email-status introspection: a valid signed token represents
  its issue-time claims and can remain accepted until expiry after account/role/
  verification changes or logout. Revocation-aware behavior belongs to a separate
  design. Cached public keys do not make a provider status check.
- This module does not prove resource ownership, completed application profiles,
  NIC identity, RAG authorization boundaries or distributed abuse enforcement.
  Gin request-body/header logging must remain disabled/redacted in each service.

## Test

From this directory: `go test ./...`, `go vet ./...`, `go test -race ./...`.
Service-module `go test ./...` compiles this dependency but does not run its own
tests; run both modules in CI. Tests use controlled TLS servers/signing keys for
claims, forgery/algorithm rejection, cached keys, rotation/cooldown, missing-token
401, role/verified-email 403, direct-TLS enforcement and context identity safety.
