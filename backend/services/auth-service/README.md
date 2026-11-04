# GovAI Go/Gin authentication backend

## Current implementation

The local HTTPS service provides registration, mobile/Postman login, JWT/RBAC,
refresh/revocation and encrypted browser sessions. Features require explicit
configuration and remain disabled by default. Keycloak owns passwords and identity;
Go owns application profiles and session custody in a separate PostgreSQL database.

- [Registration API/runtime](docs/GA-48-registration-runtime.md),
  [HTTP contract](docs/GA-48-registration-http.md),
  [administrative adapter](docs/GA-48-admin-adapter.md), and
  [orchestration/persistence contract](docs/GA-48-registration-orchestration.md).
- [Application PostgreSQL](docs/GA-48-postgresql.md) and
  [combined database/Keycloak tests](docs/GA-48-postgresql-keycloak-integration.md).
- [Mobile/Postman login](docs/GA-48-login.md),
  [JWT/RBAC](docs/authorization.md), and [shared middleware](../../pkg/authn/README.md).
- [Refresh, browser sessions and logout](docs/GA-50-token-management.md),
  [Postman instructions](docs/GA-50-local-postman.md), and
  [rollout configuration](docs/GA-50-local-rollout-review.md).
- [HTTPS startup](../../../infrastructure/keycloak/HTTPS-SETUP.md),
  [application database setup](../../../infrastructure/postgres/auth/README.md), and
  [Keycloak PostgreSQL startup/rollback](../../../infrastructure/postgres/keycloak/README.md).

## Verification and remaining work

As of 2026-10-31, backend unit/vet/race checks and isolated PostgreSQL/Keycloak/TLS
integration tests passed. Registration coverage includes duplicates, atomic NIC
claims, profile failures, reconciliation, SMTP uncertainty and pool/service reopen.
Token/session coverage includes rotation/reuse, expiry, client isolation, encrypted
custody, concurrent claims, CSRF, local logout and durable revocation recovery.
These do not establish every process-kill or uncertain-commit recovery scenario.

User-confirmed local Postman results, also observed in request status metadata:
login/refresh 200, revoke 204, revoked refresh 401; web login/session/CSRF 200,
web refresh/logout 204 and session after logout 401. Schema readiness passed;
the observed recovery queue had no pending/running/attention jobs. These are
historical verification results, not a claim that services are always running.
After the Keycloak PostgreSQL cutover, the user additionally confirmed existing
account login returned 200 with an access token.

Remaining work:

- Review/merge and actual React/mobile registration, login, protected access,
  refresh and logout integration. Postman does not prove browser SameSite/CORS
  enforcement or mobile secure storage. Current web test origin is
  `https://localhost:5001`; deployed origins must be agreed with teammates.
- Real AI/RAG integration: protected `/api/v1/ai/chat` currently returns 501.
  Bearer `/me` and chat are not a complete cookie-authenticated web gateway.
- Operational reconciliation, broader crash/unknown-commit tests, deployment TLS,
  secret/key rotation, backups and distributed abuse controls. Rate limits remain
  process-local; maintenance commands have no scheduler/alert delivery configured.
- Password recovery/theme work remains deferred.

## Accepted prototype limitations

Custom form login uses HTTPS Go-to-Keycloak Direct Grant, with the RFC 9700
conflict documented in the [architecture/API plan](docs/GA-48-registration-plan.md).
HTTPS does not eliminate that protocol limitation. NICs are self-declared;
uniqueness is per normalized number, not proof of a unique or verified citizen.

The registration service account can manage other users within its registration
group, including credentials. This is accepted trusted administrative authority
for the prototype, not operation-level isolation. The original
[permission experiment](docs/GA-48-permission-experiment.md) and
[group experiment](docs/GA-48-group-permission-experiment.md) preserve the
FAIL_UNSAFE evidence for the stricter isolation requirement. Lifecycle and combined
harness documents retain reproduction instructions and their original test limits.

Logout ends local browser custody and attempts client refresh-token revocation;
it does not promise global SSO/device logout or immediate invalidation of offline
JWTs. Unknown refresh outcomes may leave a provider credential until expiry.
No complete production-security or full-story completion claim is made.

Commit source, tests, contracts, module checksums and reproducible setup files.
Keep real environment files, tokens, private keys, exports, dumps, logs and local
state outside Git. Frontend/mobile/Python/RAG files belong to their teammates.
## GA-91 password recovery — in progress

