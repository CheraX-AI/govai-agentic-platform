# GA-48 mobile/Postman login

The user selected the access-token API for **native mobile/Postman first**.
Browser cookie/BFF sessions remain deferred. This is the existing team/mentor
approved prototype Direct Grant decision; the documented conflict with
[RFC 9700 section 2.4](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.4)
remains unchanged. HTTPS does not remove that architectural limitation.

## API

```text
POST https://localhost:5001/api/v1/auth/login
Content-Type: application/json
```

Choose No Auth in Postman. Do not supply a registration idempotency key, client
secret, client ID, scope, NIC, role, audience, issuer or redirect in the request.

```json
{
  "email": "your-registered-test-account@govai.test",
  "password": "your-test-password"
}
```

Use an account created through the Go registration API, with completed email
verification and an enabled owned PostgreSQL profile. Legacy hosted test accounts
with no application profile are intentionally rejected; they are not silently
adopted or linked by email. This endpoint accepts email, not legacy usernames/NICs.
Email uses the same approved ASCII canonicalization as new-account registration.
Passwords retain their exact Unicode/whitespace and are never trimmed or stored.

Successful response:

```json
{
  "access_token": "<sensitive Keycloak access token>",
  "token_type": "Bearer",
  "expires_in": 299
}
```

Keep the access token in native application memory for this slice. Do not paste
it into chat/screenshots, store it in browser localStorage or embed server client
secrets in apps. Refresh and ID tokens are not returned or persisted. No cookie
is created. Expiry can require login again; refresh/logout are separate work.
This endpoint does not itself provide a protected chat API or JWT middleware.

| Outcome | Status / code |
| --- | --- |
| Wrong credentials, unknown/disabled/locked account, unverified email, pending actions, missing citizen role or owned ready profile | Identical 401 `login_failed` |
| Missing/unknown/duplicate/wrong-type JSON fields, invalid email, empty/overlong password, malformed UTF-8/surrogate/trailing JSON | 400 `invalid_request` |
| Body over 4 KiB | 413 `too_large` |
| Unsupported content type/charset/encoding | 415 `unsupported_media_type` |
| Direct TLS missing | 426 `https_required` |
| Browser Origin header present | 403 (browser integration not enabled) |
| Attempt/concurrency limits | 429 `rate_limited`, Retry-After |
| TLS/network/deadline, client misconfiguration, malformed/invalid provider token, JWKS or database failure | 503 `login_unavailable` |

The generic 401 text is:

```json
{
  "error": "Unable to sign in. Check your details and complete any required account actions.",
  "code": "login_failed"
}
```

No provider descriptions, email-action lookup or automatic resend is exposed.
Users complete required actions through the existing approved email/account flow;
the API never clears VERIFY_EMAIL, UPDATE_PASSWORD, UPDATE_PROFILE or MFA actions.
All responses are no-store/no-cache with the router's generated request ID.

## Security and dependencies

- `internal/login` performs strict parsing, one bounded HTTPS password exchange,
  validated access-token checks and issuer/subject-only profile lookup. It does
  not issue Go JWTs or inspect Keycloak password storage.
- `coreos/go-oidc` verifies RS256 signature, exact issuer, intended `govai-api`
  audience and expiry, using a cached fixed-origin Keycloak JWKS endpoint.
  Additional checks require authorized party equal to the fixed login client,
  Keycloak access-token `typ=Bearer`, nonempty subject, sane iat/nbf, at most
  one-hour remaining lifetime, verified email and citizen role. ID/unsigned/wrong
  audience tokens are rejected. No caller token/JWKS URL is accepted.
- `postgres.Store.HasReadyProfile` checks the immutable issuer/subject/operation/NIC
  linkage and enabled-or-later phase without decrypting/returning citizen data.
- Client -> Go and Go -> Keycloak/JWKS use validated TLS. Unknown CA, wrong
  hostname, HTTP URL, insecure TLS hooks and redirects are rejected. Private CA
  roots are loaded explicitly, never with a certificate bypass.
- Credentials POST exactly once, with no redirect/retry, no Idempotency-Key and
  no administrator token. Fixed scope is `email`, not offline_access/openid.
- The handler deadline is five seconds, including body read deadline and the
  propagated provider/SQL context. Provider calls also have a five-second bound,
  64 KiB response and 16 KiB token cap. The OIDC library may finish a shared
  public-JWKS fetch after an individual caller cancels; its client timeout bounds
  that fetch to five seconds. Credentials are never in that request.
- Separate bounded process-local counters: IP 5/15 minutes, canonical email
  5/15 minutes, 4096 entries per map, 8 simultaneous requests. They reset on
  restart and do not provide distributed enforcement or protection from targeted
  email/NAT lockout. Forwarded IP/TLS headers are not trusted.
