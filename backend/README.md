# GovAI Go backend

Express has been retired. The Go/Gin foundation lives in services/auth-service.
See [the service guide](services/auth-service/README.md) for setup and tests.
The [core service](services/core-service/README.md) now provides the GA-65 HTTPS bearer-protected chat API foundation on local port 5002; real Knowledge Agent integration is pending. Other service directories remain placeholders.

Reusable JWT/Gin authorization is in [pkg/authn](pkg/authn/README.md). Run its tests
separately from the auth-service module; the service uses a local module replace.

Both Go services use [pkg/chatcontract](pkg/chatcontract) for chat JSON and
validation. Run its tests from that directory with `go test ./...`.

Authentication PostgreSQL setup: [local database guide](../infrastructure/postgres/auth/README.md).
It uses an isolated database, encrypted profiles, versioned migrations and a
restricted runtime user. [Design and verification](services/auth-service/docs/GA-48-postgresql.md).

The root README and this guide describe Go/Gin. Node/npm remain tools for the teammate-owned web frontend.

Local identity-environment setup is separate from the Go API. See
[Keycloak development step 1](../infrastructure/keycloak/README.md) for its isolated Compose
configuration, synthetic citizen setup, verification checklist, and runtime limits.

Local server infrastructure is under `infrastructure/` at repository root. Authentication application schema remains in `services/auth-service/internal/postgres/migrations/`. See the infrastructure PostgreSQL README for the folder map.