Agreed UI: enter old/new NIC, receive a reset link at the registered verified
email, display a masked email, and submit the new password in the custom app.
Masked email intentionally reveals account existence/domain and up to two local
characters; short local parts are entirely masked. NIC is self-declared, never
proof authorizing password changes. Email-access-loss recovery stays deferred.

Implemented components:

- `internal/recovery`: redacted 256-bit tokens, canonical parsing, hash-only storage
  boundary, transient password wrapper, masked-email helper and injected service.
- `internal/postgres`: optional migrations 004/005/006, immutable email/operation binding and one-way atomic token claims with
  database expiry; HMAC NIC lookup joins saved issuer/subject/operation ownership.
- `internal/keycloak/recovery.go`: saved-operation ownership and enabled/verified
  checks before recipient lookup, password update and separate session logout.

Service request order: lookup -> verified recipient -> challenge issue -> mail.
The configured HTTPS link carries the token in its fragment. Frontend must read
and remove the fragment, then submit the token in an HTTPS body; this backend does
not yet provide that page. Challenge metadata uses a dedicated HMAC key to bind
email and saved identity/operation. Reset claims first, rechecks the email binding,
revokes local sessions, resets the password, logs out provider sessions, fences local sessions again and records
completion. Unknown results stay spent/attention and must not retry the mutation.
SMTP failure attempts to invalidate the token; storage failure can prevent cleanup.

Verification: offline tests/vet and focused race checks passed. Real disposable
PostgreSQL tests passed expiry, migration replay, one winner across twenty claims,
replay rejection, permissions, old/new NIC and issuer/profile isolation. Synthetic
HTTPS/service tests passed ownership/account-state checks, payload/redaction,
callback restrictions, changed-email rejection, ordering and dependency failures.

Remaining before local activation: backup and review the local rollout; ensure all web instances use the guard. Operational storage/provider-outage reconciliation and deployed shared rate enforcement remain limitations. Offline JWTs may still
last until expiry. Migrations 004/005/006 are now applied to the local application database after explicit approval; recovery endpoints are enabled over HTTPS. No existing user password was changed during rollout.
API reference: https://www.keycloak.org/docs-api/26.8.1/rest-api/index.html
Recovery adapter evidence: a hash row and immutable operation/email-tag row commit
in one transaction. The service challenge adapter requires metadata on claim;
legacy hash-only rows cannot authorize service resets. Real PostgreSQL race tests
passed binding round-trip, concurrent claim/replay and denied runtime metadata
updates/deletes. The Keycloak provider wrapper fixes issuer to its configured realm.

`recovery.SMTPMailer` submits text emails without logging/persisting bearer links.
Remote servers require verified TLS 1.2+ STARTTLS; explicit plaintext development
mode accepts literal loopback addresses only and prohibits authentication. Synthetic
SMTP tests passed plaintext-loopback/trusted-TLS delivery, certificate rejection,
STARTTLS downgrade rejection, cancellation and header-input checks. These are not
yet live Mailpit or combined recovery lifecycle evidence. A post-DATA QUIT failure
does not retry an already accepted message. SMTP outcome ambiguity still requires
the pending durable recovery policy before enabling the route.
Recovery-aware web sessions: optional migration 006 stores HMAC identity cutoffs
and immutable cookie/login-order bindings. `RecoveryWebStore` rejects legacy
unbound cookies and direct unfenced creation. `websession.Start` detects this
store and captures a database sequence mark before calling provider login.
Identity-scoped transaction locks serialize reset cutoffs with cookie binding
and refresh completion. Load/CSRF/refresh check the cutoff; physical ciphertext
cleanup is separate. A second cutoff after provider reset/logout rejects logins
that started during the reset window. New logins after the final cutoff work.

All instances must use the same dedicated stable fence key and guarded store before
recovery routes are enabled. Mixed old/new instances can bypass revocation; enabling
the guard requires re-login for existing cookies. This is implemented but not wired
into local application composition/configuration. It does not invalidate offline
bearer JWTs or replace Keycloak logout. Isolated PostgreSQL race tests passed
cross-pool cutoff persistence, pending-login rejection, refresh/reset races,
identity isolation and legacy-cookie rejection. Web service tests confirm the
marker precedes password login and marker failure prevents the provider call.
### Recovery HTTP contract (opt-in runtime implemented; approved local setup enabled)

Paths registered only when recovery is explicitly enabled:

