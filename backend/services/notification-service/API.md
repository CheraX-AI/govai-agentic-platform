# Citizen notification API checkpoint (GA-270)

The handlers and storage interfaces are implemented and locally testable. They
are not registered by the current foundation application: production persistence,
verified TLS runtime configuration and web BFF integration are still pending.
Do not present the current health-only container as a deployed citizen API.

## Structure

- internal/features/inbox/: owner-scoped storage interface and response records.
- internal/features/preferences/: channel storage and effective security policy.
- internal/handlers/citizen.go: explicit registration and verified owner extraction.
- internal/handlers/inbox.go: listing, read status and unread count.
- internal/handlers/preferences.go: reading and replacing channel preferences.
- internal/handlers/requests.go: bounded, strict preference input decoding.
- internal/handlers/pagination.go: cursor validation and safe page output checks.

## Authentication and transport

RegisterCitizenRoutes requires the existing authn.Verifier, inbox.Store and
preferences.Store. The shared verifier checks RS256 signatures, issuer, audience,
approved client, access-token type and expiry against verified TLS JWKS. All
routes additionally require verified email and the citizen role. Plain HTTP is
rejected; forwarded headers cannot substitute for TLS or identity. Do not turn
off TLS verification or construct a caller from a subject header/query parameter.

Every storage operation receives the verified issuer AND subject. Neither is
accepted from JSON, query strings or URL paths. A notification ID selects a row
only inside that owner's scope. Missing and other-owner IDs return the same 404.
Authenticated responses use no-store caching policy. Errors expose codes only,
never database/provider messages. Each storage operation has a three-second bound.

Mobile callers use their access token. Web browser tokens must remain server-side
in the existing BFF; do not add browser token storage. Teammates must wire a
notification BFF proxy with its normal session/CSRF protections and approved
token client. This checkpoint does not trust cookies directly or add CORS origins.

## Endpoints

| Method | Path | Response |
| --- | --- | --- |
| GET | /api/notifications | items array and next_cursor |
| PATCH | /api/notifications/:id/read | 204, or owner-scoped 404 |
| GET | /api/notifications/unread-count | unread_count integer |
| GET | /api/notifications/preferences | channel flags and fixed policy metadata |
| PUT | /api/notifications/preferences | committed channel flags and policy metadata |

List accepts only limit (default 20, range 1-100) and cursor. Both are single-valued;
unknown selectors and duplicate parameters return 400. Items use descending
(created_at, id) order, with a strict exclusive cursor boundary. Cursors contain
only a UTC ordering timestamp and notification UUID, not owner identity. They are
not authorization credentials; changing one never bypasses the SQL owner filter.

Items expose id, event_type, priority, locale, title, message_body, created_at and
nullable read_at. Ownership, delivery records, raw errors and authentication data
are excluded. Empty lists return items:[] and next_cursor:null. A nonempty cursor
means more items exist; stores must fetch enough rows to determine HasMore. All
pages are checked for bounds, known priority/event pairs, valid text and stable
order before serialization. UI clients must display title/body as text, not HTML.

Mark-read accepts a canonical UUID and an empty request body, with no query fields.
The store sets read_at using server/database time only, preserves the first value
on repeated calls, and never changes email submission status. Counts include only
the authenticated owner's unread inbox rows, including mandatory security alerts.

PUT requires application/json (optional UTF-8 charset), no Content-Encoding and
at most 1024 bytes. Both boolean fields are required for complete replacement:

```json
{
  "email_enabled": false,
  "in_app_enabled": true
}
```

Duplicate keys, unknown fields, null/string/number booleans, incomplete objects and
trailing JSON values return 400. Unsupported media types return 415; oversized
bodies return 413. Both flags commit atomically before a 200 response is returned.
Read/replace responses also include locale:"en" and security_in_app_required:true.
Locale is fixed to the current English-only scope; it cannot be changed here.

The preferences.Channels.Effective method preserves security-critical in-app
alerts for login/password changes even if ordinary in-app notifications are off.
Email opt-out remains respected. The future consumer/worker MUST call this method
before deciding channels; the response metadata alone does not enforce delivery.
Per-event/category settings and multilingual selection require a separately agreed
API/schema extension. This checkpoint covers the global email/in-app choices.

## GA-208 storage handoff

Implement the interfaces against the agreed migration schema, without direct auth
or Keycloak table reads. Do not use the in-memory test fixture in the application.
Agree first-use preference defaults with the database owner; the handler does not
invent missing rows or overwrite defaults during a GET. Add indexes for owner and
(created_at,id) order, plus unread queries. SQL must include BOTH owner components
in list/count/read/preference operations and all cursor continuation predicates.

Register the handlers on the foundation router only after real adapters, JWT/TLS
configuration and database readiness checks are wired. Keep readiness false until
the required schema and adapters are usable. Returning mocked data must never be
used to make a production readiness probe pass.

## Local checks and teammate live acceptance

After rebuilding govai-notification-checks:local, run from repository root:

    docker run --rm govai-notification-checks:local go test -v ./internal/handlers
    docker run --rm govai-notification-checks:local go test -v ./internal/features/...

Tests use generated RSA keys, a local TLS JWKS server and a memory-only storage
fixture. They exercise the REAL JWT middleware, including rejected issuer,
audience/client, expiry, role, unverified email and plaintext access. They test
owner propagation, read/count changes, preference isolation, pagination, malformed
input and safe storage errors. They do not prove PostgreSQL owner isolation,
durability, row locking, browser session security or live Keycloak configuration.

Teammate live tests after integration: two citizens with overlapping record IDs,
same subject under different issuers, pagination with tied timestamps and concurrent
inserts, repeated mark-read, logout/re-login persistence, email opt-out, mandatory
security inbox alerts, database outages, real mobile tokens and web BFF/CSRF flows.
These live tests are assigned to teammates; implementation handoff must retain
these checks and the remaining integration items. GA-270 is not fully complete yet.
