# GA-50: token management and logout

Status: in progress. Native and web runtime compositions are opt-in, tested in
disposable environments, and now enabled in the explicitly approved local GovAI
setup. Provider expiry/client-isolation/recovery tests passed. Local valid-account
Postman verification is now confirmed in API metadata; real client integration remains pending.

Current checkpoint: encrypted recovery persistence, bounded reconciliation and
operator commands and local provisioning/rollout are implemented; actual client
integration remain pending. Sections below record successive implementation
slices; later evidence supersedes earlier pending statements.

## Agreed client custody

Web and native clients share the GovAI realm but use distinct custody contracts.
The web backend will keep provider tokens encrypted server-side; the browser
receives only an opaque Secure, HttpOnly session cookie. Browser writes require
CSRF protection, exact origin checks and an appropriate SameSite policy.
Native refresh tokens belong in platform secure storage. No client secret is
distributed to either application. Existing prototype password-login decisions
and their documented RFC 9700 exception remain unchanged.

## Implemented transport slice

`internal/login/refresh.go` adds `Keycloak.Refresh` and `RevokeRefresh`.
Both use the existing fixed issuer/client and certificate-validating bounded
HTTPS client. Credential POSTs are not retried or redirected. Malformed input
is rejected before network access, provider descriptions are discarded and
tokens are redacted from default formatting, logging and JSON serialization.

Refresh requires an access token, refresh token, positive expirations and Bearer
type. This is transport validation only. Before accepting a refresh, orchestration
must verify the access JWT and current citizen/email/profile policy. A web refresh
must also retain its original issuer/subject binding. Provider rotation may already
have happened when validation, persistence or the network fails; do not blindly
retry an old token or claim that a failed response rolled back the provider.

Revocation uses RFC 7009 `/protocol/openid-connect/revoke`. A 200 is idempotent
and does not reveal whether the token existed. It is **not** evidence of global
SSO logout, termination on every device or immediate rejection of previously
issued offline-validated access JWTs. Full logout semantics still require live
tests and an explicitly defined session scope. The legacy refresh-token POST
logout format is deliberately not used.

## Evidence

`SessionService` now checks refreshed JWTs through the injected access verifier,
requires verified citizen claims and a ready issuer/subject profile, and caps the
returned access lifetime to the remaining JWT lifetime after the profile lookup.
`RefreshBound` additionally requires the original server-owned issuer/subject.
Native refresh uses the fixed provider's client/session binding. Both paths are
bounded to five seconds and never automatically retry. Failure results contain
no tokens and unexpected provider/storage details are replaced with safe errors.
Tokens rotated before a later failure are not automatically revoked: doing so
could affect concurrent legitimate users of the session. Recovery and serialized
web custody remain pending and must not silently replay the original credential.

Synthetic orchestration tests cover role/email/identity/expiry/profile failures,
database outages, cancellation, safe error mapping and remaining-lifetime limits.

Synthetic TLS tests cover fixed credential forms, successful rotation response,
invalid grant, client failure, missing refresh token, response size limits,
redaction, cancellation, local input rejection and idempotent revocation response.
These tests do not prove live Keycloak rotation, reuse detection or expiry.

## Remaining implementation

- Review and enable the native runtime configuration in the intended environment.
- Opt-in native refresh response and protected-by-credential refresh/logout APIs,
  with strict parsing, no-store responses, limits and safe errors.
- Durable encrypted web-session migration/store, hashed opaque session IDs,
  session fixation prevention, CSRF, exact origin policy and cookie lifecycle.
- Serialized web refresh, rotation failure/restart recovery and logout cleanup.
- Reviewed client/realm refresh settings without silently changing other clients.
- Disposable Keycloak/PostgreSQL lifecycle tests for refresh, invalid/expired/reused
  tokens, logout scope, session expiry, concurrency and residual access lifetime.
