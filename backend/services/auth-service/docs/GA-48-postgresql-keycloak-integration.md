# GA-48 real PostgreSQL + Keycloak integration

This opt-in test connects the existing registration HTTP handler and orchestration
to the real PostgreSQL adapters and the real group-aware HTTPS Keycloak adapter.
It uses a newly created synthetic PostgreSQL database, a fresh disposable Keycloak
realm/container and separate Mailpit. The existing `govai_auth` database, GovAI
realm, server routes and shared infrastructure are not modified.

This evidence is separate from [the earlier combined test](GA-48-combined-integration.md),
which intentionally continues using synthetic stores. Historical permission
FAIL_UNSAFE assertions remain unchanged. The prototype still accepts group-wide
administrative authority, including credential mutation of other registration-group
users; this test does not establish per-operation administrative isolation.

## Run

First complete [local PostgreSQL setup](../../../dev/auth-postgres/README.md).
Start Docker Desktop, then run from the auth-service directory:

```powershell
$env:GA48_POSTGRES_LIVE = '1'
$env:GA48_POSTGRES_STATE_DIR = Join-Path $env:TEMP ('govai-ga48-db-' + [guid]::NewGuid().ToString('N'))
try {
    go test -run '^TestPostgresKeycloakLive$' -count=1 -v ./internal/permissionlab
} finally {
    Remove-Item Env:GA48_POSTGRES_LIVE -ErrorAction SilentlyContinue
    Remove-Item Env:GA48_POSTGRES_STATE_DIR -ErrorAction SilentlyContinue
}
```

The test has an eight-minute scenario deadline. Normal `go test ./...` skips it.
It reads local private runtime/admin database settings and refuses hosts/ports
other than the isolated `127.0.0.1:55432` setup. It creates a random
`govai_auth_live_*` database, applies migrations, uses restricted runtime
connections and fresh independent encryption/index/fingerprint keys. Cleanup
drops **only the test-created database**, never the configured application database.

The pre-existing permission-lab launcher provides validated HTTPS, a per-run CA,
loopback ports and isolated containers/volumes/network. No system certificate
trust changes or TLS verification bypass is used. Keycloak/Mailpit containers
stop after the test; their volumes/network and private state directory remain.
Do not commit that state directory: it contains a TLS private key. The retained
test realm's accounts have no application profiles after disposable SQL cleanup;
this intentionally isolated test realm is not a working application environment.

## Cases and limits

| Case | Assertion |
| --- | --- |
| Private HTTPS registration request | Real HTTP handler returns generic 202/no-store; real journal completes and profile exists; Keycloak account is enabled/unverified with citizen and VERIFY_EMAIL |
| Email delivery | Separate live Mailpit contains the verification message |
| Pool/service reopen, same-key replay with changed password | Same operation/subject/receipt; no new create, enable or verification call; credential metadata unchanged |
| Same normalized NIC, another key/email | Same generic receipt; no Keycloak create call and no account for the second email |
| Same email, another NIC | Live Keycloak duplicate response; terminal duplicate journal with no adopted subject/profile and unchanged original account credentials |
| Profile failure before SQL commit | Injected one-shot ProfileStore error; real account stays disabled; real journal retains subject_recorded |
| Password-free recovery after pool/service reopen | Resume reads encrypted SQL pending data and commits profile/enabling/mail without another create or original password |
| Lost successful create result | Wrapper deliberately discards a real success; SQL retains creating; after reopen, real marker lookup reconciles without another create |
| SMTP failure | Real closed-port SMTP failure leaves enabled/unverified account, durable profile and email_sending intent |
| SMTP recovery after pool/service reopen | An uncertain send is not automatically resent and creation is not repeated |
| Administrative scope | Actor has only query-users plus the previously accepted registration-group scopes, never broad manage-users |
| Runtime composition | Real application constructor and production router complete registration over a trusted private TLS listener |
| Output | Fixed case IDs; runtime request log contains only allowlisted request metadata, never bodies/credentials |

The HTTP listener is an **in-process private httptest TLS server**. It is not
`cmd/api`, a deployed ingress, the public registration route, or an actual
React/mobile client. Rate limiting in this HTTP fixture remains process-local.
Pool/service reopen is not an OS process-kill, host reboot, backup restore or
database failover experiment. The lost-create-result injection is at the adapter
return boundary, not a real TCP timeout. Credential metadata comparison is not a
password-login test. No exactly-once identity/SMTP guarantee is claimed.

## Remaining integration

Public runtime activation still needs private HTTPS Keycloak configuration,
stable fingerprint and abuse-limit keys, an agreed verification callback/client,
timeout/CORS/proxy review and removal of hosted-registration bypass of required
application profiles. Do not point the adapter at the existing HTTP dev realm by
disabling certificate checks. It deliberately requires HTTPS.

Login remains separate. JWT/RBAC and refresh/logout remain their later subtasks.
Uncertain operations need an explicit operational reconciliation procedure;
there is no automatic blind retry worker. Full process-crash/restore tests and
real web/mobile evidence are still pending.

## Execution evidence

2026-10-31: the explicit live test passed in approximately 32 seconds. Every case
above reported PASS, including SQL cleanup, both container stops and empty
service/adapter logs in the original run. The test-created database was removed; existing `govai_auth`
and GovAI/Mailpit settings/data were preserved. Only test code and documentation
were added for this slice; no public runtime route or configuration was enabled.

A subsequent 2026-10-31 run also passed `POSTGRES_RUNTIME_COMPOSITION` using the
new real application constructor and production router. The safe-log assertion
now verifies the exact request-metadata schema rather than expecting empty logs,
because the router intentionally logs one HTTP request. This second run took
approximately 32 seconds and again passed cleanup/container-stop checks. Runtime
code now supports opt-in activation; the user's environment remains disabled.
