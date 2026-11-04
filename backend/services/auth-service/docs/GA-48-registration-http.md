# GA-48 isolated registration HTTP adapter

Subsequent update: [runtime wiring](GA-48-registration-runtime.md) now optionally
composes this handler with real PostgreSQL and Keycloak. The original isolated
slice/evidence below is historical; default registration remains disabled.

`handler.RegistrationHandler` accepts an injected service and limiter. It is **not
registered** by `NewRouter` or `cmd/api`; health endpoints remain unchanged. Tests
attach it to a private Gin engine with synthetic service functions. No runtime
profile/journal persistence is added. PostgreSQL schema review and persistence
adapter ownership remain pending team decisions.

The handler reuses `registration.ParseRequest` for validation and canonical-email
admission, then passes original bounded bytes to the orchestration service, which
repeats authoritative validation. Password whitespace/encoding is preserved without
serializing the redacted Request. No body, password, personal value, dependency
error or panic value is logged or included in responses. The strict parser prevents
caller-supplied group/role/administrative configuration. Existing operation
ownership and ambiguous-creation reconciliation rules remain in the service.

| Outcome | Response |
| --- | --- |
| Received, including duplicate identifiers | Identical 202 `received` message from the API plan |
| Malformed JSON or idempotency key | 400 `invalid_request` |
| Body exceeds 16 KiB | 413 `too_large` |
| Unsupported content type, charset or encoding | 415 `unsupported_media_type` |
| Validation or policy rejection | 422 `validation_failed` |
| Same-operation request conflict | 409 `request_conflict` |
| Rate/concurrent admission limit | 429 `rate_limited` with integer `Retry-After` seconds |
| Dependency, cancellation, timeout, reconciliation or pending-email failure | 503 `registration_unavailable` with configured `Retry-After` |

Every handler response sets `Cache-Control: no-store`. Trusted middleware response
request IDs are preserved; otherwise a random ID is generated. Caller request-ID
headers are never reflected. Field error names/codes are allowlisted; generic
policy rejection does not invent a password-policy diagnosis. A 503 does not
promise rollback: retries must retain the same operation key/body for recovery.

## Bounded process-local admission

`NewLocalRegistrationLimiter` supplies bounded expiring abuse counters, not
registration persistence. It is not constructed by the running API. Source-IP
attempts are charged before parsing; canonical-email attempts after validation.
Concurrent admission is capped and permits released on all exits. Fixed windows
do not extend on rejected attempts. Full maps reject new keys instead of evicting
active buckets; expired entries are reclaimed on access without a background task.

The server supplies a private random HMAC key of at least 32 bytes. Only domain-
separated digests, counts and expiry times are retained. Configuration/limiter
formatting is redacted. Each IP/email map is bounded by `MaxKeys` (maximum 10,000
each), with at most 1,000 concurrent permits. A starting policy for review is IP
5/15 minutes, email 3/hour, MaxKeys 4096 and MaxConcurrent 16. No runtime settings
or defaults are enabled by this change.

Source identity uses socket `RemoteAddr`, normalizing IPv4-mapped IPv6; forwarding
headers are ignored. Behind a proxy, clients share the proxy address until a
reviewed trusted-proxy policy is introduced. NAT users share limits and full maps
can deny new users. **Process-local limits do not provide distributed enforcement**:
replicas, restarts and changing addresses bypass aggregate limits. Shared/edge
enforcement and load testing remain required. An optional bounded response floor
and generic responses do not prove enumeration timing resistance.

## Integration gates and evidence

Construction requires a bounded timeout (up to 30s) and bounded unavailable retry
advice. Context cancellation propagates to the service. Body size is bounded, but
context cannot interrupt an arbitrary blocked reader; HTTP server read deadlines
remain necessary. Before wiring, align existing 10s server read/write timeouts
with the orchestration budget (the plan proposes 15s), and review POST CORS, TLS
ingress, trusted proxies, distributed abuse controls and response timing.

Synthetic tests cover parsing, validation, password preservation, duplicates,
safe error mappings, cancellation/deadlines, limiter failure, concurrent admission,
IP/canonical-email limits and retry advice. Counter tests cover expiry, capacity,
concurrent charging, cancellation and redaction. A router regression confirms
registration remains absent; existing health tests still apply.

This proves isolated HTTP behavior with test doubles, not database durability,
distributed concurrency, restart recovery, real client integration or live Keycloak
semantics. Prior disposable HTTPS lifecycle evidence remains separate and was not
rerun for this HTTP-only change. Next: review the database teammate's schema and
assign adapters for the documented atomic reservation, normalized-NIC uniqueness
and recovery interfaces, then verify disposable PostgreSQL before route wiring.

Run from `backend/services/auth-service`: `go test ./...`, `go vet ./...`,
and `go test -race ./...` where supported.
