# GA-48 custom registration and password-login architecture/API plan

Combined test update: [orchestration plus real adapter](GA-48-combined-integration.md)
now passes in the disposable HTTPS environment with test-only synthetic stores.
Profile failure, same-operation retries, marker reconciliation and SMTP failure
were exercised. Durable database/restart/concurrency guarantees remain unproven;
no public registration route is enabled.

Latest narrow update: the Keycloak adapter now requires
[server-only registration group configuration](GA-48-group-adapter-config.md)
and emits the pinned-version tested groups-path array. No route or infrastructure
setting is changed; durable persistence and combined live integration are pending.

2026-10-30 implementation update: an isolated
[orchestration service and exact persistence contract](GA-48-registration-orchestration.md)
now implement operation reservation, create intent/reconciliation, subject/profile
checkpoints, owned enabling and verification-request state with synthetic doubles.
No database adapter, worker or public route is enabled. This contract refines the
earlier persistence/retry draft below; ambiguous create/email requests never replay
blindly and no password comparison/digest is persisted.

Current prototype decision: the user accepts the documented administrative
credential's authority across registration-group members. The historical
permission tests still FAIL_UNSAFE against per-operation isolation. Isolated
[registration lifecycle evidence](GA-48-registration-lifecycle.md) now records
password policy, duplicates, email verification and TLS checks under that accepted
trust boundary. Public routes and durable registration remain unimplemented.

Revised 2026-10-29 for the team/mentor-approved in-app registration and password-login decision.
Strict request parsing, field validation, internal interfaces and synthetic
tests have been added; no registration endpoint or runtime configuration changes.
Future commits for this work must begin `GA-48:`. No commit is authorized here.
This explicit decision supersedes the repository's default hosted/PKCE design for
this plan. It does not authorize enabling Direct Access Grants or public routes.

## Repository findings and scope

- Current branch: `GA-18-US-C-01-citizen-registration-and-login`.
- Go/Gin module: `backend/services/auth-service`, Go 1.26, Gin 1.12.0.
  Only `/health`, `/api/health` and a separate discovery/JWKS diagnostic exist.
  There is no wired registration service, profile repository or journal. An
  isolated registration Admin API transport now exists; see its contract below.
- Existing HTTP foundation has request IDs, safe logs, deadlines, a body limit,
  exact CORS origins and no trusted proxies. CORS currently permits only GET;
  registration will need a deliberate POST/header extension and abuse controls.
- No profile SQL/schema, migration or database contract was found in the repository.
  Root Compose mentions PostgreSQL 16; that is not evidence of a usable profile
  schema or provisioned database. Core/KYC service directories are placeholders.
- Keycloak local image is 26.8.1, realm `GovAI`, issuer
  `http://localhost:8180/realms/GovAI`. Realm settings were operator-reported,
  not queried using administrative credentials during this inspection.
- Earlier hosted-registration evidence remains valid only for that tested path.
  It does not verify custom Admin API registration, its password policy, default
  roles, profile writes or email delivery.

Selected path: React/mobile form -> Go registration API -> Keycloak Admin API.
Go necessarily handles the submitted password transiently; Keycloak alone hashes,
stores and verifies it. No local password hash, replacement identity token or
credential database. No frontend/mobile/Python changes, SQL/provisioning changes,
forgot-password/reset feature, JWT middleware, refresh/logout or RabbitMQ security
emails are included in this slice.

## Proposed public contract

`POST /api/v1/registrations`, proposed unauthenticated JSON API over validated HTTPS.
HTTP is permitted only in explicitly isolated local synthetic tests, not merely
because a host is loopback. The transport policy below applies to both legs.
Required headers: `Content-Type: application/json`, `Idempotency-Key` (random UUID,
no PII). A client retains that key until the attempt has a terminal outcome.
No cookies or bearer tokens are required or issued. Body has exactly five required
string properties; reject unknown fields, duplicate JSON keys, nulls and trailing
JSON values:

