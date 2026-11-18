# GovAI core service — GA-65 secure chat API

Go/Gin core-service hosts chat business endpoints separately from the authentication
service. It reuses `backend/pkg/authn` and existing Keycloak-issued access JWTs.
Canonical route: POST `/api/chat/message` on the core API (local HTTPS port 5002).
The auth-service's earlier `/api/v1/ai/chat` 501 placeholder has been removed. Use
the core URL for this contract (browsers go through auth-service `/api/v1/web/chat`).

## Implemented and verified

- Exact issuer/audience/allowed-client, signature, expiration, verified email and
  citizen role checks through the shared middleware; direct HTTPS is required.
- Strict JSON `{ "message": "..." }`, one object, no unknown/duplicate fields,
  UTF-8 preservation, nonempty trimmed message, at most 4000 Unicode code points
  and at most 16 KiB for the complete request body. Identity/roles/agent URLs
  cannot be supplied in the payload. Unsupported media/encodings are rejected.
- Injected `chat.Agent` port; only the question is passed to the agent, without
  browser headers, access/refresh tokens, NIC, email or caller-selected identity.
- JSON `{ "answer": "...", "sources": [{ "number": 1, "title": "...", "page": 3 }] }`;
  `number` matches the `[n]` citation, `page` and `url` (HTTPS only) are optional;
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
on validated JWT citizen/email claims and performs no profile lookup. Its only
database use is conversation ownership (see "Conversation IDs") and the encrypted
message record (see "Conversation messages").

## Current GA-65 limits

An opt-in HTTP adapter is implemented and tested against synthetic HTTP/TLS agents.
The real Python agent is not ready and its contract is not confirmed. By default
`KNOWLEDGE_AGENT_ENABLED=false`, so valid authenticated questions still return
**503 chat_unavailable**. No government answer or citation is fabricated.
GA-66 live integration and GA-23 completion remain pending.

## GA-66 agent transport configuration

The agent is the agent-service chat API (`ai-platform/services/agent-service`,
section "Chat API for core-service"): `POST /api/v1/chat`. The upstream payload is
exactly `{"message":"..."}` and the response is
`{"answer":"...","sources":[{"number":1,"title":"...","page":3,"url":"https://..."}]}`.

- `number` (1-100, unique) and `title` are required; `page` (1-100000) and `url`
  are optional and may be absent or null. A present `url` must be HTTPS. The agent
  does not send `url` yet because its documents have no public citizen address.
- Any other field, a missing number, or a non-integer number/page returns 502.
- Missing/null sources normalize to `[]`.
- The core request ID is sent upstream as `X-Request-ID`, so one question can be
  followed through core, agent and knowledge-service logs.

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
at startup; startup only checks that the database is migrated. Run module tests from this directory; this is independent of the auth
service module and local dependencies use the existing pkg/authn replace directive.
## GA-67 response and error verification

The authenticated router and real HTTP adapter are tested together against a
synthetic TLS agent. This verifies source preservation, absent sources as `[]`,
empty/malformed/duplicate-field/unsafe-source rejection (502), upstream 429/503
mapping (503), and request timeout mapping (504). Invalid messages (400) and
missing tokens (401) produce no upstream call. Error responses contain only
`error` and `code`; request IDs remain in headers. Responses are not cached.
Tests also check that citizen tokens, service credentials, upstream addresses,
questions, answers and upstream error bodies do not appear in logs or error bodies.
Existing tests cover panic recovery, role/email denial and admission limits.

Validation: formatting, `go test -race ./...` and `go vet ./...` passed locally.
These checks use synthetic fixtures; actual Knowledge Agent compatibility,
real generated answers/citations, and teammate web/mobile integration remain
pending. The adapter remains disabled in the local configuration. Clients must
render agent text safely; this backend does not turn AI output into trusted HTML.

## Conversation IDs

Each chat belongs to a conversation, identified by a UUID and owned by one
citizen. The ID and its owner are stored, and every exchange is saved encrypted
(see "Conversation messages"). An internal read for recent messages exists, but no prompt memory uses it yet.

