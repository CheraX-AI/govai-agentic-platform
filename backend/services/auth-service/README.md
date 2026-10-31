# Go/Gin foundation

The [mobile/Postman login API](docs/GA-48-login.md) now composes a separate
non-admin Keycloak client, strict access-token validation and owned PostgreSQL
profile checks. It returns only an access token; browser sessions and refresh/logout
remain deferred. Login is opt-in, with local settings kept outside Git.

[Registration runtime wiring](docs/GA-48-registration-runtime.md) is now opt-in:
`cmd/api` composes the real dependencies and exposes the JSON POST route only
with complete database/client settings and direct API/Keycloak HTTPS. It remains
disabled by default. The workstation now has an explicitly enabled local HTTPS
setup; follow [HTTPS startup and trust instructions](../../dev/keycloak/HTTPS-SETUP.md).

The [real PostgreSQL + Keycloak integration test](docs/GA-48-postgresql-keycloak-integration.md)
now passes with the real HTTP handler, registration orchestration, encrypted SQL
stores, HTTPS Keycloak and Mailpit. It verifies duplicate handling and durable
pool/service-reopen recovery. Its TLS HTTP listener is private to the test;
`cmd/api` registration/login are disabled by default and explicitly configured in
the local HTTPS environment.

Current database update: [PostgreSQL implementation](docs/GA-48-postgresql.md)
adds encrypted profile/journal adapters, atomic NIC uniqueness, migrations and
optional server database readiness. Follow the [local setup guide](../../dev/auth-postgres/README.md).
The earlier component reports below describe their original evidence; real
PostgreSQL tests and opt-in runtime registration/login now supplement them.

## Earlier component checkpoints (historical)

The descriptions below retain their original checkpoint context, including
statements that a component was not yet wired. Current behavior is described in
the runtime/login guides above; those later implementations supersede that status.

The opt-in [combined integration test](docs/GA-48-combined-integration.md) now
connects registration orchestration to the real group-aware Keycloak adapter in a
fresh HTTPS environment. It uses explicitly synthetic test-only stores; it does
not prove database durability, distributed concurrency or restart recovery.

The registration Admin adapter now requires a
[server-configured group path](docs/GA-48-group-adapter-config.md), using the exact
group-path array proven by the isolated Keycloak 26.8.1 tests. Missing/invalid
configuration fails before network access. No runtime loading or route is wired;
accepted group-wide authority and historical FAIL_UNSAFE verdicts remain unchanged.

The isolated [registration orchestration layer](docs/GA-48-registration-orchestration.md)
now uses injected profile/journal ports and the existing validation/Admin boundary.
Synthetic tests cover ordered commits, recovery, duplicates, concurrency and
credential exclusion. PostgreSQL adapters now implement these ports; no public
registration route is wired. See the database report for real test coverage and limits.

GA-48 prototype update: the user accepted the service account's group-wide
administrative authority. The [isolated registration lifecycle report](docs/GA-48-registration-lifecycle.md)
records password-policy, duplicate, verification-link and TLS results. Historical
permission-isolation FAIL_UNSAFE tests remain intact; no public route is enabled.
Persistence/profile ownership guarantees and real client integration remain pending.