| Method/path | JSON body | Success |
|---|---|---|
| POST `/api/v1/auth/forgot-password` | `nic` | 202: `status: received`; matching accounts additionally return `masked_email` |
| POST `/api/v1/auth/reset-password` | `token`, `new_password`, `confirm_password` | 204 with no body |

Direct HTTPS and application/json are required. Unknown/duplicate/missing/wrong-type
fields, trailing JSON, invalid Unicode and oversized bodies are rejected. The
4096-byte request cap preserves bounded password/token input. Password confirmation
and local minimum bounds run before the token can be claimed; Keycloak enforces
its actual password policy. Recovery handlers return no cookies or raw credentials,
set no-store/no-cache/no-referrer and return only fixed error codes.

`NewLocalRecoveryHandler` configures 5/IP/15m plus one attempt/NIC/minute for request
and resend, and 10/IP/15m plus three attempts/token/15m for reset. Unknown accounts
also consume NIC cooldown. Counters are HMAC keyed, bounded to 4096 entries and
four concurrent requests per limiter. Only the direct socket source is trusted;
forwarded headers do not bypass limits. A limited request returns 429 and
`Retry-After` in seconds. These limits are process-local and reset on restart.
Browser Origins must match the configured web origin. Recovery CORS permits only
POST and Content-Type, exposes Retry-After/X-Request-ID and does not grant credentialed CORS. Postman requests may omit Origin.

Handler race tests passed HTTPS/origin/media boundaries, parsing, confirmation,
Unicode preservation, safe errors/panic handling, NIC normalization/cooldown and
IP/token limits. A 400 `invalid_or_expired_reset` is used for rejected challenges;
503 `recovery_unavailable` does not prove that no provider mutation occurred.
Unknown outcomes must not trigger automatic credential replay. Local Postman status evidence is now confirmed below; isolated combined evidence remains recorded separately.
### Isolated combined recovery evidence — 2026-11-01

`TestRecoveryLive` uses a fresh labelled Keycloak 26.8.1/Mailpit pair, private CA,
loopback-only HTTP/SMTP ports and a newly created PostgreSQL test database. Its
setup/observer is separate from the scoped registration/recovery actor; no broad
manage-users role is granted to that actor. Existing GovAI users/settings and the
existing application database are never targeted. The new SQL database is removed
on completion; test containers stop and their volumes/TLS state remain private.
Historical FAIL_UNSAFE group-authority verdicts are unchanged.

Final live run passed: actual registration and email verification, HTTP forgot
request/masked response, real Go SMTP/Mailpit reset-link delivery, persisted bound
token, HTTP reset 204, old password rejection/new password login, rejected old
provider refresh and guarded web cookie, confirmation/reuse/expiry rejection,
changed-email rejection without password mutation, real Keycloak password-policy
rejection, resend cooldown, unknown NIC response, and SMTP failure with spent
challenge. Structured logs contained no captured messages. The first run used an
incorrect expected HTTP status for the upstream token endpoint; correcting that
assertion to OAuth invalid_grant/400 was necessary, not a password implementation
change. The final expanded run also included SMTP failure and live resend cases.

Reproduce from the auth-service directory (Docker and private development SQL
settings required; creates isolated resources):

```powershell
$env:GA91_RECOVERY_LIVE = '1'
$env:GA91_RECOVERY_STATE_DIR = Join-Path $env:TEMP ('govai-ga91-recovery-' + [guid]::NewGuid().ToString('N'))
try {
    go test ./internal/permissionlab -run '^TestRecoveryLive$' -count=1 -v
} finally {
    Remove-Item Env:\GA91_RECOVERY_LIVE,Env:\GA91_RECOVERY_STATE_DIR -ErrorAction SilentlyContinue
}
```

The state directory must be new and outside Git; it contains private TLS material.
The recovery harness alone publishes its Mailpit SMTP port to literal loopback;
other harness modes keep their previous port configuration. Ordinary tests skip
this live test. A dummy HTTPS reset-page URL is inspected, never opened; real
frontend/deep-link navigation remains pending. The test exercises real guarded SQL
session storage, not a complete browser application or mobile UI. Offline access
JWT lifetime and accepted group-wide administrative authority remain limitations.
Unknown provider results and database outages still require operational recovery;
SMTP failure here is a closed local port, not every possible delivery ambiguity.
Runtime composition/configuration and route wiring are implemented and tested;
the approved local rollout below applied the optional migrations and enabled the routes.
### Runtime rollout (approved and applied locally on 2026-11-01)