- Real web/mobile integration by their owners. Frontend/mobile files remain untouched.

Current running GovAI login remains access-only, with refresh issuance disabled
in its existing local client. No GovAI realm, database schema or running server
settings were changed. Disposable test settings are described below.

## Opt-in native HTTP routes

`LOGIN_REFRESH_ENABLED=true` requires `LOGIN_ENABLED=true` and a reviewed
refresh-enabled Keycloak client. It changes native login responses to include
`refresh_token` and `refresh_expires_in`, and composes:

- POST `/api/v1/auth/refresh`
- POST `/api/v1/auth/revoke`

When disabled, login retains its access-only response and these two routes are
not registered. The setting does not change Keycloak configuration automatically.
Do not enable it against the existing refresh-suppressed client. The scope of
realm-wide rotation settings must be reviewed before changing real environments.
The revocation route is deliberately not named logout: global SSO logout and web
session cleanup are not implemented. No new client secret is shipped to apps.

`NativeSessionHandler.Refresh` and `.Revoke` accept POST UTF-8 JSON containing
exactly `refresh_token`. The parser rejects duplicate/unknown/wrong-type fields,
trailing JSON, invalid Unicode, invalid credential characters and oversized input.
The handlers require direct TLS, reject browser Origin/cookie requests, bound
body reads and execution to five seconds, acquire an injected source/concurrency
limiter and return `Cache-Control: no-store` / `Pragma: no-cache` on all paths.
Origin absence alone grants no authority: the refresh credential is required.

Refresh returns explicit token fields only on success. Rejected credentials give
401; provider/internal failures give generic 503; malformed requests give 400,
media failures 415, body limits 413, and source limits 429 with Retry-After.
Revocation returns empty 204 for provider-confirmed success. This is credential
revocation, not a completed global logout implementation. Native callers must
clear their secure token storage; previously issued offline JWTs may remain valid.

Synthetic HTTP tests cover these boundaries, safe failure responses, cancellation,
no cookie issuance and rejection before service calls. Rate counters are process
local, not distributed. Native refresh/revocation share a separate source limiter
(60 attempts/minute, 8 concurrent, 4096 source buckets) without consuming login
attempts. Token values are never used as limiter identity keys.

## Live evidence — 2026-10-31

`TestRefreshLive` passed against disposable Keycloak 26.8.1, Mailpit and an
isolated temporary PostgreSQL test database. The disposable realm enabled
`revokeRefreshToken=true`, `refreshTokenMaxReuse=0`, and its own login client's
`use.refresh.tokens=true`. No existing GovAI realm/client settings were changed.

Verified through the real composition and HTTPS router:

- Native login emits refresh tokens only in opt-in mode.
- Refresh returns a different refresh token and validated access token.
- Reusing the original refresh token is rejected with 401.
- A fresh login's refresh token can be revoked with 204; refreshing it is then 401.
- An invalid refresh token is rejected with 401.
- Existing access-only login, verified citizen/profile policy and protected routes
  still work in the same lifecycle run.

The initial native run did not prove expired refresh rejection or cross-client
rejection; later combined tests below do. Whole-SSO termination, restart recovery,
concurrent provider refresh handling and actual web/mobile
integration. Disposable containers were stopped and their storage retained; the
temporary test database was cleaned up by the existing isolated harness.

Run from the auth-service module with Docker and the isolated test PostgreSQL up:

```powershell
$env:GA50_REFRESH_LIVE = '1'
$env:GA50_REFRESH_STATE_DIR = Join-Path $env:TEMP ('govai-refresh-live-' + [guid]::NewGuid().ToString('N'))
go test -run '^TestRefreshLive$' -count=1 -v ./internal/permissionlab
```

State contains private disposable credentials: keep it outside Git and do not
share its contents. A successful transport revocation does not imply immediate
invalidation of access JWTs accepted offline.

## Web persistence slice — not wired to runtime

