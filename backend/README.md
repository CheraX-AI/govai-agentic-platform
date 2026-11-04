# GovAI Go backend

Express has been retired. The Go/Gin foundation lives in services/auth-service.
See [the service guide](services/auth-service/README.md) for setup and tests.
Other service directories remain placeholders.

Reusable JWT/Gin authorization is in [pkg/authn](pkg/authn/README.md). Run its tests
separately from the auth-service module; the service uses a local module replace.

Authentication PostgreSQL setup: [local database guide](../infrastructure/postgres/auth/README.md).
It uses an isolated database, encrypted profiles, versioned migrations and a
restricted runtime user. [Design and verification](services/auth-service/docs/GA-48-postgresql.md).

The root README still describes the older Node backend; use this backend guide
for current instructions. The teammate-owned web app still uses Node/npm.

Local identity-environment setup is separate from the Go API. See
[Keycloak development step 1](../infrastructure/keycloak/README.md) for its isolated Compose
configuration, synthetic citizen setup, verification checklist, and runtime limits.

Local server infrastructure is under `infrastructure/` at repository root. Authentication application schema remains in `services/auth-service/internal/postgres/migrations/`. See the infrastructure PostgreSQL README for the folder map.