- Request: `{"message": "...", "conversation_id": "<uuid>"}`. `conversation_id`
  is optional, so existing clients keep working; leaving it out starts a new
  conversation.
- Reply: `{"answer": "...", "sources": [], "conversation_id": "<uuid>"}`. Send the
  ID back with the next message to continue the same conversation.
- The owner is always the verified token subject (`sub`), never a request field;
  unknown fields such as `citizen_id` are rejected.
- Malformed ID (not a lowercase UUID): **400** `invalid_conversation_id`. Unknown ID,
  or another citizen's: **404** `conversation_not_found` (the same answer, so IDs
  cannot be probed). Storage down: **503** `chat_unavailable`.
- The resolved ID goes to agent-service in the request body:
  `{"message": "...", "conversation_id": "<uuid>"}`. The agent returns the same ID; a
  reply with a different ID is rejected (**502** `invalid_agent_response`), so only
  core-service decides the conversation.

Storage is the `govai_core` database on the shared PostgreSQL server, separate from
every other service's database. Table `core.conversations` (`internal/database/migrations/001_conversations.sql`). The
API login `govai_core_app` can only read and insert; it cannot update or delete
owners. After pulling a new migration, run `go run ./cmd/migrate` again. The API refuses to start if `CORE_DATABASE_URL` is missing, the schema is not
migrated, or the login is too privileged.

One-time setup, from this folder:

```powershell
# 1. Create the database and logins (as a PostgreSQL superuser, new random passwords)
psql -U postgres -h 127.0.0.1 -p <port> -v owner_password='<random>' -v app_password='<random>' -f internal/database/setup/create_database.sql
# 2. Put CORE_MIGRATION_DATABASE_URL (owner) in .env.migrate and CORE_DATABASE_URL (app) in .env
# 3. Apply the schema with the owner login
go run ./cmd/migrate
# 4. Optional live test with the app login
$env:CORE_TEST_DATABASE_URL = "<CORE_DATABASE_URL value>"; go test -tags integration ./internal/conversation/pgstore/
# 5. Optional migration test: needs a login that can create databases and SET ROLE govai_core_app
#    (a local superuser); each test creates and drops its own govai_core_test_* database
$env:CORE_TEST_ADMIN_DATABASE_URL = "postgres://postgres:<password>@127.0.0.1:<port>/postgres?sslmode=disable"; go test -tags integration ./internal/database/
```

Tests: `internal/conversation` (new UUID, reuse, citizen A vs B, unknown and
malformed IDs, storage failures), `internal/httpapi/conversation_test.go` (the same
through the authenticated router, plus 400/404/503 codes), and the agent HTTP test
checks that the agent request body carries the ID.

## Conversation messages

Each exchange is saved, encrypted, in `core.conversation_messages`
(`internal/database/migrations/002_conversation_messages.sql` and `003_message_turns.sql`).
`Messages.Recent` reads the recent window internally; there is no history API and no AI memory yet.

How a chat request is recorded (`internal/conversation/messages.go`):

1. After the conversation is resolved, the question is saved as a `citizen` message,
   `pending`, before the agent is asked. If that save fails: **503**
   `chat_unavailable`, and the agent is not called.
2. Agent answer: the `assistant` message is added and the question marked `completed`
   in one transaction. Only then is the answer returned. If the save fails the citizen
   gets **503** `chat_unavailable`, never an unsaved answer.
3. Agent failure (error, timeout, invalid or mismatched reply): the question is marked
   `failed` and the usual error is returned.

Outcomes (steps 2 and 3) are saved even if the client disconnects, within 5 seconds.

Retries never add a second turn. The client contract has no idempotency key, so a
retry is a message equal to the conversation's last question:

- last question `failed`: it is reopened (`pending` again) and the agent is asked again;
- last question `completed` less than 10 minutes ago: the stored answer is returned
  (same JSON: `answer`, `sources: []`, `conversation_id`) and the agent is not asked;
  after 10 minutes the same text is a new question;