| Property | Owner and proposed validation |
| --- | --- |
| `full_name` | App profile: required nonblank Unicode text, maximum 200 Unicode code points. Preserve the entire value, including spelling, combining marks and internal/outer spacing; no name splitting or Unicode normalization. Reject control characters and Unicode line/paragraph separators. |
| `nic` | App profile only. Required string: 9 ASCII digits plus V/X (case-insensitive suffix), or 12 ASCII digits. No whitespace/separator repair or digit conversion. Syntax acceptance is not validity or person verification. Uniqueness is per normalized number only; no old/new equivalence or identity verification is inferred. |
| `mobile_number` | App profile: international number with leading + and explicit calling code; maximum 64 input bytes. ASCII digits, spaces, parentheses, hyphens and dots only. Parse without a default region, require library validity, output E.164. Reject national-only/00 prefixes, extensions and vanity letters. Validation does not prove ownership, reachability, SMS capability or actual mobile service. |
| `email` | Keycloak identity: maximum 254 ASCII bytes, local part at most 64 bytes. Unquoted dot-atom local part plus a DNS domain with at least two labels, each at most 63 bytes. Lowercase the entire address for new accounts only. Reject surrounding/internal whitespace, Unicode addresses/domains, display names, quoted local parts, IP literals and trailing domain dots. Already ASCII punycode labels are permitted, with no Unicode/IDNA conversion. Keep plus tags and dots; do not perform provider-specific alias rewriting. |
| `password` | Transient Keycloak credential: nonempty string, maximum 1,024 UTF-8 bytes. Never trim, normalize, log or persist it. Whitespace-only and short passwords pass this local transport validation; Keycloak must enforce its authoritative realm password policy. No local minimum/composition/history/breach-policy guarantee. |

Implemented parser request cap: 16 KiB of JSON bytes, enforced before decoding
(including whitespace). Invalid UTF-8 and lone UTF-16 surrogate escapes are rejected
rather than silently replacing characters. Valid JSON escapes decode to the exact
Unicode string; password bytes are not otherwise transformed. Field limits above
are the current local validation contract. Validate before any identity mutation. NIC/full name/mobile are self-declared, not KYC evidence.

Proposed responses (all `Cache-Control: no-store`, `X-Request-ID`):

| Status | Meaning and body |
| --- | --- |
| 202 | Same response for a valid newly accepted request or existing-account conflict: `{"status":"received","message":"If registration can proceed, check your email for the next step."}`. Does not promise account creation or email delivery. |
| 400 | Malformed JSON, unknown fields, missing/invalid idempotency key: `{"error":"invalid request","code":"invalid_request"}`. |
| 413 / 415 | Body too large / unsupported content type, same safe error-envelope style. |
| 422 | Field syntax/policy rejection: `{"error":"invalid registration details","code":"validation_failed","fields":{"email":"invalid_format"}}`. Allowlisted codes only; no submitted values or raw Keycloak messages. Password rejection uses `password_policy`, without credential content. |
| 409 | Idempotency key reused with different non-secret registration details: `{"error":"registration request conflict","code":"request_conflict"}`. Never identifies the conflicting email or NIC. |
| 429 | Rate limit, safe error and `Retry-After`. |
| 503 | Dependency unavailable before durable acceptance, or unresolved outcome requiring same-key retry; generic `registration_unavailable` with `Retry-After`. Never imply a rollback happened. |

No public account-existence or operation-status lookup is proposed. Duplicate
email/username conflicts get the same 202 as accepted registration; do not email
an existing user or overwrite their password/profile. Avoid explicit NIC-conflict
responses too: the same normalized NIC number must not create separate accounts. Different old/new numbers are not mapped to a person.
Match response shape
and use a bounded minimum response time to reduce obvious timing enumeration;
this does not prove enumeration resistance. Test timing/abuse behavior separately.
422 is allowed only for independently established input/policy failures, not as
an existence oracle. Upstream errors whose cause cannot safely be classified get
a generic failure or reconciliation, never an unfiltered response.

An accepted request does not log in the user. After verification and completion
of required actions, the client uses the proposed Go password-login API below.
Client secrets stay out of web/mobile. No redirect URL, role,
`email_verified`, issuer, subject or administrative attribute is accepted in JSON.

## Password login: decision, API proposal and release gates

Registration uses the **Keycloak Admin REST API** to create an identity and set
its initial credential. Login uses **POST /realms/GovAI/protocol/openid-connect/token**
with form-encoded `grant_type=password`, canonical email as `username`, and the
unchanged transient password. Login must never use Admin API credential checks,
an administrator token, or local password verification. Keycloak alone verifies
passwords and issues identity tokens. Neither flow issues replacement Go JWTs.