`002_web_sessions.sql` is a separate optional migration. The unchanged registration
migration/checksum remains the baseline. `MigrateWebSessions` is explicit and
transactional with a checksum and migration advisory lock; API startup does not
call it. `WebSessionsReady` verifies both the baseline and web schema plus runtime
privilege restrictions. No migration has been applied to the existing GovAI database.

`WebSessionStore` requires a dedicated 32-byte AES-GCM key. Runtime configuration
and cross-key independence checks remain to be wired. Issuer, subject, access and
refresh tokens and CSRF secret are encrypted with a fresh nonce and authenticated
row binding. The cookie is a random 256-bit opaque ID; only its domain-separated
SHA-256 digest reaches SQL. Sensitive structs redact default JSON, formatting and
structured logs. SQL contains state, revision and absolute expiry as metadata.

Refresh claiming atomically changes active to refreshing before the provider call.
Only one instance wins. Completion requires the claimed revision, unchanged
issuer/subject/CSRF binding and unexpired, still-refreshing state. Termination
increments revision and marks revoked, preventing a late refresh from reviving it.
An ambiguous refresh or process restart leaves a non-active checkpoint; no loader
automatically replays the old provider token. This deliberately fails closed and
may require signing in again. Automated reconciliation is not implemented.

Local termination is idempotent and does not itself revoke provider credentials.
Encrypted ciphertext is retained pending a defined revocation/retention policy;
expired/revoked cleanup, key rotation and bounded provider-revocation recovery
remain required before enabling the web routes. Session lifetime is absolute
(maximum 24 hours at creation), cannot be extended by the runtime SQL role, and
must later be capped by the chosen Keycloak session policy in orchestration.

Real disposable PostgreSQL race-enabled tests passed for migration replay,
restricted privileges, encrypted storage, wrong-key rejection, independent store
instances, concurrent claims, pending-state replay rejection, identity binding,
rotation commit, stale completion rejection, revocation race and absolute expiry.
This verifies storage semantics, not a complete browser/session lifecycle or an
OS/process crash campaign. The test database was removed by isolated cleanup.

```powershell
$env:GOVAI_DB_TEST = '1'
go test -race -run '^TestWebSession' -count=1 -v ./internal/postgres
```

## Web lifecycle service — not routed

`internal/websession.Service` now composes injected login, token verification,
profile lookup, rotation and durable storage. Start generates a fresh CSRF secret
and a new opaque session identity; it returns no provider tokens. Absolute session
expiry is capped by configured lifetime and the initial refresh lifetime.
Access/refresh expiry metadata is encrypted alongside credentials. Session reads
verify access JWT identity, citizen/email claims and profile; reads never trigger
an implicit refresh. Expired access requires an explicit CSRF-protected refresh.

Refresh verifies the secret in constant time before claiming storage, pins the
original issuer/subject and commits rotated credentials before reporting success.
Provider/commit ambiguity leaves the checkpoint non-active with no automatic
retry. Local logout is applied before provider revocation, including pending
refresh sessions; provider failure does not restore local access. A pending
refresh that already reached the provider may still create credentials after
local logout. Its commit cannot revive the local session, but complete provider
revocation/reconciliation for that race remains unimplemented. No global SSO
termination guarantee is made.

Synthetic service tests passed for login custody, lifetime bounds, profile-based
access, CSRF rejection before provider calls, rotation commit, expiry, pending
replay rejection and local logout during provider failure. Updated encrypted
payload tests also passed with the real disposable PostgreSQL race test.
These are component tests; combined web lifecycle/Keycloak evidence is pending.

The initial storage slice used one safe error class; the later operational
classification section below supersedes that behavior. Failed login persistence may leave an unused provider session
until expiry; bounded cleanup/reconciliation is also pending.

## Browser HTTP adapters — not routed

