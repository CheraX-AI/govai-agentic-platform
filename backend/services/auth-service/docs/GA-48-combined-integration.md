# GA-48 combined orchestration/Keycloak integration

2026-10-30: the opt-in combined integration test passed against a **fresh Keycloak
26.8.1 HTTPS instance and separate Mailpit**, using the real group-aware adapter
and registration service with **explicitly synthetic, process-local stores**.
This proves component integration under those store assumptions, not database
durability, distributed concurrency, restart recovery or completion of GA-48.

The accepted prototype limitation remains: the administrative credential can
manage other users/credentials throughout the registration group. That authority
is not public-user permission or create-only/per-operation isolation. Historical
FAIL_UNSAFE permission assertions are unchanged and were not relabelled as passing.

## Reproduce safely

Prerequisites: Docker Desktop's Linux engine, Go, and the existing pinned images
used by the disposable harness. From the repository root:

```powershell
cd backend/services/auth-service
$ga48CombinedState = Join-Path $env:TEMP ('govai-ga48-combined-' + [guid]::NewGuid().ToString('N'))
try {
    $env:GA48_COMBINED_LIVE = '1'
    $env:GA48_COMBINED_STATE_DIR = $ga48CombinedState
    go test -run '^TestCombinedLive$' -count=1 -v ./internal/permissionlab
} finally {
    Remove-Item -LiteralPath Env:\GA48_COMBINED_LIVE -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath Env:\GA48_COMBINED_STATE_DIR -ErrorAction SilentlyContinue
}
```

The new state directory must be private and outside Git, with an existing parent.
An existing directory is refused, never overwritten. `-count=1` prevents cached
results from being mistaken for a new live run. The command uses an eight-minute
scenario deadline. It prints safe case IDs/results, not bodies, accounts, NICs,
credentials, tokens, action links or profile data. Do not enable HTTP tracing,
Docker debug dumps, terminal transcripts or full realm/container exports.

Without GA48_COMBINED_LIVE=1, ordinary `go test ./...` **skips the live test**.
Missing state configuration fails safely before infrastructure setup. There is no
new `cmd/api` route or combined CLI persistence mode: synthetic stores exist only
in `_test.go` files and are excluded from application builds.

The test reuses the disposable launcher: random labelled Keycloak/Mailpit
containers, independent volumes/network, loopback-only random published ports,
validated HTTPS for identity/admin calls, a per-run private CA, and no system
trust-store changes. Local synthetic Mailpit HTTP/SMTP remain confined to this
explicit test setup. Existing GovAI, existing users, shared volumes and the
existing Mailpit are never addressed. Both test containers stop on completion;
their volumes/messages, network and private TLS assets remain. Nothing is deleted.
The non-secret resource manifest is in `$ga48CombinedState/resources.json`.
Keep that directory private: it contains a server TLS private key. As in prior
experiments, Docker administrators can inspect the random bootstrap service-account
secret in container environment metadata; never publish full inspect output.

## Components and boundaries exercised

`registration.Service.Register` calls the real strict ParseRequest, reserves the
synthetic operation/NIC, commits a creating checkpoint, and calls the real
`keycloak.Admin.CreateDisabled`. The adapter supplies exactly the configured
`groups: ["/ga48-registration"]`, disabled/unverified state and server-generated
operation marker. Profile persistence is injected and password-free. Subsequent
owned reads, enabling and verification-mail requests also reach the real adapter
and live Keycloak. A separate setup/observer principal checks actual state.

The tested actor uses only view-members/manage-members/manage-membership on the
fixed group; query-users is added before collection recovery and its effective
realm-management roles are checked. There is no manage-users/view-users/realm-admin
grant. The initial successful registration follows the established no-query-users
provisioning order; later cases use the final scoped actor plus query-users.
Citizen is inherited from the realm default composite, not caller-assigned.

The profile-store hook independently verifies real disabled/unverified state,
password credential presence, exact marker/group, required VERIFY_EMAIL action
and effective citizen role **at the profile-write boundary**. The adapter wrapper
asserts a saved creating intent before real creation, a stored profile and enabling
intent before real enabling, and email_sending before real mail requests.

## Live results versus injected assumptions

