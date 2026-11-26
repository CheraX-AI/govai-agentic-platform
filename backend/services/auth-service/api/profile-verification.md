# GA-256 citizen profile verification

Verification date: 26 November 2026. Scope: GA-199 and GA-252/253/254/255/288.

Automated checks below passed. GA-256 still has external integration and device
acceptance checks outstanding; this record is not a production-readiness sign-off.
No production application logic or database schema is changed by this test task.

## New regression coverage

- Auth-service `internal/http/handlers/profile_flow_integration_test.go` joins
  real PostgreSQL migrations, restricted application-role persistence, profile
  services, native JWT verification, browser/native HTTP handlers and the S3
  adapter in one flow. Two synthetic citizens are registered through the real
  registration journal/persistence transitions. Keycloak itself is not contacted;
  the browser identity/session dependency is a test double.
- The flow verifies web edits visible through the native API, native edits
  visible through the web API, stale revision rejection, read-only NIC protection,
  photo visibility across adapters, owner isolation, failed-upload reference
  preservation, a separate database connection reading persisted details, and
  absence of plaintext profile values in encrypted storage and request logs.
- Storage requests use the real signed S3 adapter against an HTTP fixture. The
  fixture checks signing headers; it does not validate signatures or prove
  SeaweedFS bucket policy, deployment credentials or storage durability.
- Core-service `internal/httpapi/profile_privacy_test.go` verifies that profile
  fields in JWT claims, extra headers or injected chat JSON do not reach the AI
  payload. The real HTTP agent adapter sends only question/conversation fields
  and its service credential to a synthetic HTTPS endpoint. Citizen tokens and
  profile values are excluded from the observed request logs.
- Web `scripts/integration/check-profile-browser.mjs` additionally checks an
  initial profile load failure followed by explicit retry, safe error text, and
  invalid photo type/size rejection without an upload request.

## Acceptance evidence and limits

| Story criterion | Automated evidence | Remaining acceptance work |
| --- | --- | --- |
| 1. Linked profile and migration | PostgreSQL registration journal/persistence flow; existing migration, backfill, grants and replay tests | Real registration through deployed Keycloak |
| 2–3. Permitted edits and protected identity | Both HTTP adapters, validation/read-only tests, browser form and mobile transport/controller checks | Authenticated client runs against the deployed services |
| 4. Persistence and cross-client consistency | Real database read through a second store; web/native API round trip; browser fixture reload | Actual logout/login and web-to-phone round trip |
| 5. Revision and DOB/NIC rules | HTTP/service rule tests, PostgreSQL CAS tests, browser/mobile conflict preservation | Confirm agreed rules remain the release contract |
| 6. Photo limits and replacement | Existing decode/size/type/metadata/replacement/cleanup tests; new S3-plus-database failure flow; client tests | Real SeaweedFS upload, replacement, permissions and outage recovery |
| 7. Ownership | Existing JWT/session/CSRF and owner tests; two-citizen persisted profile/photo isolation | Two live accounts on web and phone |
| 8. Encryption and AI privacy | PostgreSQL ciphertext tests; AI boundary excludes profile fields/claims/headers | Define and verify any required masking of personal names typed into chat |
| 9. UI and errors | Headless web checks at 1440, 1024, 768 and 390 px widths; mobile type/controller checks | Final design review, phone keyboard, large text, safe areas and native photo picker |
| 10. Tests and unresolved integrations | Suites listed below pass; limitations recorded here | Complete external and device checks before closing all acceptance criteria |

The user reported that the mobile synthetic preview opened successfully on their
phone. That preview has no authentication, network calls or permanent saves. It
does not establish a connected mobile profile flow, native photo upload or APK
delivery. No APK is confirmed by this task.

## Checks run

Tests ran on an isolated source copy. PostgreSQL 17 used a disposable container
without a published port; the Go runner shared its network namespace. Synthetic
test configuration and keys were used. No existing application database or
private environment file was used or migrated.

- Auth-service: `GOVAI_DB_TEST=1 go test ./... -count=1` and `go vet ./...`.
  Opt-in PostgreSQL tests were enabled, including the new cross-adapter flow.
- Core-service: `go test ./... -count=1` and `go vet ./...`. Unrelated opt-in
  external database tests were not enabled.
- Web: `npm.cmd run build`, `npm.cmd run lint`,
  `node scripts/integration/check-profile-api.mjs`, and the headless profile
  browser script with a loopback frontend relay and synthetic API responses.
- Mobile: `npm.cmd run typecheck` and all six
  `scripts/integration/check-*.cjs` scripts. These use synthetic dependencies;
  they are not native device rendering or accessibility tests.

## Repeating the checks

Use the commands above from the corresponding service or client directory.
For browser fixtures, start Vite on `http://127.0.0.1:5176` in a test copy, then:

```powershell
$env:GOVAI_FIXTURE_FRONTEND_URL = 'http://127.0.0.1:5176'
node scripts/integration/check-profile-browser.mjs
```

The existing browser harness uses headless Edge and fulfills frontend/API
fixtures locally. Certificate verification is not disabled. Leave the fixture
relay variable unset when performing a separately configured live HTTPS test.

Database integration is explicitly opt-in. Use a disposable PostgreSQL cluster
at `127.0.0.1:55432`, an owner role able to create databases, and the restricted
`govai_auth_app` role. Supply synthetic `.env` (`DATABASE_URL`) and `.env.migrate`
(`MIGRATION_DATABASE_URL`) in the auth-service test copy. Do not substitute a
shared development or production database. Tests create, migrate and drop only
random `govai_auth_test_` databases; the configured database is not migrated.

## Required live acceptance run

1. Register synthetic citizen A using the actual authentication stack, then
   log out/in and verify their linked profile on web and Android.
2. Save approved fields on web, refresh on Android, then reverse the direction.
   Keep one stale draft and verify HTTP 409 preserves edits pending explicit
   reconciliation. Confirm NIC, registered name and mobile remain read-only.
3. Use actual private SeaweedFS storage to upload and replace a JPEG/PNG. Reject
   oversized, invalid or forged content. Simulate a storage failure and verify
   the prior saved image/reference remains usable. Review cleanup/orphan handling
   described in `profile-photo.md`.
4. Repeat profile/photo operations as citizen B, including attempts to supply
   A's identifiers. Verify no access to A's record or photo and verify browser
   CSRF/session rejection and mobile expired-token handling.
5. Review both screens against approved frames. On Android test scrolling,
   keyboard, enlarged system text, picker cancellation/permission denial,
   network errors and app interruption during photo selection/upload.
6. Confirm the AI privacy policy with the owner. Stored profile values are
   excluded by the tested contract. Recognition/redaction of Sinhala/Tamil
   personal names embedded in a user's question is not implemented or verified
   by these tests; do not mark that requirement passed without an agreed rule
   and matching implementation/test evidence.

Real SeaweedFS, live Keycloak and connected cross-device end-to-end acceptance
remain unverified. The previously noted Expo patch compatibility warning is
outside this test change; no SDK upgrade or dependency installation was made.