The HTTP service provides liveness and optional database readiness checks. A separate read-only
OIDC discovery/JWKS connectivity checker exists; it does not authenticate users.
Public Go registration/login APIs, JWT authorization, RabbitMQ and AI chat
are not enabled. Optional PostgreSQL connectivity and readiness are implemented.
Keycloak-hosted registration/login configuration and
user-confirmed GA-48 manual results are recorded in the
[Keycloak development guide](../../dev/keycloak/README.md#ga-48-keycloak-hosted-registration-and-login).
Those results do not establish independent server-side validation tests or real
web/mobile integration. The service currently does not accept passwords.

GA-48 now selects custom registration through Go and the Keycloak Admin API,
and password login through Go to Keycloak's token endpoint (Direct Grant/ROPC).
This approved prototype decision conflicts with RFC 9700; it is not PKCE and HTTPS
does not remove its limitations. Both network legs require validated HTTPS outside
explicitly isolated local synthetic tests. No endpoints or settings are enabled.
Web BFF and mobile bearer-token custody are separate proposals; refresh/logout
remain deferred. See the
[draft registration plan and API contract](docs/GA-48-registration-plan.md) for
ownership, validation, partial-failure recovery, open decisions and implementation
steps. Strict request parsing and field validation (Unicode names, NICs,
international phones, new-account emails and unchanged transient passwords), plus
atomic number-reservation contracts, have synthetic tests. Uniqueness is per normalized NIC number, not per person; NIC data
is self-declared and unverified. No trusted old/new mapping is required. No
registration endpoint is enabled. Real PostgreSQL persistence adapters and
database-backed uniqueness now exist; combined live Keycloak/database registration
integration remains pending. In that design Go handles the
password transiently; Keycloak alone stores, hashes and verifies it.

The isolated [registration Admin API adapter](docs/GA-48-admin-adapter.md) now
supports disabled-user creation, operation-marker reconciliation, owned reads,
owned enabling and verification-email requests. It injects administrative config
and a verified-TLS HTTP client, caches service-account tokens in memory and returns
safe typed errors. It is not wired to the application or environment. Synthetic
tests do not establish live Keycloak permissions, policy or SMTP behavior.

The opt-in [GA-48 permission experiment](docs/GA-48-permission-experiment.md)
has now run against a separate Keycloak 26.8.1 instance. Both creation-capable
permission models allowed unrelated/admin account mutation; the owned-user scope
blocked creation. Public registration remains blocked on a narrower design.
The documented command creates only isolated test resources and never runs as part
of the normal API or unit test suite. Those baseline checks did not test SMTP or
the full lifecycle.

The [group-scoped follow-up](docs/GA-48-group-permission-experiment.md) demonstrates
creation/read/reconciliation/enable and delivery into a separate synthetic Mailpit
inbox with query-users plus three scopes on a fixed registration group. It still
permits changing other registration operations' users and passwords within that
group. This is group administration, not operation ownership; the release gate
remains unsatisfied. Verification completion and full lifecycle remain pending.

## Files and their purpose

| File | What and why |
| --- | --- |
| go.mod | Names this independent Go project (module) and records dependency versions, similar to parts of package.json. Gin is the HTTP framework. |
| go.sum (generated by go mod tidy) | Records dependency checksums for download verification. Commit with go.mod. |
| cmd/api/main.go | The executable entry point: loads configuration, starts HTTP, and handles shutdown. |
| internal/config/config.go | Validates PORT and the exact CORS origin allowlist before starting. |
| internal/config/config_test.go | Checks default/custom ports and invalid configuration. |
| internal/handler/router.go | Maps URLs to handlers using Gin. c.JSON sends JSON to the caller. |
| internal/handler/router_test.go | Simulates HTTP requests using httptest without opening a network port. |
| .env.example | Documents PORT and CORS_ALLOWED_ORIGINS; copy to .env for local loading. |

Go starts an executable at func main() in package main. The internal directory
keeps implementation packages private to this service tree. The small structure
keeps server lifecycle, settings, and HTTP handling separate without adding
unused service/repository layers.

## Install and run in PowerShell

Install Go 1.26 or newer from [official downloads](https://go.dev/dl/).
Reopen the terminal after installation and check `go version`.
Node/npm, Docker, and a database are not needed for this service.

From the repository root:

```powershell
cd backend/services/auth-service
go mod tidy
Copy-Item .env.example .env # Once; edit the copy as needed
go run ./cmd/api
```

Run Go commands from this directory: the module is here, not at backend root.
`go mod tidy` resolves dependencies and generates go.sum; `go run` compiles and starts
our program. The service loads .env from its working directory when present.
Existing process environment values take precedence over .env, including empty values. Malformed or unreadable files fail startup with a redacted error. Run from this service directory; parent/root .env files are never searched. Deployed environments should inject process settings and omit .env.
PORT defaults to 5000 when absent. It must be 1 through 65535; empty/invalid
values fail startup. Use 8080 if port 5000 is occupied.

In another terminal:

```powershell
Invoke-RestMethod http://localhost:5000/health
Invoke-RestMethod http://localhost:5000/api/health
```

Both return HTTP 200 with `{"status":"ok"}`. The second URL preserves the old
documented health path; the former Express response fields are not preserved.
Health is liveness only: it confirms this HTTP process responds, not database or
login readiness. Unknown URLs return 404 with a JSON error.

Press Ctrl+C in the server terminal. The server stops accepting new requests and
allows active requests five seconds to finish, then closes remaining connections.
Read/header/write/idle timeouts bound connection handling. Port-binding errors
exit nonzero. Lifecycle logs use the standard structured logging library.

## Format and verify

From this service directory:

```powershell
gofmt -w cmd internal
go test ./...
go vet ./...
```

`gofmt` applies Go formatting. `go test` runs all package tests (`./...` means all
packages below this module). Tests check health status, JSON content type and
body, plus port validation. `go vet` checks common coding mistakes. Tests should
print `ok`; successful vet normally prints nothing.

To run a built executable directly (useful for checking Ctrl+C on Windows):

```powershell
go build -o govai-api.exe ./cmd/api
./govai-api.exe
```

The generated executable is ignored by backend/.gitignore. Gin and godotenv are direct
external dependencies; configuration, lifecycle, and tests use Go's standard library.
Browser development origins default to http://localhost:5173 and http://127.0.0.1:5173. No teammates' files or shared Docker configuration were changed.


## Foundation policy and remaining gaps

Set `CORS_ALLOWED_ORIGINS` to a comma-separated list of exact HTTP(S) origins,
with no paths or wildcards. The defaults are local development settings; deployed
services must explicitly set their own origins. Set an empty value to disable
cross-origin access. Preflight permits GET and Authorization/Content-Type headers
for the current health-only API; extend deliberately when adding application routes.
Cookies/credentialed CORS are not enabled. CORS is a browser policy, not authentication.

404, 405, denied CORS, and panics before response commitment return JSON
`{"error":"safe message"}`. A panic after response commitment cannot replace the
response; it is logged and handler execution stops. Every request gets a generated
X-Request-ID. Structured request logs contain the route template, method, status,
duration and request ID; they exclude raw paths, queries, headers, bodies and panic
values. Request bodies are bounded to 1 MiB; future body-reading handlers must map
MaxBytesError to a safe JSON 413. Reverse proxies are not trusted. No rate limiter
is installed for these health-only endpoints; define limits before adding public
application routes.

Real `.env` and `.env.*` files are ignored; `.env.example`, go.mod and go.sum must
be included in the change. The application optionally loads the working-directory .env file.
The pre-existing shutdown path drains requests for up to five seconds and then
forces connection closure. Automated middleware/config tests do not prove OS signal
handling; check Ctrl+C with the built executable using the instructions above.

The requested HTTP foundation checklist is implemented. The broader phase 2 is
incomplete: PostgreSQL connectivity and dependency-aware readiness are absent.
There is no application persistence feature yet, so no migrations are needed.
Keycloak, RabbitMQ and web/mobile/Python integration remain separate future work.
Express retirement is authorized; the existing removal changes were preserved.


## Package boundaries for future development

`cmd/api` owns startup, dependency construction and shutdown. `internal/config`
owns validated settings. `internal/handler` currently owns routes, health handlers
and HTTP middleware together because the API is small. Extract `internal/middleware`
and `internal/routes` as reusable policies and route groups grow. Add
`internal/service` for application use cases and `internal/repository` for database
access when an actual persistence feature requires them. Handlers will receive
services through constructors; services will receive repositories and external
clients. Keep Keycloak verification, PostgreSQL, Redis, object storage, RabbitMQ
and the Python REST client behind separate injected boundaries when implemented.
No empty packages, connection stubs or infrastructure configuration are required
for the current health-only server. Gin JSON binding is available for future body
handlers; use ShouldBindJSON and return the same safe error envelope on failures.

## Recorded local verification

On 2026-10-26, gofmt, go test ./..., go vet ./..., go test -race ./...,
and go build succeeded. The built server ran on temporary port 15087 and
GET /api/health returned HTTP 200 with {"status":"ok"} and X-Request-ID.
Ctrl+C emitted both shutdown and server-stopped logs; the PTY wrapper reported
exit code 1 for the interrupted command, so this verifies the shutdown path ran,
not a clean shell exit code or draining under concurrent load.

## OIDC discovery and JWKS connectivity check

`cmd/oidc-check` is a separate read-only diagnostic. It never runs during API
startup or health requests and does not change /health or /api/health. No admin
password, user password, client secret or token is needed. Existing local dotenv
loading and process-environment precedence apply.

From the repository root:

```powershell
cd backend/services/auth-service
$env:OIDC_ISSUER_URL = "http://localhost:8180/realms/GovAI"
$env:OIDC_CHECK_TIMEOUT = "5s"
go run ./cmd/oidc-check
```

Alternatively put those settings into this service's ignored .env. Do not copy
Keycloak bootstrap credentials into the Go service. OIDC_ISSUER_URL is optional
for the API; when provided it is validated without contacting Keycloak. The
check command requires it. OIDC_CHECK_TIMEOUT defaults to 5s and must be positive,
at most 30s. This is one total deadline for discovery plus JWKS, not 5s each.

The realm identifier is case-sensitive: this user's realm is `GovAI`, so the
expected issuer is exactly http://localhost:8180/realms/GovAI. The earlier local
setup example used lowercase `govai`; use the actual realm identifier, not its
display name. Do not change realm data to make the check pass. Hostname, case,
port and trailing slash must match the provider's discovery issuer exactly.

The checker fetches issuer/.well-known/openid-configuration, compares its issuer
exactly, then fetches jwks_uri. Each JSON document is capped at 1 MiB. Redirects
are rejected. For this Keycloak integration, jwks_uri must use the same origin as
the configured issuer (a deliberately stricter policy than generic OIDC).
HTTPS uses normal certificate verification; HTTP is allowed only for localhost
or literal loopback IPs for development. URLs cannot contain user credentials,
queries or fragments. The configured issuer is trusted operator configuration,
never request input. There are no retries or credential forwarding.

Exit 0 reports matching discovery and a nonempty JWKS with key-type entries.
Exit nonzero reports a safe stage-specific configuration, HTTP, JSON, issuer,
connection/TLS, or deadline failure without raw URLs, response bodies, key
material or underlying transport errors. Check that Keycloak is running, the
realm spelling is exact, and its hostname setting agrees with the configured
issuer. A canceled/expired request does not leave background requests running.

This checks reachability and metadata shape only: it does not cryptographically
validate JWKs or JWTs, authenticate a citizen, establish API audience policy,
exercise login/refresh/logout, or enable authorization. Real JWT verification
will use a maintained OIDC/JWT library in a later phase. No PostgreSQL changes
are required. Keycloak availability remains independent from public liveness.

Run deterministic tests from this module with go test ./... and go vet ./...;
go test -race ./... checks concurrent HTTP test behavior. Tests cover exact
issuer mismatch, discovery/JWKS failures, size limits, invalid JSON, redirects,
unsafe JWKS origins, cancellation, offline providers, and bounded timeouts.

References: [Keycloak OIDC endpoints](https://www.keycloak.org/securing-apps/oidc-layers)
and [OIDC Discovery validation](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigValidation).

Verification on 2026-10-27: gofmt, go test ./..., go vet ./..., and
 go test -race ./... passed. The live command above passed against
http://localhost:8180/realms/GovAI: discovery matched and nonempty JWKS was
retrieved. Existing health tests passed; no live login, JWT validation, or
Keycloak outage manipulation was performed. No identity configuration changed.
