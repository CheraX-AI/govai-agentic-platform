# GA-50 local Postman verification

Run against the approved local HTTPS API at `https://localhost:5001`.
Keep SSL verification enabled and trust only the documented local CA. Use your
existing synthetic, email-verified citizen account; do not share its password or
access/refresh/session/CSRF values in chat, screenshots or Git.

## Native token flow

Use a request without Origin or browser cookies. Disable the Postman cookie jar
for these native requests if earlier web testing stored the web session cookie.
Choose No Auth so an old inherited bearer credential is not sent accidentally.

1. POST `/api/v1/auth/login`, Content-Type `application/json`, raw JSON:

   ```json
   {"email":"YOUR_TEST_ACCOUNT_EMAIL","password":"YOUR_TEST_PASSWORD"}
   ```

   Expected: 200 with access_token, token_type, expires_in, refresh_token and
   refresh_expires_in. No ID token or browser cookie is returned.
2. POST `/api/v1/auth/refresh`, same JSON content type:

   ```json
   {"refresh_token":"YOUR_CURRENT_REFRESH_TOKEN"}
   ```

   Expected: 200. Replace stored tokens with the new pair. Do not reuse the old
   refresh token; refresh calls must be serialized. Reuse/invalid/expired tokens
   are rejected with 401. A timeout may have consumed the token; do not blindly
   retry it. Sign in again if its outcome is unknown.
3. POST `/api/v1/auth/revoke` with the current refresh_token JSON. Expected 204.
   Clear the client's stored tokens. Further refresh with it should be 401.
   Offline-validated access JWTs may remain valid until expiry (normally 300s).

## Browser-cookie backend flow

Use the Postman cookie jar for these requests, separate from native requests.
Every request needs `X-GovAI-Web: 1`. Writes also need
`Origin: https://localhost:5001`. No bearer Authorization header is used.

1. POST `/api/v1/web/login`, Content-Type `application/json`, the same email/password
   JSON as above. Expected 200 with csrf_token and expires_at. Postman receives a
   Secure/HttpOnly/SameSite Strict `__Host-govai_session` cookie. JSON contains no
   provider access/refresh token. Existing sessions must be logged out before
   switching accounts.
2. GET `/api/v1/web/session`. Expected 200 with your subject only.
3. GET `/api/v1/web/csrf` restores csrf_token after a page reload. This can also
   recover the secret needed for logout from a pending refresh; it grants no
   application access.
4. POST `/api/v1/web/refresh`, Body **none**, add `X-CSRF-Token` with the current
   csrf_token. Expected 204; provider tokens rotate only on the server.
5. POST `/api/v1/web/logout`, Body **none**, same Origin/custom/CSRF headers.
   Expected 204 and cookie removal. GET session afterwards should return 401.

If logout returns 503 with provider_revocation_pending, local custody ended and
the cookie is cleared, but provider revocation is queued. An operator can inspect
and reconcile it using the documented explicit maintenance commands. Do not
interpret this as completed global SSO logout.

This Postman origin is for backend testing. Real frontend HTTPS origin and
same-site deployment must be agreed with its owner before browser integration.
Postman does not prove browser SameSite/CORS behavior. Frontend/mobile/Python files
remain owned by teammates, and actual client evidence is still pending.

## Existing safe local commands

From `backend/services/auth-service`:

```powershell
go run ./cmd/db check-web
go run ./cmd/dev-auth inspect
go run ./cmd/dev-auth sessions-check
go run ./cmd/web-maintenance status
```

The local rollout already ran these checks successfully using invalid/synthetic
credentials only. Repeated attempts count toward abuse limits. Do not run cleanup
or reconfigure clients merely to repeat this manual verification.
