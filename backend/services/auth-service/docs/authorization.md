# GA-49: JWT authentication and citizen authorization

Implemented as an opt-in Go runtime composition with a reusable module at
`backend/pkg/authn`. No frontend/mobile/Python/RAG file is modified. Existing
registration/login behavior remains separate and unchanged.

## Routes

| Endpoint | Requirements | Current successful-authority outcome |
| --- | --- | --- |
| GET `/api/v1/me` | Direct TLS; valid Keycloak API access token; verified email; citizen realm role; owned ready SQL profile | 200 with own subject, verified flag and role list |
| POST `/api/v1/ai/chat` | Same checks, all before the placeholder | 501 `chat_not_implemented` |

Chat is **not implemented** and makes no upstream request. This endpoint proves
authentication precedes future AI handling; it does not satisfy real chat/RAG
integration or prove that a future direct Python URL is protected. Agree the REST
contract and deployment network boundary with the Python teammate before adding
the proxy. Do not expose Python as a public bypass of the protected Go entry point.

Missing/malformed/expired/wrong-issuer/wrong-audience/wrong-client/wrong-signature
tokens return identical 401 `unauthorized`, with WWW-Authenticate: Bearer. A
cryptographically valid token without citizen role/verified email, or without an
owned ready application profile, gets 403 `forbidden`. Profile database failure
gets 503 `profile_unavailable`. Verifier concurrency overload gets 503 with retry
advice. Direct HTTP requests get 426; forwarded TLS headers are never trusted.
Every protected-route response is no-store/no-cache with the router request ID.

The only credential source is **one Authorization header**:

```text
Authorization: Bearer <access_token from the login response>
```

Do not use refresh/ID tokens, put tokens in query strings, paste them in chat or
capture Authorization in request logs. `/me` deliberately returns only the
authenticated caller's identity fields, not NIC/name/mobile/email or raw tokens.
Profiles are matched by validated issuer+subject, never caller email/NIC/role.

## Configuration

Private `.env.authorization` is loaded before login/registration/database files;
process environment variables win. Committed examples disable authorization.

| Variable | Purpose |
| --- | --- |
| API_AUTH_ENABLED | Explicit true activation |
| API_AUTH_ISSUER | Exact HTTPS issuer |
| API_AUTH_AUDIENCE | Intended Go API audience, locally govai-api |
| API_AUTH_CLIENT_IDS | Comma-separated exact authorized-party IDs, locally govai-login-mobile-dev |
| API_AUTH_CA_FILE | Optional trusted private CA PEM |

The reviewed API TLS listener and restricted PostgreSQL settings are also required.
No administrative key/token/client secret is needed to validate public signatures.
For the existing local setup, from the auth-service directory:

```powershell
go run ./cmd/dev-auth configure-authorization
# Stop the prior Go API process before starting this version on the same port:
go run ./cmd/api
```

From a second terminal, `go run ./cmd/dev-auth authorization-check` verifies the
actual local TLS endpoints return 401 for missing/malformed credentials. It uses
the trusted development CA and never needs a real user password or prints a token.

The helper verifies the existing login client's type, writes only fixed public
local verifier settings, and does **not** change realm/users/roles/clients. New
web or other clients require deliberate allowlist changes and integration tests.
The current desktop setup is enabled for the existing mobile/Postman login client.

The auth service uses a committed local replace for the shared module. The
[module guide](../../../pkg/authn/README.md) explains reuse, build context and
running its tests separately. No root Go workspace or unrelated service is changed.

## Postman checks

1. Obtain a **fresh** access token through the existing POST `/api/v1/auth/login`.
   Local access tokens last about five minutes. Keep the token private.
2. Create GET `https://localhost:5001/api/v1/me`, Authorization > Bearer Token,
   paste the access token, then Send. Expect 200 and your own identity.
3. Remove the Authorization header and resend. Expect 401.
4. Replace the token with invalid text or an expired token. Expect 401.
5. POST `https://localhost:5001/api/v1/ai/chat` without a token gives 401. A valid
   verified citizen with a profile gets **501**, not an AI response.

Do not change a real user's roles just to test 403. Automated tests create the
appropriate synthetic identities in isolated environments. They also prove that
query/cookie/forwarded role inputs cannot grant authorization.

## Evidence and remaining work

Shared-module tests use real controlled RSA signatures and trusted TLS servers.
They cover wrong issuer/audience/client/type/time/signature/algorithm, missing
subject, unverified/wrong-role policy, multiple/malformed Bearer headers, key
cache/cooldown/rotation and context-role-copy safety. Auth-service tests cover
profile ownership/readiness, missing-profile 403, database-error 503, protected
chat placeholder, no database access before authentication and CORS/no-store.

The explicit live Keycloak/PostgreSQL test passed on 2026-10-31. It performs real
registration, verification link completion and password login; uses the resulting
token for `/me` 200 and chat's guarded 501; rejects missing/invalid tokens with
401; temporarily removes/restores only a **disposable user's** default-role
membership to prove 403 for a real signed non-citizen token. It checks safe logs,
cleans up only its test database and stops its private containers. Existing GovAI
users and data remain unchanged.

Reproduce with the same opt-in command documented in [login evidence](GA-48-login.md):
`GA48_LOGIN_LIVE=1` with a new private GA48_LOGIN_STATE_DIR, then
`go test -run '^TestLoginLive$' -count=1 -v ./internal/permissionlab`.
Ordinary test runs skip live infrastructure. Run tests/vet/race in **both** Go
modules; a green auth-service unit command does not run shared-module tests.

Offline JWT validation is not immediate revocation or a live Keycloak account
status check. Role/email/session changes can take effect only after access-token
expiry unless a later revocation-aware design is added. JWKS outages fail closed
with generic 401 when needed keys are unavailable; cached valid keys keep working.
Refresh/logout, real browser/mobile integration, distributed abuse controls and
the actual protected RAG proxy remain pending. Do not mark chat or the overall
authentication story complete based on the placeholder.

## User-confirmed manual evidence, 2026-10-31

The user confirmed that the protected /me checks worked after the supplied
fresh-token 200 / no-auth 401 instructions. This is user-confirmed manual evidence,
separate from the independently run signature/RBAC/live tests. No access token or
password was collected. Actual chat/RAG functionality remains pending; the guarded
chat endpoint is a 501 placeholder, not an implemented AI chat service.