`WebSessionHandler` now provides login, session check, CSRF retrieval, explicit
refresh and logout handlers. Construction requires one exact HTTPS frontend
origin. All requests require direct TLS and `X-GovAI-Web: 1`; mutation requests
also require that exact Origin. Foreign/duplicate origins, cross-site Fetch
Metadata and bearer-header/cookie mixing are rejected. A read without Origin
requires the custom header; runtime CORS must allow only the configured frontend
origin with credentials. The current generic router must not be used unchanged
to enable cross-origin browser sessions.

Login accepts the existing strict JSON credential contract and issues a fresh
`__Host-govai_session` cookie, Secure, HttpOnly, SameSite Strict, Path `/`, with no
Domain. Its JSON response contains only CSRF and expiry. Existing session cookies
must be logged out before switching accounts. Session check returns own subject;
CSRF retrieval restores the synchronizer token after reload, including after
access expiry, without granting access or refreshing tokens. Mutation handlers
require one correctly shaped `X-CSRF-Token`, take no request body, and rely on the
service's constant-time secret comparison. Duplicate session cookies are rejected.

Logout clears the cookie after local termination. If provider revocation fails
after local termination, it still clears the cookie and returns safe 503
`provider_revocation_pending`; it does not claim full provider logout. Reconciliation
for this outcome is pending. Errors before confirmed local termination do not
silently clear a potentially active cookie.

Handlers set no-store/no-cache, impose source/concurrency permits and bound
execution; login body reads have a deadline and existing parser size limits.
Runtime must compose dedicated browser-login abuse limits (including email
counters) separately from normal session traffic before enabling public login.

Synthetic HTTP tests passed for cookie flags, safe response fields, origin/CSRF
and fixation boundaries, duplicate cookies, media/body rejection and pending
provider revocation. These tests use injected services; real browser cookie/CORS
behavior and combined Keycloak/PostgreSQL lifecycle remain unverified.

## Opt-in web runtime

The API now loads ignored `.env.web` before the other local environment files;
process settings still win. `WEB_SESSION_ENABLED` defaults false. When enabled,
startup requires the optional migration/checksum and restricted runtime privileges
through `WebSessionsReady`, plus the baseline application profile store.

Required configuration: `WEB_ORIGIN` (one exact HTTPS origin with no trailing slash),
`WEB_ISSUER_URL`, `WEB_CLIENT_ID`, `WEB_CLIENT_SECRET`, `WEB_AUDIENCE`, optional
`WEB_CA_FILE`, `WEB_SESSION_DATA_KEY` and `WEB_RATE_KEY` (independent base64 32-byte
keys). Keys must differ from each other and profile/NIC/login/registration keys.
The web client must differ from native login and registration admin clients.
`WEB_SESSION_LIFETIME` defaults 30m, range 1m–24h, and actual custody is capped
by provider refresh expiry. The provider requires refresh issuance in this mode.
Local Keycloak client provisioning is still pending; no existing client is changed.

Routes enabled only in this composition:

- POST `/api/v1/web/login`: strict email/password JSON; returns CSRF and expiry,
  sets HttpOnly session cookie, returns no provider tokens.
- GET `/api/v1/web/session`: own subject only, requires a valid access JWT/profile.
- GET `/api/v1/web/csrf`: retrieve synchronizer token after reload.
- POST `/api/v1/web/refresh`: cookie plus X-CSRF-Token, no body, 204 on success.
- POST `/api/v1/web/logout`: same mutation protection; local termination and
  provider revocation attempt, with pending-revocation outcome described above.

All require `X-GovAI-Web: 1` and direct TLS; writes require exact WEB_ORIGIN.
The router gives web paths their own exact credentialed CORS policy, allowing only
Content-Type, X-GovAI-Web and X-CSRF-Token preflight headers. General CORS origins
cannot broaden web access. Frontend requests must use `credentials: 'include'`.
SameSite Strict requires a same-site HTTPS deployment (prefer same origin via
reverse proxy). Unrelated-site frontend/API hosting is not supported by this
cookie contract. Actual browser behavior remains to be tested with client owners.

