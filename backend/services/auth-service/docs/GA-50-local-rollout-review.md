# GA-50 local rollout review

Reviewed 2026-10-31. Local target only: existing GovAI realm at
`https://localhost:8443`, API at `https://localhost:5001`, application database
on `127.0.0.1:55432`. No frontend/mobile/Python files are in scope.

## Review outcome

Fixed a logout/refresh race: immediate revocation now uses the credential in the
atomically captured recovery job, rather than a potentially stale pre-logout
snapshot. Added regression coverage for a rotation occurring between those reads.
A pending refresh now permits CSRF restoration for logout after page reload while
continuing to deny application access.

Passed: auth-service full tests, vet and race tests; shared authn module tests/vet;
real disposable PostgreSQL race tests; final combined real Keycloak/PostgreSQL
web/native/expiry/client-isolation/recovery harness; whitespace checks. Private
environment files are ignored and new SQL migrations have LF Git attributes.
Baseline migration 001 was unchanged. Tests prove the documented backend paths,
not actual React/mobile integration or global logout on every device.

## Existing local configuration (read-only inspection)

- Five non-service users; email verification enabled; hosted signup disabled;
  SMTP configured. No user/role/credential mutation was performed in this review.
- Realm refresh rotation: disabled; max reuse: zero.
- Owned `govai-login-mobile-dev`: refresh suppressed, no service account.
- `govai-login-web-dev`: not configured.
- Baseline application database readiness passed.
- Private `.env.web` prepared with fresh independent secrets and
  `WEB_SESSION_ENABLED=false`. Origin is the user-approved backend-only Postman
  origin `https://localhost:5001`. No secret values are in this document.

## Concrete proposed rollout

1. Preserve current relevant realm/client settings and private local configuration
   in ignored local backup state; do not export user credentials or print secrets.
2. Apply explicit additive optional migrations 002/003 with migration credentials.
   API credentials stay restricted; no existing table/data is dropped or rewritten.
3. Create separate non-admin confidential `govai-login-web-dev`, direct grant and
   refresh enabled, service account/standard/implicit disabled, citizen role scope
   and govai-api/email-verification token mappers. Use the prepared private secret.
4. Enable refresh issuance on the owned mobile client, retaining its existing secret,
   scope and non-admin status. Enable LOGIN_REFRESH_ENABLED in private settings.
5. Set GovAI realm revokeRefreshToken=true and refreshTokenMaxReuse=0. This is a
   realm-wide behavior change, including other clients in GovAI; they must serialize
   refresh and replace rotated tokens. Do not silently make this change.
6. Enable prepared web settings, retain 300s access tokens and a maximum 30m local
   browser session, then gracefully restart only the owned Go API on port 5001.
7. Verify readiness, invalid-credential errors and Postman registration/login/session/
   refresh/logout with synthetic accounts only. Do not request or display real
   passwords/tokens. Check safe recovery counts. Real browser/mobile work remains
   with client owners.

The existing prototype password-grant/RFC 9700 exception remains documented in the
GA-48 plan; this rollout does not silently change the chosen architecture.

## Scope and rollback

No commit/push/merge is authorized by this rollout. Shared Docker configuration,
frontend/mobile, Python/RAG, existing users and registration privileges remain
untouched. No actual AI upstream exists; authenticated chat remains a 501 placeholder.

Rollback first disables private WEB_SESSION_ENABLED and LOGIN_REFRESH_ENABLED and
restarts the owned API, then restores only the saved owned-client/realm settings if
needed. Additive schema remains; do not drop it or its records as a rollback shortcut.
Private web encryption keys must remain stable and backed up outside Git.

Explicit approval was received and the local rollout above was executed on
2026-10-31. Migrations/configuration/readiness checks passed; five existing users
were preserved. Valid-account local Postman confirmation and real client
integration remain pending. No commit, push or merge was performed.
