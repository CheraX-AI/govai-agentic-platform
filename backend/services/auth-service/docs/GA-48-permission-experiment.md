# GA-48 isolated permission experiment

Follow-up: the [group-scoped experiment](GA-48-group-permission-experiment.md) now
demonstrates creation and required operations without broad manage-users, while
retaining credential authority across all users in the registration group. Its
strict operation-isolation verdict also remains unsafe. Results below describe
the original group-free baseline slice; `-experiment groups` selects the follow-up.

2026-10-29: live permission experiment completed against Keycloak **26.8.1**.
The result is **inadequate isolation for the current registration design**. No
configuration tested both permitted group-free account creation and prevented
mutation of unrelated/admin users. Do not enable public registration on this basis.

The locally tested image digest was
`quay.io/keycloak/keycloak@sha256:a78d6b79572168cb25886b80a8b5b8d4ed4390743e3eecb810538683e209dd6b`.

## Files and scope

- `cmd/permission-lab/main.go`: explicit opt-in command, not an API dependency.
- `internal/permissionlab/lab.go`: isolated Docker setup and direct permission
  probes, exercising the existing registration adapter for create/read/search/enable.
- `internal/permissionlab/lab_test.go`: offline boundary, certificate, transport,
  redaction and assertion tests. Ordinary `go test ./...` never starts Docker.

The command creates a random labelled Keycloak container, volume, bridge network
and two random realms. It never selects an existing Keycloak endpoint. The HTTPS
port is assigned dynamically and bound only to 127.0.0.1. There is no HTTP listener.
All Admin API traffic uses validated HTTPS and a per-run CA trusted only by the
harness. No host trust-store changes or TLS verification bypasses are used.
Certificates expire after 24 hours; the CA signing key stays only in memory.
Keycloak uses its own dev-file storage, never PostgreSQL or the GovAI volume.

The environment uses synthetic identities only. The privileged setup/observer
service account is separate from the tested principals and belongs to the new
instance's master realm. It performs setup and reads resulting state; the tested
clients never receive its token or secret. This observer is intentionally a trusted
test oracle, not a proposed application credential. No end-user password grant,
password-login API, credential recovery feature or lifecycle suite is implemented.

In baseline mode, no Mailpit container is used or contacted. The email **permission gate** is probed
with a nonexistent action client, expecting 400 after authorization; it proves
neither SMTP delivery nor verification. Existing Mailpit messages are untouched.

## Reproduce in PowerShell

Prerequisites: Go, Docker Desktop with the Linux engine running, and the local
image `quay.io/keycloak/keycloak:26.8.1` (Docker may pull it when absent). Version
26.8.1 is mandatory for this experiment; do not substitute another version.

From the repository root:

```powershell
cd backend/services/auth-service
$ga48State = Join-Path $env:TEMP ('govai-ga48-permission-' + [guid]::NewGuid().ToString('N'))
go run ./cmd/permission-lab -allow-disposable-experiment -state-dir $ga48State
$ga48Exit = $LASTEXITCODE
```

Use a **new directory outside every Git working tree**, under a private user-owned
temporary directory. The harness refuses an existing directory and checks for Git
ancestors. It does not search or load `.env` files, accept a Keycloak URL, or use
shared Compose. The opt-in flag and state path are required. Do not run it with
shell tracing, HTTP debugging, or a terminal transcript.

Output contains fixed case IDs and results only. Exit 0 means all assertions passed;
exit 2 means completed but unsafe/incomplete privilege isolation; exit 1 means an
infrastructure/protocol/ambiguous-outcome block. `go run` reports `exit status 2`
but itself commonly returns 1; use the printed `LAB_RELEASE` verdict, or build
the binary under the ignored backend `bin/` directory if exact process exit codes
are needed. A successful command launch is not a successful security experiment.

`resources.json` in `$ga48State` records only new container/network/volume names,
image and local endpoint. `ca.pem`, `server.pem` and `server-key.pem` remain there;
the directory must remain private. No access token, submitted password or client
secret is written to the manifest or source tree. Keycloak necessarily stores its
own credentials/hashes in the new volume. Docker stores the random bootstrap
service-account secret in container environment metadata; anyone with Docker
administrative access can inspect it. Do not print full `docker inspect`, process
environments, HTTP dumps or realm exports. The CA is not installed system-wide.

The harness stops its newly created container on completion/failure after launch;
it does **not** remove the container, volume, network or TLS files. If interrupted,
use only the name in that run's manifest to stop it after verifying its label.
No automatic deletion/cleanup is provided. Do not run prune, shared Compose down,
or volume deletion. Every rerun creates independent resources and fresh credentials.

Offline checks from the same module:

```powershell
go test ./...
go vet ./...
go test -race ./...
```

## Configurations actually tested

Common realm setup: SSL required, no self-registration, Verify Email enabled,
password policy `length(8)`, no SMTP and no captured admin representations. First
and last names are not mandatory. `govai_registration_operation` is a declared
single-value attribute with admin-only view/edit. This experiment did not test
end-user marker immutability or authoritative password-policy boundaries.

1. **BASE — broad baseline, never least privilege:** service account with only
   `realm-management/manage-users` added; normal default roles remain. No extra
   realm-admin, impersonation or client-management role is granted to the actor.
2. **FINE — feasible creation-capable fine-grained v2 candidate:** no broad admin
   roles; user policy selects the candidate service-account subject. A Users
   resource-type permission grants `view`, `manage`, `reset-password` on all users.
   No `map-roles`, role mapping permissions, impersonation, group, client or realm
   management permissions are added. Full token scope is allowed in this synthetic
   setup; it does not itself grant administrative roles.
