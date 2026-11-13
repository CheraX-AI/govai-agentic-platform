# `authn` — Shared Keycloak Authentication & Authorization Package

`authn` is a reusable, self-contained Go package providing **OpenID Connect (OIDC) access token verification** and **Gin HTTP middleware** for GovAI backend microservices.

It validates Keycloak RS256 Bearer JWTs, enforces security boundaries, and provides role-based and email-verification authorization guards without storing administrative secrets, passwords, refresh tokens, or user profiles.

---

## Architecture Overview

```
Client Request (HTTPS)
       │
       ▼
[ Authorization: Bearer <token> ]
       │
       ▼
┌────────────────────────────────────────────────────────┐
│               Gin Middleware Pipeline                  │
│                                                        │
│  1. verifier.Authenticate()                            │
│     ├── Verifies HTTPS is active (426 if plain HTTP)   │
│     ├── Extracts single Bearer token                   │
│     ├── Validates signature via Keycloak JWKS (RS256) │
│     ├── Validates issuer, audience, client, not-before │
│     └── Stores sanitized Identity in Gin context      │
│                                                        │
│  2. authn.RequireVerifiedEmail() (Optional Guard)      │
│     └── 403 Forbidden if email_verified is false       │
│                                                        │
│  3. authn.RequireRole("citizen" | "officer") (Guard)   │
│     └── 403 Forbidden if role is missing               │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
                    Route Handler
             (authn.Current(c) -> Identity)
```

---

## Package Files & Structure

| File | Purpose | Key Types & Functions |
| :--- | :--- | :--- |
| [`config.go`](config.go) | Configuration definitions and safety validation rules. | `Config`, `(c Config) validate()` |
| [`verifier.go`](verifier.go) | Core token verifier using `go-oidc`. Manages JWKS remote keysets and JWT claim validation. | `Verifier`, `New(...)`, `(v *Verifier) Verify(...)`, `(v *Verifier) Close()` |
| [`identity.go`](identity.go) | Safe user identity model and request context extractor. | `Identity`, `Current(c *gin.Context)` |
| [`middleware.go`](middleware.go) | Gin HTTP middleware handlers and HTTP response helpers. | `Authenticate()`, `RequireVerifiedEmail()`, `RequireRole(role)`, `bearerToken()` |
| [`errors.go`](errors.go) | Package sentinel error definitions. | `ErrConfig`, `ErrToken` |
| [`authn_test.go`](authn_test.go) | Full test suite using mock TLS Keycloak server and custom RSA keys. | Unit tests & race condition tests |

---

## Security Guarantees & Features

1. **Strict HTTPS & Header Verification:**
   * Rejects unencrypted plain HTTP requests with `426 Upgrade Required`.
   * Accepts tokens exclusively via a single `Authorization: Bearer <token>` header. Cookies, query strings, and custom forwarded headers are ignored.
2. **Cryptographic Validation (RS256):**
   * Validates digital signatures against Keycloak's public keys fetched from `<Issuer>/protocol/openid-connect/certs`.
   * Uses `go-oidc` with automatic public key caching and key-rotation handling.
3. **Strict Claim Validation:**
   * **Token Type:** Requires `typ == "Bearer"`. ID tokens and refresh tokens are rejected.
   * **Issuer & Audience:** Must exactly match the configured `Issuer` URL and `Audience`.
   * **Authorized Party (`azp`):** Must belong to the configured `Clients` allowlist.
   * **Timing Bounds:** Enforces expiration (`exp`) and not-before (`nbf`) with a 30-second clock skew tolerance.
4. **Data Privacy & Redaction:**
   * The raw JWT token string is never placed in the Gin context.
   * `Identity` contains only essential claims (`Subject`, `Client`, `Roles`, `EmailVerified`, `Expires`).
   * `Current(c)` returns an isolated copy of user roles so downstream handlers cannot mutate the context.
5. **No Caching of Authenticated Responses:**
   * Automatically sets `Cache-Control: no-store` and `Pragma: no-cache` on authenticated and error responses.
6. **Safe Failure Responses:**
   * Failed authentication fails closed with a generic `401 Unauthorized` (`WWW-Authenticate: Bearer`).
   * Missing permissions return `403 Forbidden`.

---

## How to Use in a Sibling Microservice

### 1. Link the Package in `go.mod`

From your service directory (e.g. `backend/services/core-service`):

```bash
go mod edit -require=github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn@v0.0.0
go mod edit -replace=github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn=../../pkg/authn
go mod tidy
```

### 2. Initialize the Verifier at Startup

Create a single, long-lived `Verifier` during server initialization and reuse it across requests:

```go
package main

import (
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
)

func main() {
    // 1. Configure the verifier
    cfg := authn.Config{
        Issuer:   "https://auth.gov.lk/realms/GovAI",
        Audience: "govai-api",
        Clients:  []string{"govai-portal-web", "govai-mobile-app"},
    }

    // 2. Initialize with a trusted HTTP client (verifying TLS)
    httpClient := &http.Client{Timeout: 10 * time.Second}
    verifier, err := authn.New(cfg, httpClient)
    if err != nil {
        log.Fatalf("Failed to initialize token verifier: %v", err)
    }
    defer verifier.Close()

    // 3. Setup Gin router with authn middleware
    router := gin.Default()

    // Public route
    router.GET("/health", healthHandler)

    // Protected route group
    api := router.Group("/api/v1")
    api.Use(verifier.Authenticate()) // Requires valid Bearer token over HTTPS
    {
        // Requires verified email + specific role
        api.GET("/citizen/profile",
            authn.RequireVerifiedEmail(),
            authn.RequireRole("citizen"),
            getProfileHandler,
        )

        // Officer-only endpoint
        api.POST("/officer/review",
            authn.RequireRole("officer"),
            reviewApplicationHandler,
        )
    }

    router.RunTLS(":8443", "cert.pem", "key.pem")
}
```

### 3. Read Authenticated Identity in Handlers

Use `authn.Current(c)` to retrieve the authenticated caller's identity:

```go
func getProfileHandler(c *gin.Context) {
    identity, ok := authn.Current(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }

    // Access validated fields:
    userID := identity.Subject
    roles := identity.Roles
    clientID := identity.Client

    c.JSON(http.StatusOK, gin.H{
        "user_id": userID,
        "roles":   roles,
        "client":  clientID,
    })
}
```

---

## Testing

Run tests and race detection from this directory:

```bash
# Run all tests
go test -v ./...

# Run tests with race condition detector
go test -race ./...

# Run code vetting
go vet ./...
```

The test suite spins up an in-memory TLS Keycloak server and validates:
* Valid access tokens (RS256).
* Expired, not-yet-valid, and tampered tokens.
* Missing or wrong issuer/audience/client (`azp`).
* ID tokens and refresh tokens (rejected).
* Missing/malformed `Authorization` headers.
* Missing roles and unverified emails.
* Plain HTTP rejection (`426 Upgrade Required`).
* Unsafe configurations and TLS bypass rejection.