This is Keycloak Direct Grant / OAuth Resource Owner Password Credentials (ROPC),
not Authorization Code with PKCE. [RFC 9700 section 2.4](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.4)
prohibits ROPC. The approved prototype decision conflicts with that guidance:
credentials reach the apps and Go, expanding theft/phishing exposure; browser SSO,
federated redirects, passkeys and interactive MFA/required-action flows are not
provided by this password exchange. HTTPS protects transport but removes none of
these limitations. Record this exception in review; hosted Authorization Code
with PKCE remains the migration recommendation, not the selected login flow.
The [pinned Keycloak 26.8.1 guide](https://www.keycloak.org/docs/26.8.1/server_admin/)
documents Direct Access Grants and client authentication; live behavior still
requires isolated tests. Existing clients remain unchanged in this planning step.

Proposed `POST /api/v1/auth/login`: strict JSON with exactly `email` and `password`,
same new-account email canonicalization and unchanged password transport rules as
registration, 4 KiB body cap, no caller client ID, scope, issuer, role or redirect.
Legacy username compatibility must be agreed before release; do not migrate users.
Use separate server-configured confidential login clients for web and mobile,
with no service-account/admin permissions. Their secrets stay in Go's secret
configuration, never apps. The Admin registration client must not allow password
grants; login clients must not have administrative credentials or privileges.
Use least scopes and an intended Go API audience; never request offline access
in this slice. No automatic retry of credential POSTs, including after timeouts.

Proposed safe responses, all no-store and with a request ID:

- 400/413/415: safe malformed-request/body/content-type errors without values.
- 401: identical `{"error":"Unable to sign in. Check your details and complete any required account actions.","code":"login_failed"}`
  for unknown accounts, wrong passwords, disabled/locked accounts, unverified email
  and pending actions. Do not forward Keycloak error descriptions or distinguish
  these cases publicly. No account-existence or required-action lookup endpoint.
- 429 with Retry-After: throttled; no credential forwarding after rejection.
- 503 with generic `login_unavailable`: network/TLS/timeouts, malformed upstream
  success or server/client misconfiguration; no raw dependency details or tokens.
- 200: only after validating the intended access token (signature/algorithm,
  exact issuer, API audience, expiry/time claims and nonempty subject), verified
  email and completed owned application profile. Success shape depends on the
  custody decision below; do not expose a universal raw Keycloak token response.

Require email verification before successful application login and protected chat.
Never clear VERIFY_EMAIL, UPDATE_PASSWORD, UPDATE_PROFILE, MFA enrollment or other
required actions to make Direct Grant work; never mark email verified from JSON.
Direct Grant cannot complete interactive actions. Refuse login generically and
provide the same static guidance to everyone to use approved Keycloak email/account
flows or support. Exact allowlisted action destinations/lifetimes and compatibility
with the pinned realm need owner review; no automatic action-email sending on
failed login. If Keycloak unexpectedly issues an unverified token, discard it and
deny access. Chat independently checks verified-email and citizen-role claims;
missing role is 403, invalid credentials are 401. Profile access uses validated
issuer/subject ownership, never caller email/NIC as authorization.

Proposed login limits: 5 attempts/15 minutes per source IP, 5/15 minutes per keyed
canonical-email digest, plus bounded global concurrency and expiring bounded state.
Apply before upstream calls, count failures, avoid logging digest keys, and test
shared-network fairness, victim lockout and bounded timing differences. Deployment
requires shared enforcement and reviewed Keycloak brute-force settings; no setting
changes here. Trust forwarding headers only from explicitly configured proxies.
Use a 5-second total login deadline and bounded upstream response size (64 KiB).
Cancel on disconnect; test outage recovery without retrying credential requests.

### Transport and sensitive-data invariants for both APIs

Require validated HTTPS on client -> Go and Go -> Keycloak (Admin, token,
discovery/JWKS and action requests) outside explicitly isolated local synthetic
tests. Validate certificate chains and hostnames with trusted roots; private CAs
must be explicitly trusted, never `InsecureSkipVerify`. Reject insecure configured
URLs and cross-origin/downgrade redirects; credential calls follow no redirects.
If TLS terminates at an ingress, re-encrypt and validate ingress -> Go too; do not
assume a private network authorizes plaintext. The current HTTP-only health/dev
setup is not ready to receive real credentials. Test untrusted/expired/wrong-host
certificates with controlled TLS servers and explicitly trusted test roots.

Passwords exist only during parsing and the bounded Keycloak call; no password
hash, digest, persistence DTO, journal, retry job or queue. Never copy credentials,
client secrets, tokens or action links into logs, traces, metrics, error responses,
screenshots or HTTP debug dumps. Disable body/header capture at ingress, Gin,
HTTP transports, APM and Keycloak admin-event representations. Token-bearing success
responses are the only proposed client transfer; failures contain none. Go memory
cannot guarantee zeroization. Credentials must be resubmitted after a failed
pre-create attempt; recovery workers never retain/reconstruct passwords.

### Separate custody proposals (not implemented)

- **Web:** prefer a Go BFF session with tokens only in bounded server memory,
  opaque rotated Secure/HttpOnly/SameSite cookie, session fixation protection,
  exact Origin checks and CSRF tokens for login and state-changing requests.
  No browser localStorage/sessionStorage or token JSON response. A process restart
  loses the session; re-login is required. Persistent token storage is not proposed.
  Cookie lifetime must not exceed accepted access-token lifetime in this slice.
  This requires a separate session/CSRF design and teammate agreement before wiring;
  do not accidentally combine bearer and cookie authentication.
- **Mobile:** propose access token plus token_type/expires_in only, retained in
  memory and sent as a bearer token to Go. No embedded administrative/login-client
  secrets and no stored password. Future durable credential custody must use platform
  secure storage following a separate reviewed refresh design. App suspension or
  expiration can require re-login for now. Bind the mobile response mode to a fixed
  server route/configuration, not an arbitrary caller-selected token destination.

Refresh/logout APIs and workers are not implemented or promised. Do not return or
persist refresh/ID tokens from upstream in this slice; discard unused token fields
without logging them. Review whether login clients can suppress refresh issuance
before release; discarding an issued token does not terminate its Keycloak session.
Account/session termination and client cleanup need a later design/test slice.
Offline-validated access tokens can remain usable until expiry after logout; no
immediate revocation guarantee. Web/mobile custody proposals must be selected and
tested independently before defining final success contracts or enabling routes.

### Smallest next implementation step and dependencies

The constructor-injected **registration Admin API transport adapter** is now
implemented with controlled httptest TLS servers, no route or runtime wiring;
see [the adapter contract and evidence limits](GA-48-admin-adapter.md).
The original narrow step was:
Keep it separate from a future password-token adapter. Test fixed URLs, mandatory
TLS validation/no redirects, deadlines/body caps, safe errors, credential redaction
and create response Location validation using synthetic secrets. This step needs
the reviewed adapter interface and pinned API request/response contract; it does
not need PostgreSQL or live client/settings changes. It cannot prove real Admin
permissions, password policy or durable recovery.

Later dependencies: owner-reviewed least-privilege registration credentials;
separate login-client configuration and explicit Direct Grant exception review;
trusted TLS endpoints; rate-limit infrastructure; agreed custody/session contracts;
SMTP/action configuration; database teammate's atomic profile/journal/NIC contract;
isolated live permission, duplicate, verification and pending-action tests. Add
deterministic login tests for generic error equivalence, required actions, invalid
tokens, unverified email, throttling and redaction before wiring any public route.
Do not provision PostgreSQL, modify Keycloak settings or enable endpoints now.

All approved field rules below remain intact: international phone normalization,
application-only NIC/name/mobile ownership, unique issuer/subject, and durable
uniqueness per normalized NIC number, without old/new person mapping. Phone-number
uniqueness/ownership verification remain unapproved separate requirements.

## Identity mapping and compatibility decisions

New-account policy: use the ASCII lowercased address returned by ParseRequest as
both Keycloak `username` and `email`, with duplicate emails disabled and Verify email
enabled. This deliberately treats ASCII email case as insignificant for new GovAI
accounts, while retaining provider dots and plus tags. Confirm the intended realm
user-profile/email settings and live duplicate behavior before wiring the adapter. Do not
automatically change realm-wide email-as-username/login settings: inspect existing
usernames, emails, verification states, federated users and collisions first.
Keep existing subjects and username login working until a migration is agreed.
Never join a new request to an existing account solely by matching email/NIC.
Existing users missing an app profile need a separate authenticated onboarding
contract; registration is not account recovery or account linking.

Email changes must not change the app identity key `(issuer, subject)`. Whether
username follows a changed email, and which email/username login modes remain
available to legacy accounts, need an explicit compatibility decision.

NIC/mobile/full name as on NIC belong only in the app profile, not Keycloak
attributes, tokens or Admin API logs. Inspect Keycloak user-profile requirements:
if first/last name are mandatory, resolve that configuration/contract with the
owner; do not fabricate a name split or silently duplicate the legal name.

New accounts receive the configured default realm `citizen` role. Verify actual
Admin API creation inherits it, and that default composites contain no privileged
roles. The public API does not assign roles. Never accept caller-selected groups.
Keep `emailVerified=false` and required action `VERIFY_EMAIL`; only Keycloak's
verification action may set verified status. API registration collects the password
before verification, unlike the earlier observed hosted sequence.

Decide how hosted self-registration is retired or restricted before releasing
custom registration: leaving it enabled permits identities without app profiles.
Do not disable it or alter existing flows in this planning step. Deployment must
explicitly handle such identities; a citizen role alone does not prove profile
completion. Email verification is still required for future protected chat.

## Keycloak integration and privilege boundary

2026-10-29 permission-gate result: the isolated
[live experiment](GA-48-permission-experiment.md) found that the current group-free
creation path required privileges permitting unrelated/admin user mutation.
Restricting the candidate to an owned subject blocked creation. This gate is
unsatisfied; review a narrower architecture before further dependent live work.

The later [group-scoped permission follow-up](GA-48-group-permission-experiment.md)
demonstrates functional group-bearing creation without manage-users, but its
service account can still change other operation-owned group members' credentials.
At the time of that experiment the application's adapter remained group-free.
It now supports the required server-only group path under the accepted prototype
trust boundary. Group membership still is not ownership by a registration operation.

Use a dedicated confidential service-account client in GovAI with client
credentials, no browser/direct-password grant, secrets supplied outside Git.
This administrative client is separate from all login-client configuration. Use fixed
configured URLs, normal TLS validation, bounded responses, no redirects and a
bounded token cache; never forward end-user Authorization headers.

Required capabilities: create a user with initial non-temporary password,
read/reconcile only registration-owned users, enable only those users after
profile completion, and request Keycloak verification email. Proposed endpoints:
`POST /admin/realms/GovAI/users`, user GET/PUT and
`PUT /admin/realms/GovAI/users/{id}/send-verify-email`. Validate the returned
Location against the configured Admin API origin/path; never follow arbitrary URLs.
Password setup is part of create, not a public reset-password operation.
Confirm policy rejection is atomic under the pinned version using a real test;
do not assume create-with-credentials validates exactly like hosted registration.

Prefer pinned-version fine-grained user permissions, scoped where possible to
registration-owned users, with explicit denial tests. Do not describe broad
`realm-management/manage-users` as create-only: it permits substantially more.
If create/reconciliation/email cannot be narrowly scoped in 26.8.1, record the
exact necessary permissions and residual power for review before enabling it;
do not silently grant realm-admin, manage-realm, manage-clients, impersonation,
role-mapping or master-realm privileges. No deletion permission is needed for the
proposed quarantine/reconciliation strategy. A future cleanup actor is separate.
Prove the service account cannot mutate legacy/admin accounts or grant roles;
unmet denial tests block enabling registration beyond isolated synthetic tests.

## Profile and operation persistence contract for the database teammate

Propose constructor-injected `ProfileStore` and `RegistrationJournal` boundaries.
Profile fields: issuer, subject, full name as on NIC, NIC, canonical mobile,
timestamps and lifecycle state. Enforce a unique `(issuer, subject)` and idempotent
write-by-registration-operation. No password/token fields. The approved requirement is one account per normalized NIC number, not per
verified person. No trusted mapping is required. Database-backed enforcement
remains pending; see the number-uniqueness contract below. Mocks cannot establish
the real guarantee.

Journal needs an opaque operation ID, idempotency key, keyed digest of canonical
non-password input, phase, owned Keycloak subject when known, attempts, next retry
and safe failure code. Never include password or password-derived digest. A
same-key replay with different password must not reset anything: first accepted
credential wins; a pre-creation failed attempt requires a new key to change input.
Persist pending profile data with appropriate access controls/retention so a
crashed process can resume without retaining credentials. Atomic app-side
profile/state updates, unique keys, leases/CAS for concurrent retries and a bounded
cleanup/retention policy must be agreed with the teammate. This is a proposed
logical contract, not SQL, an existing schema or permission to provision anything.

Until persistence is available, use synthetic in-memory doubles with deterministic
failure injection and an explicitly local test harness. Do not enable a live
registration endpoint backed by volatile state or accept real NIC/mobile data.
Durable recovery, distributed idempotency and duplicate-NIC guarantees stay unproven.

## Partial-failure and recovery state machine

No transaction spans Keycloak, the application store and SMTP. Propose:

1. Validate/rate-limit, normalize the supplied NIC number, then atomically reserve
   that number with the operation and pending profile input before Keycloak create.
   No trusted old/new mapping or identity-verification service is required. A
   conflicting reservation returns the generic 202 without creating another user.
   Store failure means 503 and no Keycloak mutation. Serialize same-key work.
2. Create a disabled Keycloak user with initial password, verified=false and
   VERIFY_EMAIL. Add only an internal random registration-operation marker,
   protected from caller edits, for crash reconciliation; verify user-profile
   configuration permits this marker. Persist no credential. No automatic retry
   of create on timeout.
3. Record returned subject then idempotently persist the profile by configured
   issuer plus subject. On profile failure keep the account disabled and retry
   profile work with bounded backoff. Never delete or update a pre-existing user.
4. After profile success, enable that owned account, retaining VERIFY_EMAIL.
   On ambiguous enable, read back and reconcile. A failed phase stays pending,
   with operational alerts; it does not masquerade as completion.
5. Request verification email through Keycloak SMTP only. Use a configured
   allowlisted client/redirect and explicit agreed action lifetime; no caller URL.
   Persist email-request state, retry transient failures with bounded backoff and
   a resend cap, then mark operator-attention after exhausted attempts. SMTP/API
   success is not inbox delivery or verification completion. Never queue links.

Crash after create but before subject recording: search using the protected
operation marker and reconcile an exact owned match; do not adopt a user just
because email matches. Zero/multiple/ambiguous matches remain quarantined for
operator review, without blind create retries. If create definitely never happened,
resume only when the client resubmits the password with the same operation key;
workers cannot reconstruct it. A completed operation replay returns the same
generic 202 and neither changes credentials nor triggers another email.

Journal failure after mutation leaves an uncertain outcome: stop further mutation,
return generic 503 if no acceptance can be confirmed, and reconcile owned state
on recovery. Ambiguous email send may cause a duplicate email; there is no
exactly-once SMTP guarantee. Existing identity operations are never rolled back
because SMTP fails. Unfinished owned accounts remain disabled until profile
completion; enabling prior to email delivery permits Keycloak's required-action
flow to offer verification. Test that no login completes without verification.
Stale-account retention/manual cleanup needs an agreed policy; no automatic
destructive compensation is proposed.

## Abuse prevention and handling sensitive input

Proposed starting limits for review: 5 attempts per 15 minutes per source IP,
3 per hour per keyed email digest, and bounded global concurrent registrations.
Use bounded limiter storage and expiry, count failed attempts, and return 429
with Retry-After. Shared mobile networks and deliberate victim throttling need
testing. A single-process limiter is local-only; deployment needs shared enforcement.
Keep trusted proxies disabled until exact proxy addresses are configured; ignore
spoofed forwarding headers. CORS permits exact web origins, POST and only required
headers; CORS does not protect against scripts/native callers. Native clients may
omit Origin. No ambient-cookie authentication or credentialed CORS is introduced.

Propose a 15-second registration deadline and 5-second per upstream-call cap;
review current 10-second server write timeout before wiring this flow. Cancellation
must stop foreground calls while persisted recovery work remains resumable by an
owned bounded worker. Never let a disconnected request launch untracked goroutines.

Log only request/operation IDs, phase, safe error codes, status and duration;
no request/response bodies, email, NIC, phone, legal name, passwords, action links,
client secrets, bearer tokens, token endpoint responses or raw Keycloak errors.
Redact reverse-proxy/APM traces and disable admin-event representation capture
where it could expose credential payloads. Secrets remain server-only. Passwords
are transient request data; Go memory cannot offer a guaranteed zeroization claim.
Restrict profile/journal access and decide sensitive-data retention with the owner.

## Small implementation steps after decisions are resolved

1. Agree the open decisions below and database interface contract. Keep historical
   hosted-test evidence separate. No runtime settings changed until reviewed.
2. Add registration DTO validation/strict parsing and service orchestration under
   `internal/registration`, handler under `internal/handler`, and interfaces/fakes
   under that boundary. Unit-test unknown role/identity overrides and all limits.
3. Add an injected bounded Admin API adapter under `internal/keycloak`, token
   custody/config checks and httptest error/timeout tests. Validate real isolated
   Keycloak password-policy, default-role and permission semantics before wiring.
4. Add synthetic profile/journal doubles and fault-injected state-machine tests,
   covering concurrent duplicates, every crash boundary, uncertain create, store
   outage, email failure/retry exhaustion and recovery. No PostgreSQL adapter yet.
5. Wire route/CORS/rate limits only in an explicitly synthetic local harness until
   durable profile/journal adapters and privilege gates exist. Test redacted logs,
   dependency failures and preservation of existing health behavior.
6. Run gofmt, go test ./..., go vet ./... and supported race checks from the module
   after code exists. Record isolated live tests separately: Admin API registration,
   7/8-character policy boundary, duplicate email/username, required email verification,
   Mailpit delivery, expired/invalid links and Direct Grant login/required-action
   refusal. Historical PKCE results do not prove this path. Mocks are not live evidence.
7. Coordinate database and actual web/mobile integration with their owners; report
   those checks pending until performed. Do not mark GA-48 production-ready.

## Decisions needed before dependent code

- NIC number uniqueness is approved; agree correction/reuse and retention rules
  with the database teammate. Identity remains self-declared and unverified.
  Duplicate-person prevention and old/new mapping are outside this requirement.
- Mobile normalization and name limits are now defined above. Mobile uniqueness
  and ownership verification remain separate decisions; SMS is not scoped.
- Email normalization for new accounts is defined above. Legacy collisions and
  username/email-change behavior remain integration decisions; do not migrate,
  rewrite or auto-link existing users. Issuer/subject stays the identity key.
- Disable/restrict hosted registration, or define how profile-less identities are
  onboarded without bypassing this registration path.
- Approve proposed response privacy, validation caps, limits, retention, email
  action lifetime and allowlisted verification return destination.
- Teammate profile/journal durability/transaction contract and feasible least
  privilege in the pinned Keycloak version. These gate recovery beyond doubles.

## Version-specific references consulted

- [Keycloak 26.8.1 Admin REST API](https://www.keycloak.org/docs-api/26.8.1/rest-api/index.html):
  user creation/update and verification-email operations. Endpoint availability
  alone does not prove this application's permissions or policy behavior.
- [Keycloak 26.8.1 administration guide](https://www.keycloak.org/docs/26.8.1/server_admin/):
  user profiles, service accounts, email, password policy and admin permissions.

## Approved NIC number normalization and uniqueness contract

The revised approved requirement supersedes the earlier mandatory trusted-mapping
proposal. Accept supported old/new NIC syntax and prevent separate accounts using
the **same normalized NIC number**. Do not require an identity resolver, provenance
record or old/new mapping to register. There is no mapping-dependent pending-review
state. NIC data is self-declared and identity-unverified.

- Old format: nine ASCII digits followed by V/v/X/x. Uppercase only the suffix.
- New format: twelve ASCII digits, retained exactly, including leading zeros.
- Reject whitespace, separators, Unicode lookalikes and malformed lengths; do not
  silently repair digits. Preserve the original accepted value separately.
- Keep V and X distinct. Never add/remove/rearrange digits or convert old numbers
  into new ones. Different normalized numbers remain distinct even if they belong
  to one person. This is duplicate-number prevention, not duplicate-person prevention.
- Format acceptance is not proof of issuance, checksum, date validity, NIC ownership,
  citizenship or authenticity. No KYC or DRP lookup is implemented or required here.

Earlier research remains background only: the [DRP FAQ](https://drp.gov.lk/en/faq.php)
explains ordinary conversion and number-change cases, and
[DRP verification guidance](https://drp.gov.lk/en/nicverification.php) describes
institutional verification. Neither is used to infer person equivalence in this
registration design. No new NIC source research was performed for this revision.

### Required database teammate contract and tests

`NICReservations` in `internal/registration/identity.go` is the Go boundary:

- `Reserve(nic, operationID)` uses `nic.NormalizedNumber()` as the uniqueness input
  across the GovAI citizen namespace, regardless of email or Keycloak subject.
  Atomically persist the number reservation AND durable journal operation before
  Keycloak creation. Enforce a database unique constraint covering both pending
  and completed attempts; no check-then-insert implementation.
- Same normalized number and operation is idempotent; another operation conflicts.
  Each operation is bound to one normalized number, so it cannot be repurposed.
- `Bind(nic, operationID, issuer, subject)` requires reservation ownership and
  atomically completes the pending profile/journal linkage. Enforce unique
  `(issuer, subject)` and one binding per normalized number. A conflict cannot
  overwrite or link an existing account. Neither method merges old/new numbers.
- Reject zero/unparsed NIC values, honor cancellation and return sanitized errors.
  Adapters must normalize consistently on every write path, not only HTTP requests.
- For sensitive lookup indexes, agree a deterministic keyed digest of the exact
  normalized number with a separately managed secret; do not use unsalted hashes.
  Digest/key rotation must preserve uniqueness across versions and existing claims,
  not create independent uniqueness domains. No digest adapter is implemented yet.
- Pending claims survive restarts, timeouts and SMTP failures. Uncertain Keycloak
  outcomes retain the claim until reconciled. Cleanup/reuse needs an explicit
  policy; no automatic release while an account may exist.
- Original NIC values belong in access-restricted encrypted application storage,
  never logs, tokens, public responses, queues or real-person test fixtures.
  Normalized numbers and their lookup keys are sensitive too. The API exposes none.

Teammate acceptance tests must use disposable PostgreSQL and independent
connections/processes: simultaneous registrations with the same number (including
lowercase/uppercase suffix variants) and different emails produce one durable
reservation and at most one authorized create operation. Same-key retries must be
idempotent; different old/new numbers and V/X suffixes remain separate; a reused
operation cannot switch numbers; duplicate subject binding fails; profile/journal
rollback is atomic; restart/crash/uncertain create retains the claim; index-key
rotation cannot bypass uniqueness. A process-local mutex is not database evidence.
No PostgreSQL provisioning, migrations or adapters are included in this work.

### Current narrow implementation and evidence limits

- `internal/registration/identity.go`: syntax-only NIC parser, normalized-number
  accessor, redacted formatting/JSON/log values and `NICReservations` interface.
  The old resolver, evidence requirement and person-key gate have been removed.
  Restricted raw accessors are for registration/storage adapters only; redaction
  does not excuse logging raw request data or accessor results.
- `identity_test.go`: syntax, original preservation, redaction, normalized-number
  concurrency/idempotency against a test-only mutex fake, distinct old/new numbers,
  V/X distinctions, no digit conversion, subject/operation conflicts, zero values
  and cancellation. There is no runtime in-memory registration store.
- Database-backed atomic uniqueness and concurrency remain pending until the
  teammate supplies real persistence and its tests. Synthetic tests prove neither
  durable uniqueness nor duplicate-person prevention. Do not enable real registration
  with volatile storage.
- Request JSON parsing and field validation exist as a library only. The isolated
  [Admin API adapter and contract](GA-48-admin-adapter.md) were added on 2026-10-29;
  registration handler and recovery flow remain unimplemented; remaining
  integration decisions above still apply. No KYC,
  password recovery, JWT middleware or client changes are included.

Previous checks on 2026-10-27 passed for the earlier mapping-based draft. They do
not verify this revision; final check results are recorded separately below.

Revised-number-contract verification on 2026-10-27: gofmt completed for both Go
files; go test ./..., go vet ./... and go test -race ./... passed from the
auth-service module with Go build-cache access. git diff --check passed. No real
PostgreSQL, Keycloak registration, SMTP or web/mobile checks were run. Database
uniqueness remains pending; these results cover only validation and synthetic
interface behavior.

## Request parser implementation and safe errors

`ParseRequest(io.Reader)` in `internal/registration/request.go` performs bounded
strict decoding and returns either a transient `Request` or a zero request plus
`RequestError`. It accepts exactly the five case-sensitive field names above.
Malformed JSON, unknown fields, repeated keys (including escaped equivalent keys),
invalid Unicode and trailing values yield `invalid_request`. Missing/empty fields
use `required`; null and non-string values use `invalid_type`. Field validation
uses only `invalid_format` and `too_long`. Field names are allowlisted and decoder,
reader and phone-library error text never reaches the caller. Oversized bodies use
`too_large`. Proposed HTTP mappings remain 400/413/422; no handler is wired.
Content-Type and Idempotency-Key validation remain future HTTP adapter work.

Request formatting, JSON serialization and structured logging redact the entire
request; its password is private and exposed only through TransientPassword for a
future credential adapter. These safeguards do not protect code that deliberately
logs raw bodies or accessor results. No logging, storage, queueing or password
verification occurs in the parser. No PostgreSQL/profile DTO contains a password.

Phone parsing uses the maintained Go libphonenumber port
[nyaruka/phonenumbers v2.0.14](https://pkg.go.dev/github.com/nyaruka/phonenumbers/v2@v2.0.14),
pinned in this backend module. Validity depends on its bundled numbering metadata;
review metadata updates over time. The dependency requires x/text v0.41.0 and
protobuf v1.36.11; backend go.mod/go.sum record these transitive updates. No teammate
lockfiles are changed. Tests use synthetic/example numbers, never send SMS/calls.

Email lowercasing is a new-account application policy compatible with submitting
one canonical value to both Keycloak fields; it is not a claim that every email
provider treats local-part case identically. Existing username/email login and
stored values are untouched. No account discovery, linking or migration is added.
Verification/delivery and actual realm compatibility require later live tests.

Table-driven tests cover Unicode names, preserved combining marks/spacing,
international phones, invalid formats, NIC normalization, email case/dot/tag rules,
password preservation (including spaces, controls and valid surrogate pairs),
field/body boundaries, wrong types, duplicates, unknowns, malformed/trailing JSON,
reader failures and redaction. Local validation does not cover all Keycloak rules.

Request-validation verification recorded on 2026-10-28: gofmt, go test ./...,
go vet ./... and go test -race ./... completed successfully from the auth-service
module; git diff --check passed. Only backend module dependencies and validation/
documentation files were changed for this step; prior uncommitted work remains.
No endpoint, Keycloak Admin API call, persistence, SMS/email delivery or real
client integration was exercised. Remaining work is the reviewed HTTP adapter,
rate limiting, Keycloak integration/recovery and durable profile/journal/NIC
uniqueness implementation with the database teammate and real integration tests.

Subsequent isolated HTTP step: see [GA-48-registration-http.md](GA-48-registration-http.md)
for the injected handler, safe response mapping and bounded process-local limiter.
The handler remains disconnected from the running router; schema review and
persistence adapter ownership are pending with the database teammate.