- last question still `pending`: **503** `chat_busy` with `Retry-After: 2`. A question
  pending longer than `CHAT_REQUEST_TIMEOUT` + 30s belongs to a lost request; it is
  marked `failed` and can be retried.

A per-conversation advisory lock makes this decision safe under concurrent requests.

Storage:

- `message_id` (UUID primary key) and `conversation_id`, a foreign key to
  `core.conversations`. Ownership stays on the conversation: every write checks the
  verified token subject. A conversation that has messages cannot be deleted.
- `ordinal` is assigned by the database and orders a conversation's messages;
  the `(conversation_id, ordinal)` index serves ordered reads.
- `role` is `citizen` or `assistant`. An assistant message has `reply_to`, its
  question in the same conversation; each question has at most one answer.
- `status`: a question is `pending`, `completed` or `failed`; pending may complete or
  fail, failed may be retried, completed is final. Answers are always `completed`.
- `created_at` and `updated_at` are set by the database.
- `content_ciphertext`: the original text sealed with AES-256-GCM (nonce + ciphertext
  + tag, as in auth-service), never plaintext. The additional data binds it to its
  conversation, message ID and role, so it cannot be moved to another row. No tokens
  or credentials are stored, and message text is never logged.
- `govai_core_app` may read, insert and set `status`; it cannot choose the ordinal or
  timestamps, rewrite content, move or delete messages.

### Reading recent messages

`Messages.Recent(ctx, owner, conversationID, limit)` is the internal read for the
future memory loader. There is no HTTP endpoint for it and nothing calls it yet.

- The owner must be the verified token subject. Unknown and other citizens'
  conversations both return `ErrNotFound`.
- It returns the last `limit` completed turns (1 to 50), each as the citizen message
  then the assistant message, oldest first. Failed and pending questions are left out.
- Order comes from the database `ordinal`, so it is the same after a restart.
- A turn that does not decrypt (sealed with another key) is skipped, not fatal.
- Storage errors return `ErrUnavailable`, with no database detail.

### Encryption and key handling

- Message text is encrypted in core-service before it reaches PostgreSQL, with
  AES-256-GCM: a random 12-byte nonce, the ciphertext and a 16-byte tag, as in
  auth-service. The database never sees plaintext or the key.
- The additional data binds each value to its conversation, message ID and role, so a
  stored value copied to another row does not decrypt.
- The key is `CORE_CONVERSATION_DATA_KEY` (required): base64 of 32 random bytes, kept
  in the git-ignored `.env` or the deployment's secret store, never in the database,
  logs or responses. The API refuses to start if it is missing, malformed, not 32
  bytes or all zero. Generate one:

  ```powershell
  [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
  ```

- Keep it stable. Messages sealed with an old key cannot be read with a new one: they
  are skipped by `Recent`, and a repeated question starts a new turn. There is no key
  rotation yet, as in auth-service.

### Retention

Messages are kept indefinitely. The runtime login cannot delete messages or
conversations, and there is no deletion job, the same as auth-service ("No automatic
key rotation, retention deletion or schema downgrade"). A retention period and
deletion (for example on citizen request) need a separate decision and migration.

### Masking and metadata

- No masked copy is stored. The only copy is the encrypted original, which already
  keeps the text unreadable at rest. The project has no message-masking design to follow
  (masking exists only for email and phone in account recovery). If the agent ever needs
  masked history, mask when the memory is built, not in storage.
- No agent retrieval metadata (sources, document names, scores) is stored. Citizens are
  not shown sources, and core-service only keeps what is needed to rebuild the chat.
- Stored per message: IDs, conversation, role, status, the encrypted text and two
  timestamps. Not stored: access tokens, passwords, credentials, the token subject (it
  stays on `core.conversations`), IP addresses, user agents or request IDs.

Migration checksums are taken over LF line endings, so a Windows checkout (CRLF) and
the Linux image agree. `internal/database/database_test.go` pins every shipped
migration: never edit an applied file, add a new one.