Browser login has dedicated source/email limits of 5 attempts/15m; other session
traffic has 60 source attempts/minute. Each has 4096 buckets/8 concurrent capacity.
Counters remain process-local. No raw passwords/tokens enter rate-limit keys.

Optional schema commands are explicit, never run by API startup:

```powershell
# Only after reviewing/authorizing migration in the intended database:
go run ./cmd/db migrate-web
go run ./cmd/db check-web
```

Baseline `migrate`/`check` behavior remains unchanged. No command above was run
against the existing GovAI database in this work. Config tests, full service
tests, vet and handler/config/session race tests passed; CORS tests verified exact
origin isolation and rejected bearer authorization preflight. Combined live web
tests, provisioning, revocation recovery and real clients remain pending.

No private configuration, existing database or running server settings were changed.

## Combined web lifecycle evidence — 2026-10-31

`TestWebSessionLive` uses a separate confidential web client with no service-account
or administrative privileges, real Keycloak token validation/rotation, the real
encrypted PostgreSQL store and application composition, and a TLS HTTP test server.
The optional web migration is applied only to a newly created disposable database.

Verified:

- Login creates a Secure/HttpOnly/SameSite Strict cookie; HTTP JSON responses have
  no access, refresh or ID token fields.
- Cookie session accesses its own subject; missing CSRF and foreign origins fail.
- Refresh commits a different provider refresh token to encrypted storage with the
  original subject preserved.
- CSRF retrieval succeeds for page reload.
- Logout clears the cookie; replaying that cookie returns 401.
- The provider rejects the stored refresh token after logout revocation.
- The same run retains native rotation/reuse rejection and baseline login/RBAC checks.

This is a Go TLS HTTP client with a cookie jar, not a real browser. Cookie flags
are asserted, but browser SameSite/CORS enforcement, actual frontend/mobile flows,
whole-SSO logout and crash/reconciliation behavior remain pending. Later expiry
checks below cover expired provider tokens. A reconstructed application/server check exercises
existing persisted custody; it is not an OS-process restart or disaster recovery test.

```powershell
$env:GA50_WEB_LIVE = '1'
$env:GA50_WEB_STATE_DIR = Join-Path $env:TEMP ('govai-web-live-' + [guid]::NewGuid().ToString('N'))
go test -run '^TestWebSessionLive$' -count=1 -v ./internal/permissionlab
```

Disposable containers are stopped with storage retained. The harness cleans up
its temporary SQL database. Existing GovAI realm, users, Mailpit, private settings
and running server were preserved. No commit or push is part of this test step.

## Provider expiry and client isolation evidence — 2026-10-31

The expanded combined run passed in 35.52 seconds against Keycloak 26.8.1:

- A valid web refresh token was rejected by the native client.
- A valid native refresh token was rejected by the web client; it still refreshed
  successfully under its correct native client afterwards.
- After web logout/revocation, offline verification still accepted the unexpired
  signed access token. This is the documented residual lifetime, not a logout bug
  hidden by an immediate-invalidation claim.
- In the disposable web client only, access lifespan, client session maximum
  lifespan and client session idle timeout were set to five seconds. A real login
  confirmed both positive provider-issued lifetimes were at most five seconds.
  The harness waited the returned refresh lifetime plus two seconds. Access JWT
  verification then failed, and the provider rejected the refresh with invalid_grant.

No fabricated token or overridden clock was used for these provider expiry checks.
They prove rejection after that short test lifetime, not precise subsecond timing
or all combinations of SSO idle/max configuration. Long-duration idle, multi-device
logout and operational recovery remain separate concerns. Only disposable client
settings changed; real GovAI lifetimes and clients were preserved.

## Operational error classification and explicit cleanup