- Browser Origin requests and POST browser preflight are denied. This prevents
  accidental reuse as the proposed browser BFF flow; it is not client attestation.
  Mobile/Postman HTTP requests do not need an Origin header.

## Configuration and local start

Committed examples leave LOGIN_ENABLED=false. Private settings live in ignored
`.env.login`, loaded before `.env.registration`, `.env.database`, then `.env`.
Process environment settings always win. The API never loads migration/admin
credentials or makes administrative client/realm changes at startup.

| Setting | Value/purpose |
| --- | --- |
| `LOGIN_ENABLED` | Explicit `true` activation |
| `LOGIN_ISSUER_URL` | Exact HTTPS realm issuer |
| `LOGIN_CLIENT_ID`, `LOGIN_CLIENT_SECRET` | Separate confidential login client; never registration/admin client |
| `LOGIN_AUDIENCE` | Intended Go API audience, locally govai-api |
| `LOGIN_CA_FILE` | Optional trusted private CA PEM |
| `LOGIN_RATE_KEY` | Independent base64 32-byte server abuse-counter key |
| `API_TLS_CERT_FILE`, `API_TLS_KEY_FILE`, database settings | Reuse reviewed HTTPS listener and restricted application database settings |

For this local GovAI setup, after Keycloak/PostgreSQL start:

```powershell
cd C:\Users\LENOVO\Desktop\govai-agentic-platform\backend\services\auth-service
go run ./cmd/dev-auth configure-login
# Stop the previous Go process with Ctrl+C before starting the new code:
go run ./cmd/api
```

An operator-only check of the actual local route is also available:

```powershell
go run ./cmd/dev-auth login-check
```

It uses the explicit development CA, checks readiness, and expects the generic
401 for one randomly generated nonexistent identity. It never reads/prints a real
user password/token. It consumes one attempt from the process-local IP limit.
Windows curl's Schannel backend may require certificate revocation metadata absent
from this private development CA; use this validated Go check rather than disabling
certificate/revocation checks to get a curl result.

The setup command creates only the owned `govai-login-mobile-dev` client, with
Direct Grant on, browser/service-account/implicit flows off, full scope off,
citizen role scope mapping and explicit audience/email-verification mappers.
Refresh issuance is disabled and access lifespan set to 300 seconds. It does not
grant administrative roles, change existing user credentials, migrate legacy
accounts, enable web sessions or modify the registration client. The confidential
client secret stays server-side. This is local development provisioning, not a
production identity deployment or brute-force-policy audit.

## Evidence

Unit/TLS tests cover strict credential preservation/redaction; generic error and
no-store behavior; denied browser/HTTP use; fixed counters and no forwarding after
throttle; response bounds; no credential retry/redirect; wrong issuer/audience/azp,
expired/future/ID/unsigned/forged tokens; signing-key rotation and cached JWKS;
required citizen/verification/profile checks; cancellation and database outage.

The explicit live test passed on 2026-10-31 against a fresh HTTPS Keycloak realm,
Mailpit and disposable PostgreSQL. Real registration and a genuine verification
email link preceded successful login. Wrong password, unknown account and
unverified/pending-action login had identical 401 bodies. Signed access token,
audience, subject/profile/citizen checks passed; provider refresh/ID issuance was
absent; the sixth API attempt returned 429. Logs contained only request metadata.
Its test database was removed and its containers stopped; GovAI data was preserved.

Reproduce from the service directory:

```powershell
$env:GA48_LOGIN_LIVE = '1'
$env:GA48_LOGIN_STATE_DIR = Join-Path $env:TEMP ('govai-ga48-login-' + [guid]::NewGuid().ToString('N'))
try { go test -run '^TestLoginLive$' -count=1 -v ./internal/permissionlab }
finally {
  Remove-Item Env:GA48_LOGIN_LIVE -ErrorAction SilentlyContinue
  Remove-Item Env:GA48_LOGIN_STATE_DIR -ErrorAction SilentlyContinue
}
```

Ordinary unit/race runs skip live infrastructure. Real mobile-client integration,
browser BFF custody, deployment rate limits/Keycloak brute-force review,
JWT/RBAC middleware/chat, refresh/logout and deployed session termination remain
pending. An issued-but-discarded refresh token would not terminate its Keycloak
session; suppressing refresh issuance was independently verified in the live test.

References: [Keycloak token endpoint](https://www.keycloak.org/securing-apps/oidc-layers),
[go-oidc verifier/key-set](https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc).
## User-confirmed Postman evidence, 2026-10-31

The user explicitly confirmed successful Postman login and reported completing
the wrong-password check. These are user-confirmed manual results, distinct from
the independently run automated/live tests above; no credential or token value
was collected. Actual native-client integration and web sessions remain pending.
