# GA-50 implementation and verification status

Date: 2026-10-31. Backend/local verification complete for the implemented scope.
The Jira subtask is not declared Done: actual web/mobile integration, review and
merge remain pending. Delivery is on the GA-18 citizen-registration/login feature
branch with a GA-50-prefixed commit; Git records the final commit/push state.

## Delivered

- Opt-in native login refresh-token issuance, refresh and RFC 7009 revocation.
- Keycloak-issued JWT validation, verified citizen policy and owned profile checks
  before releasing refreshed credentials; rotated tokens are never blindly retried.
- Encrypted PostgreSQL browser sessions, opaque Secure/HttpOnly/SameSite Strict
  cookies, exact-origin credentialed CORS and synchronizer CSRF protection.
- Explicit browser login/session/CSRF/refresh/logout APIs with process-local abuse
  limits and no provider tokens in browser JSON responses.
- Additive migrations 002/003, restricted runtime SQL privileges, revision/claim
  concurrency checks and fail-closed ambiguous refresh handling.
- Atomic local logout plus encrypted provider-revocation recovery queue; bounded
  reconciliation, client-filtered status and owner-only completed-record cleanup.
- Approved local GovAI rollout, private settings backups, independent web client
  and refresh rotation; original five users were preserved.

## Verification evidence

Automated: full auth-service tests/vet/race checks, shared authn module tests/vet,
real disposable PostgreSQL race checks and combined Keycloak 26.8.1/PostgreSQL/TLS
lifecycle checks passed. These cover token rotation/reuse, real provider expiry,
client isolation, encrypted custody, stale ownership, concurrent claims, CSRF,
local logout and persisted provider-revocation recovery. Final review fixed the
logout/refresh snapshot race and pending-session logout after reload.

Local Postman: user reports completion; the agent-owned API's request metadata
independently confirms the following statuses. No credential/token payload was
collected or recorded from those manual requests.

| Request | Observed status |
|---|---|
| POST /api/v1/auth/login | 200 |
| POST /api/v1/auth/refresh | 200 |
| POST /api/v1/auth/revoke | 204 |
| Subsequent POST /api/v1/auth/refresh | 401 |
| POST /api/v1/web/login | 200 |
| GET /api/v1/web/session | 200 |
| GET /api/v1/web/csrf | 200 |
| POST /api/v1/web/refresh | 204 |
| POST /api/v1/web/logout | 204 |
| Subsequent GET /api/v1/web/session | 401 |

Schema readiness passed after manual verification. Safe queue status showed
pending=0, running=0, completed=1, attention=0. Only metadata/counts were inspected.
The local API remains running at https://localhost:5001.

## Remaining work and boundaries

- Teammates must integrate and verify actual React and native mobile clients.
  Postman is not evidence of browser SameSite/CORS enforcement or mobile secure
  storage. The currently approved origin is backend-only https://localhost:5001.
- Real frontend HTTPS origin/deployment and protected downstream browser routes
  must be agreed and tested. Existing bearer /me/chat routes are not a complete
  cookie-authenticated web application gateway.
- Logout here ends local browser custody and attempts client refresh revocation;
  it does not promise global SSO/device logout or immediate offline JWT revocation.
  Previously issued access JWTs can remain valid until expiry.
- The approved prototype password-grant/RFC 9700 exception remains documented in
  the GA-48 architecture plan. This is not a production compliance claim.
- Unknown provider responses or persistent database failure can still leave an
  unknown rotated credential until expiry. The recovery harness simulates a
  checkpoint interruption, not a full process-kill/disaster-recovery campaign.
- Rate limits are process-local. Maintenance is explicit; no scheduler or alert
  delivery is configured. Key rotation/backups and deployment policies require
  operational review before use beyond this local prototype.
- Real AI/RAG chat integration is pending; the protected chat route remains 501.
- PR review and merge remain pending; this implementation does not mark Jira Done.

No frontend/mobile/Python/RAG/shared Docker files were changed. Private .env,
TLS keys and rollout backups remain ignored by Git.

References: [implementation details](GA-50-token-management.md),
[local Postman guide](GA-50-local-postman.md),
[rollout review](GA-50-local-rollout-review.md).
