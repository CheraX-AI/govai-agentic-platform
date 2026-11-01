# GovAI core service — GA-65 secure chat API

Go/Gin core-service hosts chat business endpoints separately from the authentication
service. It reuses `backend/pkg/authn` and existing Keycloak-issued access JWTs.
Canonical route: POST `/api/chat/message` on the core API (local HTTPS port 5002).
The auth-service's earlier `/api/v1/ai/chat` 501 placeholder is unchanged; it is not
an alias or the Knowledge Agent integration. Use the core URL for this contract.

## Implemented and verified

- Exact issuer/audience/allowed-client, signature, expiration, verified email and
  citizen role checks through the shared middleware; direct HTTPS is required.
- Strict JSON `{ "message": "..." }`, one object, no unknown/duplicate fields,
  UTF-8 preservation, nonempty trimmed message, at most 4000 Unicode code points
  and at most 16 KiB for the complete request body. Identity/roles/agent URLs
  cannot be supplied in the payload. Unsupported media/encodings are rejected.
- Injected `chat.Agent` port; only the question is passed to the agent, without
  browser headers, access/refresh tokens, NIC, email or caller-selected identity.
- JSON `{ "answer": "...", "sources": [{ "title": "...", "url": "..." }] }`;
  absent sources become `[]`. Returned content comes only from the agent. Basic
  empty/size/HTTPS-source bounds fail closed. These checks do not establish source
  truth or government authority. Detailed transport processing belongs to GA-66/67.
- Bounded request deadlines and eight in-flight handler slots; overload returns
  Retry-After. This is not distributed or per-citizen rate limiting.
- Exact-origin, non-credentialed CORS; configured preflights permit POST with
  Authorization/Content-Type. Trusted proxies are disabled. Safe metadata logs
  contain generated request ID/method/route template/status/duration, never question,
  answer, token, subject, raw path or query values. Unexpected errors are generic.
- TLS server timeouts/header limits, health endpoint and graceful shutdown.

`go test -race ./...` and `go vet ./...` passed with signed synthetic JWTs and a
trusted synthetic JWKS server. Tests cover success/citations, Tamil/Sinhala text,
401/403/no agent calls for rejected identities, malformed/empty/oversized requests,
media/TLS/CORS boundaries, cookie-only rejection, safe failures/logs and concurrent
admission limits. Success answers/citations in tests are explicitly synthetic.
Local server startup and trusted-CA health 200 / unauthenticated chat 401 passed.
Existing authentication/profile storage remains in auth-service; core only relies
on validated JWT citizen/email claims and performs no profile/database lookup.

## Current GA-65 limits

An opt-in HTTP adapter is implemented and tested against synthetic HTTP/TLS agents.
The real Python agent is not ready and its contract is not confirmed. By default
`KNOWLEDGE_AGENT_ENABLED=false`, so valid authenticated questions still return
**503 chat_unavailable**. No government answer or citation is fabricated.
GA-66 live integration and GA-23 completion remain pending.

## GA-66 provisional transport configuration

The proposed upstream POST payload is exactly `{"message":"..."}` and the response
is `{"answer":"...","sources":[{"title":"...","url":"https://..."}]}`.
Missing/null sources normalize to `[]`. Confirm these fields and the exact endpoint
with the Python owner before enabling; no Python files were changed.

- `CHAT_REQUEST_TIMEOUT`: total handler budget, default 30s (1s to 1m).
- `KNOWLEDGE_AGENT_ENABLED`: false by default; true enables the adapter.
- `KNOWLEDGE_AGENT_URL`: exact server-configured POST endpoint, including path.
  No query, fragment, user information, redirects or caller-selected destinations.
- `KNOWLEDGE_AGENT_TIMEOUT`: default 20s; at least 1s and less than the handler budget.
- `KNOWLEDGE_AGENT_AUTH_MODE`: explicit `bearer` for remote HTTPS services;
  `KNOWLEDGE_AGENT_BEARER_TOKEN` is a separate service credential, never a citizen
  access/refresh token. This mechanism must be agreed with the Python owner.
- `KNOWLEDGE_AGENT_CA_FILE`: optional PEM trust roots for the agent's certificate;
  certificate and hostname verification remain enabled.
- `none` authentication is restricted to literal loopback IPs. Plain HTTP additionally
  requires `KNOWLEDGE_AGENT_ALLOW_LOOPBACK_HTTP=true`, only for isolated local tests.
  Never send service credentials over HTTP.

The adapter forwards only the validated question. Cookies and citizen identity
headers are not forwarded. It disables redirects, compression and proxy inheritance,
uses bounded responses and connections, and performs no application retries.
Upstream 429/5xx or connection failures return safe 503 errors; malformed/unexpected
responses return 502; deadlines return 504. Logs exclude question/answer content
and credentials. Source URL checks do not prove source authenticity.

Synthetic tests cover multilingual payloads, service authentication, exact routing,
redirect rejection, cancellation, timeouts, untrusted TLS, response bounds,
malformed/duplicate fields, and source preservation. These are mock transport tests,
not evidence of successful real-agent or web/mobile integration. Keep the adapter
disabled until the contract, endpoint, TLS and service authentication are confirmed.
GA-67 response/error acceptance and actual running-agent checks remain separate.

The endpoint accepts bearer access tokens. Existing web HttpOnly cookie sessions
are owned by auth-service; they intentionally do not authenticate at this separate
core API. A backend BFF/gateway forwarding bridge is still required for that web
flow, using server-held access credentials and preserving CSRF/origin checks. Do
not expose refresh tokens to React or move auth-service tables into core. Web and
mobile will share the message/reply schema; real client integration remains pending.

## Local setup and API checks

Run from `backend/services/core-service` with Go 1.26+:

```powershell
Copy-Item .env.example .env
# Fill TLS certificate/key paths and confirm issuer, API audience and client IDs.
go mod tidy
go test ./...
go vet ./...
go run ./cmd/api
```

Never overwrite an existing `.env`. Parent backend ignore rules exclude it.
Only placeholders belong in `.env.example`. Local configuration currently reuses
the existing trusted localhost development certificate/key paths, without copying
private key material. Deployed services require their approved TLS configuration.
Keycloak must be running at the configured issuer for JWT key retrieval.

Verify the correct service in Postman: GET https://localhost:5002/api/health returns
`{"service":"govai-core","status":"ok"}`. Auth APIs stay on HTTPS 5001.
For chat, choose POST https://localhost:5002/api/chat/message, application/json,
and Authorization -> Bearer Token with a fresh access token issued by GovAI:

```json
{"message":"What documents are required for a Sri Lankan passport?"}
```

Expected: no/invalid bearer -> 401; wrong role/unverified email -> 403; valid citizen
with invalid/empty message -> 400; request envelope too large -> 413; wrong media ->
415; valid citizen/question while no agent is wired -> 503. Synthetic tests establish 200 handling. The user-confirmed local Postman sequence on 2026-11-01 returned health 200, no-token chat 401, valid-token question 503 chat_unavailable and empty message 400 invalid_message. Agent-owned request metadata independently confirmed these status codes; no tokens or question bodies were collected. Core
never treats a cookie, query token or forwarding header as authenticated identity.
A valid token can remain accepted until expiry after logout, per the shared offline
JWT policy; no immediate bearer-revocation guarantee is added by this API.

Restarting after Ctrl+C is safe. No migrations or external identity changes occur
at startup. Run module tests from this directory; this is independent of the auth
service module and local dependencies use the existing pkg/authn replace directive.