| Case | Observed result | What is synthetic/injected |
| --- | --- | --- |
| Caller group/role overrides | Validation rejected input before any store reservation or adapter create call | None in validation; empty test stores are inspected |
| Successful full orchestration | Completed receipt; one real enabled/unverified citizen with VERIFY_EMAIL retained; matching verification message in the new Mailpit | Reservation, journal checkpoints and profile commit are process-local test stores |
| Completed same-operation replay with changed password | No additional create/profile/enable/email call; observer found one account, unchanged user representation and credential metadata | Idempotency/session ownership is implemented by synthetic stores |
| Profile-write failure | Service returned registration_unavailable; real account stayed disabled; no enable/email call or stored profile | ProfileStore.Persist deliberately returns an error after observer readback |
| Same-operation retry after profile failure | Completed with the same subject, one create call, one account, two profile attempts and unchanged credential metadata | Same in-process store survives between the two calls |
| Ambiguous creation result | One real account created; service retained creating with no subject and did not enable it | Wrapper discards one successful REAL adapter result and returns AdminAmbiguous/UnknownOutcome |
| Recovery of that ambiguity | Resume used real FindOwned(marker), then persisted/enabled/requested email; one create call, one marker search and one real account; credential metadata unchanged | Outcome-loss injection is at the adapter return boundary, **not an actual TCP timeout**; pending state is synthetic |
| Email failure | Real SMTP connection failure returned through Keycloak/adapter; profile and enabled/unverified identity remained; service retained email_sending and reconciliation_required | Only this disposable realm's SMTP port is temporarily set to closed port 1 |
| After restoring SMTP | Resume did not send again or create again; failure recipient had no message in the private inbox | Operator/provider reconciliation is not implemented; conservative uncertainty is expected |
| TLS validation | Real untrusted-CA and wrong-hostname probes were rejected | Per-run local CA, not a deployed certificate infrastructure test |
| Redacted output | No service/adapter structured logs emitted; fixed case results only; arbitrary panic text is scrubbed by a tested guard | Offline guard test injects private text to verify it is not printed |

The observer compares credential **metadata**, without password login or reading
passwords/hashes from application storage. The combined test does not navigate
verification links, redeem authorization codes or test login/JWT/chat access.
It verifies delivery and required actions; earlier lifecycle evidence separately
covers verification completion and invalid/expired/reused links.

Counts are adapter method-call counts plus independent real-account observations,
not a claim of exactly-once HTTP or SMTP delivery. The ambiguous-result case proves
the service's reconciliation branch against real marker lookup and identity state;
it does not reproduce every network timeout/crash. The email upstream failure is
deliberately treated as uncertain even though the synthetic inbox contained no
matching failed message. The adapter's safe outcome, rather than a raw HTTP body,
is inspected by the orchestration test.
No identity/profile rollback or blind resend occurs. Pending email needs reviewed
operational reconciliation before any deliberate resend.

## What remains unproven

The combined stores are mutex/channel-backed **test-only doubles**, with no files,
SQL, durable leases, encryption at rest, database transactions or external process
coordination. Their behavior must not be cited as evidence for:

- Atomic PostgreSQL journal/NIC reservation or normalized-NIC unique constraints.
- Durable profile ownership/binding, revision CAS or atomic commit behavior.
- Cross-process concurrency, lock-loss fencing, failover or stale-owner cancellation.
- Process restart/crash recovery: Resume here uses the same process-local records.
- Durable retry schedules, operator reconciliation or notification delivery state.

On test exit these synthetic profiles/journals disappear while the disposable
Keycloak volume is retained. This is intentional test infrastructure, never an
acceptable runtime persistence fallback. The exact database requirements remain
in [the orchestration contract](GA-48-registration-orchestration.md). Implement and
test real stores against disposable PostgreSQL with independent connections/processes
before enabling registration. Runtime config/secret loading, rate limiting, deployed
SMTP/TLS, real web/mobile integration and the remaining auth features are still
separate dependencies. No registration route, runtime store, PostgreSQL, login,
teammate-owned file, commit or push was added.

## Affected files and checks

- `internal/permissionlab/lab.go`: private scenario hook reusing the existing
  disposable environment; baseline/group/lifecycle dispatch and verdicts preserved.
- `combined_live_test.go`: opt-in live scenario, real-adapter call instrumentation
  and one explicit lost-result fault; no mock identity server.
- `combined_stores_test.go`: explicitly synthetic journal/profile/session doubles.
- `combined_test.go`: offline redaction guard and synthetic contract safety tests.
- Backend README/GA-48 documentation link the new integration evidence.

Two fresh combined runs completed successfully; the final run includes validation
override rejection and credential preservation through reconciliation. Both reported
LAB_COMBINED PASS and stopped their own containers. Existing GovAI/Mailpit remained
stopped. After final code changes, gofmt, `go test ./...`, `go vet ./...`,
`go test -race ./...` and `git diff --check` passed. These full-module unit/race
runs were offline and skipped the combined live scenario by default; the separate
explicit live command above is the source of live component-integration evidence.