`cmd/api` loads ignored `.env.recovery` before other local files. Defaults keep
recovery disabled. Enabling it requires registration/admin and web configuration,
the same issuer, API TLS, independent 32-byte base64 binding/fence/rate keys, an
HTTPS reset-page URL and approved web origin, and SMTP settings. See placeholders
in `.env.example`. Secrets are never supplied by public requests or printed.
Missing checksummed migrations or unsafe configuration prevent startup.

Before a separately approved rollout, stop the API, take a private SQL backup and
preserve existing environment keys. Explicit commands (from this service directory):

```powershell
go run ./tools/db migrate-recovery
go run ./tools/db check-recovery
```

Migrate-recovery adds optional migrations 004/005/006, preserving existing users,
profiles and passwords. Migration 006 **revokes existing active/refreshing browser
cookies once**. Enabling the guard rejects unbound cookies; users must log in again.
It uses the same stable fence key on every instance. `RECOVERY_ENABLED=false`
disables only reset routes while a configured fence key keeps the guard active.
Once migration 006 exists, web startup without that key fails closed. Do not remove
or regenerate the key or deploy a mixture of guarded and old instances as rollback.
To stop offering recovery, disable the routes and preserve guarded session handling.
Recovery-aware metadata cascades when an owner-authorized web-session cleanup
removes an expired parent; normal application roles cannot delete these rows.

The existing Mailpit container has only its inbox port published. A separate,
pinned, persistent reset-email sink is prepared so existing messages/container and
Keycloak verification SMTP settings stay untouched. From the repository root:

```powershell
docker compose -f infrastructure/keycloak/compose.recovery-mail.yaml up -d
```

It uses the built-in bridge (no new default address-pool allocation), SMTP on
127.0.0.1:1026 and reset inbox http://localhost:8026. Set recovery SMTP host to
127.0.0.1, port 1026, from noreply@govai.test, explicit dev plaintext true, no auth.
The original Keycloak verification inbox remains on 8025. Do not confuse the two.
The frontend reset page is not implemented here: local Postman tests can extract
the fragment token from the emailed link without pretending a working UI exists.

Latest checks: runtime config/redaction, disabled-route guard retention, route/CORS,
SQL readiness, full offline tests/vet and focused race suites passed. The updated
live harness also passed application recovery/guarded-web composition and all
previous lifecycle assertions. Repeated retained test bridges exhausted Docker's
default allocation pool; recovery-mode tests now use a fresh explicit /28 bridge,
with loopback-only published ports, preserving old resources. One intermediate
internal-network attempt did not expose the host test ports; the final bridge
configuration passed. Other historical harness modes retain their previous setup.
The user explicitly approved local activation after the preparation checks. A private
custom-format SQL backup restored into a separate isolated verification database;
profile and registration-journal fingerprints matched both backup and current data.
The verification database was removed afterward. Migrations 004/005/006 applied;
private recovery settings and key backup remain ignored. Existing users/passwords
were preserved; legacy browser sessions were revoked as documented. No commit/push occurred.
### Current local verification and next manual test

API runs on https://localhost:5001 with recovery and guarded browser sessions.
`go run ./tools/dev-auth recovery-check` checks fixed loopback readiness, invalid
recovery bodies and local-origin CORS using the trusted CA; it never sends a real
NIC/password/token or reads email contents. That check, login/session boundary
checks, SQL recovery readiness and the reset inbox API passed after rollout.
Existing GovAI inspection still reported five users. Full password-reset behavior is exercised by the isolated live harness. The user also completed local Postman reset/login/reuse checks below.

Send POST https://localhost:5001/api/v1/auth/forgot-password with application/json
and `nic` set to a registered, email-verified test account's NIC. A matching ready
profile returns 202 with masked_email; unknown accounts return received without a
mask. Check reset inbox http://localhost:8026. Its link currently points to the
configured local `/reset-password` URL; there is no frontend reset page here yet.
For Postman, copy the fragment token into the reset JSON body's token field and
submit new_password plus confirm_password to the reset endpoint. Never share the
link/token or add credentials to tracked Postman examples. Existing Keycloak
verification emails continue using the separate inbox on 8025. Real frontend/mobile
link navigation and deployed abuse controls remain pending.
### Local Postman results — 2026-11-01

