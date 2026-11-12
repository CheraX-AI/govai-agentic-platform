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

## Docker images

Each service has its own `Dockerfile` and is deployed on its own. Build from this
`backend/` folder, because both services compile in the shared `pkg/authn`:

```powershell
docker build -f services/auth-service/Dockerfile -t govai-auth .
docker build -f services/core-service/Dockerfile -t govai-core .
```

Images contain only the compiled binary (distroless, non-root). `.dockerignore`
keeps `.env` files, keys and local state out of the build. Pass settings and
TLS files when the container starts, for example:

```powershell
docker run --env-file services/auth-service/.env -e API_TLS_CERT_FILE=/tls/api-cert.pem `
  -e API_TLS_KEY_FILE=/tls/api-key.pem -v ${PWD}/../infrastructure/keycloak/tls:/tls:ro `
  -p 5001:5001 govai-auth
```

Inside a container `localhost` is the container itself: Keycloak, PostgreSQL and
core-service URLs in the environment must use addresses the container can reach,
and the TLS certificates must be valid for those host names. Database migrations
are not run by the API image; apply them with `go run ./tools/db` as before.

Local server infrastructure is under `infrastructure/` at repository root. Authentication application schema remains in `services/auth-service/internal/postgres/migrations/`. See the infrastructure PostgreSQL README for the folder map.
