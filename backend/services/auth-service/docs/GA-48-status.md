# GA-48 registration and login - implementation status

Status: IN PROGRESS. This checkpoint contains registration components and tests;
it does not complete citizen registration/login or enable a public API.

## Implemented in this checkpoint

- Strict request parsing and validation for Unicode name, approved old/new NIC
  syntax, international phone numbers, email and transient passwords.
- Restricted-group Keycloak registration adapter with validated HTTPS, token
  caching, bounded calls, safe errors and operation-marker reconciliation.
- ProfileStore and RegistrationJournal interfaces, registration orchestration,
  synthetic failure/retry tests and an unwired registration HTTP handler.
- Bounded process-local IP/email/concurrency admission controls.
- Opt-in permission/lifecycle/combined test harnesses and reproduction notes.

Historical live evidence is recorded in the linked experiment/lifecycle documents.
This commit preparation reran offline tests, vet and race checks only; no live
Keycloak or PostgreSQL tests were performed during commit preparation.

## Accepted prototype limitation

The tested group-scoped administrative account can manage other citizens in its
registration group, including their credentials. It is not create-only or
per-operation isolation. Historical FAIL_UNSAFE assertions are preserved for the
strict isolation requirement. The prototype accepts this documented authority;
it is not a public citizen permission or a production-security certification.

## Remaining GA-48 work

1. Review the database teammate's schema/migrations against the profile/journal
   contract and agree ownership of Go PostgreSQL adapters.
2. Implement real profile/journal stores with transactions, atomic normalized-NIC
   uniqueness, ownership binding, locking/idempotency and durable recovery.
3. Verify real database concurrency, rollback, process-restart recovery and
   operational reconciliation; synthetic stores do not prove these guarantees.
4. Integrate the handler, orchestration, real stores and group-aware adapter;
   verify failures end to end before wiring the registration route.
5. Review server timeout budget, POST/idempotency-header CORS, trusted proxies,
   deployed TLS and distributed rate limiting/abuse behavior.
6. Configure actual scoped registration credentials outside Git and address
   hosted-registration bypass of required application profiles.
7. Implement password-login API/client configuration under the approved design,
   verified-email/required-action handling and safe error/rate-limit behavior.
8. Agree web/mobile token custody and delivery contracts; verify actual clients
   with their owners without editing protected teammate files.
9. Complete end-to-end registration/login evidence, documentation and PR review.

Login's Direct Grant conflict with RFC 9700 remains documented in the architecture
plan. HTTPS does not remove that protocol limitation. The registration permission
experiments do not resolve it.

JWT/RBAC middleware and protected chat belong to subtask 3. Refresh/logout and
session lifecycle belong to subtask 4. Password recovery/theme work stays deferred.

## Git hygiene

Commit Go source, synthetic tests, go.mod/go.sum, reproducible harness source and
redacted documentation. Do not commit real environment files, administrative
credentials, tokens/action links, private keys, local TLS directories, experiment
state/manifests, database dumps, inbox exports, generated binaries or test reports.
Existing named Docker volumes and test resources are not Git source files.