Missing/inactive/expired session rows (and malformed cookie identifiers) now return
an internal safe `ErrWebSessionMissing`, mapped to 401 by the web service. Database
errors and corrupt/wrong-key ciphertext instead map to generic 503, without SQL,
DSN or token details. Tests verify database outages do not trigger provider refresh
or falsely report a successful logout. Access remains denied in every failure case.

`PruneWebSessions` is a separate maintenance operation requiring the actual table
owner and migration checksum. Runtime application credentials cannot invoke it
successfully. It deletes only records whose absolute session expiry is older than
the retention period, with 24h minimum retention, at most 1000 rows per invocation,
five-second deadline and SKIP LOCKED selection. The CLI uses 24h and 500 rows:

```powershell
# Explicit maintenance only, after reviewing retention and the target database:
go run ./cmd/db prune-web
```

The command loads the separate migration/owner credentials, prints only a removed
record count and is never invoked by API startup or a background schedule. It was
tested only against a fresh disposable database; it was not run on existing GovAI.
Real PostgreSQL race-enabled tests confirmed the runtime role is rejected, recent
records survive, old expired records are removed in a bounded batch, and invalid
retention/batch settings fail. Profile and registration records are not targeted.

This is encrypted-record retention cleanup, not provider logout, token revocation
or complete refresh recovery. Deleting a row does not prove its provider session
ended. Provider retry/reconciliation, lost rotated-token handling, operator key
rotation/backup and a reviewed deployment-specific retention policy remain pending.

## Durable provider revocation recovery

`003_web_revocations.sql` adds an encrypted recovery queue without rewriting
migrations 001/002. Explicit `migrate-web` installs both optional migrations
transactionally; web readiness requires both checksums and restricted privileges.
No migration was applied to existing GovAI.

Web composition uses `BoundWebSessionStore`, fixed to its issuer/web client.
Local logout and a last-known refresh-token recovery record commit in one
transaction. Immediate provider success marks the job done and erases queued
ciphertext. Provider or acknowledgement failure leaves local access revoked and
the job durable. Tokens use fresh AES-GCM nonces and a separate AAD domain from
session payloads; jobs redact output and contain no password/raw cookie.

Claims lease one due record for 30 seconds. Concurrent instances cannot claim
the same live lease. Failure applies exponential backoff, with at most eight
attempts before attention state; an expired final lease also enters attention.
Completed records have empty token ciphertext. Job retention/monitoring is pending.

Failed refresh paths attempt a bounded termination/queue checkpoint. A known
rotated token is separately enqueued when a later local commit fails. Refresh
exchange itself is never retried. Persistent database outage, death before the
checkpoint or an unknown provider response can still leave a credential until
provider expiry. Complete distributed atomicity/global logout is not claimed.

`websession.Reconcile` is an explicitly invoked function, not a scheduled worker:
at most 20 records/30 seconds, five-second provider attempts and bounded queue
acknowledgements. Only idempotent revocation is repeated. An operator CLI entry
point remains pending. It must use the queue's same trusted issuer/client config.

Real PostgreSQL race tests passed for atomic termination, encrypted storage,
issuer/client isolation, concurrent leases, backoff, stale owners, retry caps,
completed ciphertext clearing and runtime DELETE denial. Synthetic reconciliation
tests cover provider/ack failure, cancellation and batch limits.

The combined live harness passed: normal logout completed its recovery record;
a simulated stop after atomic local logout but before provider contact left a
persisted job; a reconstructed store reconciled it; Keycloak then rejected that
refresh token. This is a checkpoint simulation, not an OS-process kill campaign.
Existing GovAI data/settings were preserved; no recovery pass ran against it.

## Operator commands and monitoring

After reviewed web configuration and optional migrations are installed, run from
the auth-service module:

```powershell
go run ./cmd/web-maintenance status
go run ./cmd/web-maintenance reconcile
```