The saved synthetic sample initially had an unverified email, so recovery returned
503 under the current fail-closed eligibility policy. `dev-auth recovery-sample-check`
reads only metadata for that exact generated test account. The explicit
`dev-auth recovery-sample-verification` action sent a fresh verification message
through Keycloak to its local inbox; it did not mark email verified or change a
password. The user completed verification before retrying.

Afterward, the user reported and agent-owned API request metadata confirmed:

| Request | Status |
|---|---|
| POST /api/v1/auth/forgot-password | 202 |
| POST /api/v1/auth/reset-password | 204 |
| POST /api/v1/auth/login with the new password | 200 |
| POST /api/v1/auth/reset-password using the same token again | 400 |

The exact 400 response-body code was not independently collected from the manual
request; deterministic and isolated live tests separately verify replay rejection.
No real user's password, raw reset token, link or credential payload was collected.
This manual test does not establish real web/mobile UI or deep-link integration,
distributed rate enforcement or production disaster-recovery readiness.
## Files and their purpose

| File | What and why |
| --- | --- |
| go.mod | Names this independent Go project (module) and records dependency versions, similar to parts of package.json. Gin is the HTTP framework. |
| go.sum (generated by go mod tidy) | Records dependency checksums for download verification. Commit with go.mod. |
| cmd/api/main.go | The executable entry point: loads configuration, starts HTTP, and handles shutdown. |
| internal/config/config.go | Validates PORT and the exact CORS origin allowlist before starting. |
| internal/config/config_test.go | Checks default/custom ports and invalid configuration. |
| internal/http/handlers/router.go | Maps URLs to handlers using Gin. c.JSON sends JSON to the caller. |
| internal/http/handlers/router_test.go | Simulates HTTP requests using httptest without opening a network port. |
| .env.example | Documents PORT and CORS_ALLOWED_ORIGINS; copy to .env for local loading. |

Go starts an executable at func main() in package main. The internal directory
keeps implementation packages private to this service tree. The small structure
keeps server lifecycle, settings, and HTTP handling separate without adding
unused service/repository layers.

## Install and run in PowerShell

Install Go 1.26 or newer from [official downloads](https://go.dev/dl/).
Reopen the terminal after installation and check `go version`.
Health-only mode needs Go. Enabled authentication also requires the configured PostgreSQL and Keycloak services.

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
owns validated settings. `internal/http/handlers` currently owns routes, health handlers
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

`tools/oidc-check` is a separate read-only diagnostic. It never runs during API
startup or health requests and does not change /health or /api/health. No admin
password, user password, client secret or token is needed. Existing local dotenv
loading and process-environment precedence apply.

From the repository root:

```powershell
cd backend/services/auth-service
$env:OIDC_ISSUER_URL = "http://localhost:8180/realms/GovAI"
$env:OIDC_CHECK_TIMEOUT = "5s"
go run ./tools/oidc-check
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

## Folder guide

| Location | Purpose |
| --- | --- |
| `cmd/api/main.go` | Start the authentication API; command remains `go run ./cmd/api`. |
| `internal/http/handlers/router.go` | Gin routes and shared HTTP request policies. |
| `internal/http/handlers/` | Registration, login, reset and session request handlers. |
| `internal/config/` | Environment settings and validation. |
| `internal/application/` | Assemble feature dependencies. |
| `internal/registration/`, `login/`, `recovery/`, `websession/` | Feature rules and workflows. |
| `internal/keycloak/` | Keycloak communication. |
| `internal/postgres/` | Application database operations. |
| `internal/postgres/migrations/` | Versioned SQL schema; existing migrations unchanged. |
| `internal/oidccheck/` | OIDC checks also used by configuration validation. |
| `tools/db/` | Explicit database checks/migrations. |
| `tools/dev-auth/`, `tools/oidc-check/` | Local setup and connectivity commands. |
| `tools/web-maintenance/` | Session-revocation queue maintenance. |
| `tools/permission-lab/`, `tools/keycloak-transition-check/` | Opt-in isolated experiments and transition checks. |
| `tools/internal/` | Implementation of development/experiment tools. |

Run tools from this service directory, for example `go run ./tools/db check`.
Tool arguments and environment requirements remain unchanged. Unit tests stay
beside their packages as `*_test.go` because they exercise private implementation
boundaries. Use the VS Code Testing view to navigate tests separately. No separate
routes package is introduced: router.go uses package-private handler policies.
Live Docker experiments remain opt-in; ordinary tests do not rerun them.