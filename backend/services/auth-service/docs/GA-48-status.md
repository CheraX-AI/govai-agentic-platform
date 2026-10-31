# GA-48 registration and login - implementation status

Status: IN PROGRESS. This checkpoint contains registration components and tests;
it does not complete the full citizen registration/login story or real clients.
The local HTTPS registration API is explicitly enabled and smoke-tested; login
now has a mobile/Postman API. Subsequent [GA-50 work](GA-50-status.md) adds
locally enabled refresh/revocation and browser sessions with verified Postman
flows; actual web/mobile application integration remains pending.

## Implemented in this checkpoint

- Strict request parsing and validation for Unicode name, approved old/new NIC
  syntax, international phone numbers, email and transient passwords.
- Restricted-group Keycloak registration adapter with validated HTTPS, token
  caching, bounded calls, safe errors and operation-marker reconciliation.
- ProfileStore and RegistrationJournal interfaces, registration orchestration,
  synthetic failure/retry tests and an unwired registration HTTP handler.
- Bounded process-local IP/email/concurrency admission controls.
- [Mobile/Postman login](GA-48-login.md) with separate non-admin client, access-token
  signature/issuer/audience/expiry and citizen/verified-email checks, owned profile
  lookup, generic errors, no-store responses and separate bounded admission.
- Opt-in permission/lifecycle/combined test harnesses and reproduction notes.
- Authentication PostgreSQL schema/migration, restricted runtime role, encrypted
  ProfileStore/RegistrationJournal adapters, database checks and server readiness.
  See [database report](GA-48-postgresql.md) for current evidence and limitations.
- [Real PostgreSQL + HTTPS Keycloak/SMTP integration](GA-48-postgresql-keycloak-integration.md)
  with the HTTP handler on a private test listener: success, duplicates, profile
  failure recovery, lost-create-result reconciliation and SMTP uncertainty pass.
- Opt-in runtime composition and HTTPS-only registration route, configuration
  validation, stable-key generator, direct TLS checks and registration-specific
  CORS. [Local HTTPS/client setup](../../../dev/keycloak/HTTPS-SETUP.md) now preserves
  existing users and passes actual API/profile/citizen/Mailpit smoke checks.

Historical live evidence is recorded in the linked experiment/lifecycle documents.
The previous commit preparation reran offline tests, vet and race checks only.
The subsequent 2026-10-31 database implementation ran real PostgreSQL tests and
server outage/readiness checks; see the database report. A subsequent explicit
combined live run also verified real PostgreSQL with HTTPS Keycloak/SMTP.

## Accepted prototype limitation

The tested group-scoped administrative account can manage other citizens in its
registration group, including their credentials. It is not create-only or
per-operation isolation. Historical FAIL_UNSAFE assertions are preserved for the
strict isolation requirement. The prototype accepts this documented authority;
it is not a public citizen permission or a production-security certification.

## Remaining GA-48 work

1. Review the implemented authentication schema and adapters with the team.
2. Extend the passing real PostgreSQL + Keycloak/SMTP integration with independent
   process kills at every checkpoint and uncertain commits, then implement
   operational reconciliation. Current tests cover independent pools, atomic NIC
   claims, pool/service reopen and backend connection termination, not all crash modes.
3. Test deployment TLS, secret backups/restoration and planned key rotation before
   using real citizen data. Local synthetic database evidence is not a deployment audit.
4. Review the enabled local HTTPS prototype, configure intended deployment and
   actual web/mobile verification destinations, and complete real client tests.
   The development callback currently returns to Keycloak's account console.
5. Review server timeout budget, POST/idempotency-header CORS, trusted proxies,
   deployed TLS and distributed rate limiting/abuse behavior.
6. Preserve/secure the configured local scoped credentials and review deployed
   provisioning. Hosted registration is now off in the local GovAI realm, avoiding
   required-profile bypass. Accepted group-wide authority remains documented.
7. Complete actual native-mobile integration and deployment
   review. The new access-token login API and isolated real provider/database test
   cover verified email, pending actions, generic failures and safe token validation.
8. Agree web/mobile token custody and delivery contracts; verify actual clients
   with their owners without editing protected teammate files.
9. Complete end-to-end registration/login evidence, documentation and PR review.

Login's Direct Grant conflict with RFC 9700 remains documented in the architecture
plan. HTTPS does not remove that protocol limitation. The registration permission
experiments do not resolve it.

JWT/RBAC middleware and protected chat belong to subtask 3; the reusable middleware
and guarded placeholder now have [separate implementation/evidence](authorization.md).
The actual AI/RAG functionality remains unimplemented. Refresh/logout and
session lifecycle belong to subtask 4. Password recovery/theme work stays deferred.

## Git hygiene

Commit Go source, synthetic tests, go.mod/go.sum, reproducible harness source and
redacted documentation. Do not commit real environment files, administrative
credentials, tokens/action links, private keys, local TLS directories, experiment
state/manifests, database dumps, inbox exports, generated binaries or test reports.
Existing named Docker volumes and test resources are not Git source files.