These load the same ignored web/login/registration/database environment files as
the relevant server configuration, with process settings taking precedence. They
never load migration/owner credentials. Status displays only pending, running,
completed and attention counts for the configured issuer/client; it performs no
provider calls and exposes no token, subject, cookie or job identifier. Reconcile
uses the same trusted HTTPS client configuration as web runtime and prints only
completed/deferred counts. One invocation is bounded to 20 jobs/30 seconds.

Deferred jobs require another explicit pass after backoff. A running job beyond
its lease can be reclaimed on a later pass. Attention jobs have exhausted attempts
and require operator review; these commands do not silently reset or delete them.
No scheduler, metrics exporter or alert delivery has been configured. Check the
status command during prototype operation, particularly after provider outages.

Completed recovery metadata can be pruned separately with owner credentials:

```powershell
# Explicit maintenance after reviewing the target database and retention:
go run ./cmd/db prune-web-jobs
```

This removes at most 500 completed jobs older than 24h, within a five-second
deadline. It never removes pending/running/attention jobs, and the runtime SQL
role cannot invoke the deletion. Completed token ciphertext is already cleared
before this metadata cleanup. PostgreSQL tests verify client-filtered counts,
runtime deletion rejection and preservation of attention records during pruning.
The function accepts only bounded batches and at least 24h retention.

No operator command above was run against existing GovAI; configuration/provisioning
is still disabled there. Shared operator composition is exercised by the disposable
combined lifecycle harness, while the CLI itself is compiled/vetted by full checks.

References: [Keycloak OIDC endpoints](https://www.keycloak.org/securing-apps/oidc-layers),
[RFC 7009](https://www.rfc-editor.org/rfc/rfc7009),
[browser application guidance](https://www.ietf.org/ietf-ftp/rfc/rfc10017.html).


## Approved local rollout — 2026-10-31

Following explicit user approval, optional migrations 002/003 were applied to the
local authentication database and readiness passed. Private realm/client settings
and `.env.web`/`.env.login` backups were written under ignored
`.local/ga50-rollout-*` before Keycloak/client setting changes.

Created the separate non-admin `govai-login-web-dev` confidential client with
citizen role scope and audience/email-verification mappers. It has no service
account, standard flow or implicit flow. Web access lifespan is 300s, client
idle/max lifespan 1800s. The existing owned native client's secret was retained;
refresh issuance was enabled with 300s access lifespan. Realm rotation is now ON
with max reuse zero, which applies to other GovAI realm clients as approved.

Private `.env.web` and LOGIN_REFRESH_ENABLED are enabled. The approved Postman-only
web origin is `https://localhost:5001`. Existing users, registration permissions,
SMTP and profile data were preserved; read-only inspection confirmed five users.
No frontend/mobile/Python/shared Docker files were changed.

The owned old Go API stopped gracefully, and the new API started on HTTPS 5001
with registration, native login, JWT authorization and web-session routes enabled.
Local checks passed for readiness, native invalid-refresh rejection, idempotent
invalid-token revocation, web missing-session/foreign-origin rejection, generic
invalid login errors, protected-route 401, optional schema readiness and safe
empty recovery queue status. No local user was created or password changed.

These local checks deliberately did not use an existing user's password. The full
valid-account lifecycle remains proven in the isolated live harness; valid-account
local Postman confirmation remains for the user. Actual React/mobile integration
is not complete. Existing private files/backups remain ignored by Git. Nothing was
committed, pushed or merged.

See [local Postman verification](GA-50-local-postman.md) for the next manual step.

## Latest manual verification checkpoint

The local native and browser-cookie Postman flows were completed by the user and
confirmed by agent-owned request metadata. See [verification and remaining work](../README.md#verification-and-remaining-work) for
observed status codes, safe queue counts and remaining real-client scope. This
latest checkpoint supersedes earlier statements that local Postman evidence was
pending. No passwords, access/refresh/session/CSRF values were collected.