3. **SCOPED — tightened candidate:** update the same fine-grained permission to
   cover one already-created owned subject only. Re-test creation and direct
   unrelated/admin mutation denials. This is a feasibility experiment, not a
   preauthorization scheme for unknown future subjects.

All creation payloads are group-free, matching the current adapter. The tested
admin victims are separate synthetic accounts carrying realm-admin inside the
disposable target realm; no real/master administrator is attacked.

## Observed live results

`PASS` for a negative probe means HTTP 403 and, where mutation was attempted,
the observer confirmed unchanged state. `FAIL_UNSAFE` means the mutation returned
204 and the observer confirmed the change. No assertion is changed to an expected
success just because the baseline is known to be broad.

| Case | BASE | FINE | SCOPED |
| --- | --- | --- | --- |
| Create disabled user with initial password | PASS | PASS | FAIL_REQUIRED: 403 |
| Find/read exact operation-owned identity | PASS | PASS | Not re-tested |
| Enable owned identity; observer confirms enabled | PASS | PASS | Not re-tested |
| Verification-email authorization gate, invalid client | PASS: 400 | PASS: 400 | Not re-tested |
| Disable unrelated user | FAIL_UNSAFE | FAIL_UNSAFE | PASS: denied |
| Replace unrelated user's password | FAIL_UNSAFE | FAIL_UNSAFE | PASS: denied |
| Disable synthetic realm-admin user | FAIL_UNSAFE | FAIL_UNSAFE | PASS: denied |
| Replace synthetic realm-admin user's password | FAIL_UNSAFE | FAIL_UNSAFE | PASS: denied |
| Assign realm-management/realm-admin to owned user | PASS: denied | PASS: denied | PASS: denied |
| Assign realm-management/manage-users to owned user | PASS: denied | PASS: denied | PASS: denied |
| Read users in the other disposable realm | PASS: denied | PASS: denied | Not re-tested |
| Create user in the other disposable realm | PASS: denied, observer found none | PASS: denied, observer found none | Not re-tested |

Password changes are verified through before/after credential metadata read by the
observer, not by logging credentials or performing password login. These results
show a successful credential replacement operation and changed metadata; they do
not constitute an end-user authentication test. Role grants are checked through
observer role mappings; the actor does not supply the oracle result.

Two live runs occurred during development. The final expanded run used its own
bridge network and tested both named administrative roles and cross-realm creation.
Both runs produced an unsafe release verdict. All experiment containers are stopped;
their storage remains. The pre-existing GovAI and Mailpit containers stayed stopped,
and no requests were made to them. Docker Desktop was started to run this experiment.

Validation completed after the final harness changes: gofmt, `go test ./...`,
`go vet ./...`, `go test -race ./...` and `git diff --check` passed. The live
experiment's nonzero unsafe verdict is separate from these passing code checks.
No full lifecycle, SMTP, verification-completion, password-policy boundary,
web/mobile, PostgreSQL or public-route checks were run in this slice. Those checks
remain deferred; further dependent lifecycle work is blocked by the permission gate.

## Definite versus ambiguous outcomes

The recorded mutation exposures are definite: successful responses plus independent
state observations. Recorded denials are 403 plus unchanged observed state. A 400
create rejection, timeout, cancellation, lost response, malformed response or 5xx
does not prove rollback. The harness aborts with a safe BLOCKED result on uncertain
probe outcomes, preserves resources and never retries the mutation. Readiness
polling retries only service-account token requests before realm creation; it is
not a mutation retry or a registration recovery implementation.

## Residual capabilities and design choices

Both creation-capable models demonstrably allow unrelated/admin account disabling
and credential replacement. Separate role-assignment denials do not prevent this
account-takeover capability. The operation marker is an application ownership
check, not a Keycloak administrative boundary. No tested permission is create-only.
Only the two named administrative role grants were tested; this is not evidence
that every application role, group inheritance or privilege path is denied.

Practical alternatives for review, without changes to GovAI:

- Return registration to Keycloak-hosted flows and remove public registration's
  need for an administrative credential.
- Investigate group-bound creation with server-controlled group membership and
  fine-grained group permissions. This needs an adapter/design change and separate
  tests: the present payload has no group. A group-scoped actor could still manage
  all members of that group; privileged membership/inheritance must be denied.
- Introduce a narrowly constrained registration bridge/provider that enforces
  create-only operations and owned transitions server-side. A separate credential
  behind an allowlisted bridge reduces exposure only if direct Admin API access
  is also prevented; it does not make a broad credential intrinsically safe.
- Isolate citizen identity into a dedicated realm as a blast-radius measure, while
  acknowledging that a broad actor could still take over citizens within it.

Do not grant these tested all-user permissions in GovAI or begin the lifecycle
suite. Choose and review a narrower architecture before dependent implementation.

Pinned implementation references used to configure the experiment:

- [26.8.1 creation and initial credential permission checks](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/services/resources/admin/UsersResource.java)
- [26.8.1 fine-grained evaluator](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/services/resources/admin/fgap/UserPermissionsV2.java)
- [26.8.1 permission schema](https://github.com/keycloak/keycloak/blob/26.8.1/server-spi-private/src/main/java/org/keycloak/authorization/fgap/AdminPermissionsSchema.java)
- [26.8.1 scope permission examples](https://github.com/keycloak/keycloak/blob/26.8.1/tests/base/src/test/java/org/keycloak/tests/admin/authz/fgap/UserResourceTypePermissionTest.java)
