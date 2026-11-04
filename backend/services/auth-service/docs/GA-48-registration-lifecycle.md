# GA-48 isolated registration lifecycle evidence

Later adapter update: [required group configuration](GA-48-group-adapter-config.md)
now connects the production adapter payload to this tested representation. The
historical results below were obtained using harness-only group-bearing creation;
they are not evidence of a new live run of the updated adapter.

2026-10-29–30 (Asia/Colombo). The user accepted group-wide administrative authority for this
prototype and authorized isolated lifecycle testing. This is a risk acceptance,
not a change to the historical **FAIL_UNSAFE** verdict against per-operation
administrative isolation. The baseline/group permission tests and their assertions
are unchanged. No public routes or GovAI permissions are enabled by this decision.

**Accepted limitation:** a compromised administrative service-account credential
can manage other users and replace credentials throughout the registration group.
Membership is not ownership by a particular registration operation. This authority
belongs to the trusted administrative service account, not to public citizens, and
is not create-only permission. Go's marker/ownership checks remain necessary but
cannot confine a compromised credential making direct Admin API requests.

## Reproduce

With Docker Desktop's Linux engine running, from the repository root:

```powershell
cd backend/services/auth-service
$ga48LifeState = Join-Path $env:TEMP ('govai-ga48-lifecycle-' + [guid]::NewGuid().ToString('N'))
go run ./cmd/permission-lab -allow-disposable-experiment -experiment lifecycle -state-dir $ga48LifeState
```

Use a new, private temporary directory outside Git with an existing parent. The
command creates its own Keycloak 26.8.1 container/storage/network and separate
Mailpit inbox/storage, with random names and loopback ports. It never accepts an
existing Keycloak endpoint, loads shared environment files, or reuses old volumes.
It stops only the newly created containers, preserving all storage/messages and
the private TLS assets/resource manifest. No resources are deleted.

Keycloak/admin/token/action requests use validated HTTPS with the per-run CA,
hostname verification and no redirect following. Mailpit HTTP/SMTP are plaintext
only within this explicitly isolated synthetic environment. Existing GovAI and
Mailpit are never contacted. No real citizen data is used.

Credentials, access tokens, action links and cookies remain private. The harness
does not log or write them to Git, reports, traces, queues or an application store.
Keycloak owns credential storage. Mailpit necessarily retains its generated test
messages; keep the new inbox/storage private. The temporary directory contains a
server TLS private key. Docker administrators can inspect the random bootstrap
client secret in the container environment metadata; never publish full inspect
output, realm exports, terminal transcripts or HTTP/message dumps.

`LAB_LIFECYCLE PASS` means this registration slice passed, **not** that permission
isolation passed or GA-48 is complete. Lifecycle exit 0 is separate from the
unchanged group/baseline experiments, which still return unsafe verdicts. Exit 1
means blocked/ambiguous work; exit 2 means failed lifecycle assertions (with
`go run`, the wrapper may return 1 and print `exit status 2`).

## Exact tested setup

- Fresh synthetic realm: SSL required, duplicate emails disabled, Verify Email
  enabled, password policy `length(8)`, no self-registration or admin-event body
  capture. Optional first/last names match the existing registration contract.
- Fixed, server-selected group `/ga48-registration`; it is not a default group.
- Fine-grained v2 group permission: `view-members`, `manage-members`,
  `manage-membership`, bound only to this group's ID and this service-account
  subject. No broad manage-users/view-users, role-mapping or realm-admin grant.
- The trusted setup/observer is a separate principal. It verifies authoritative
  accounts, group membership, operation markers, credentials metadata and roles.
- The initial create/password-boundary checks use the same provisioning sequence
  as the working group experiment: install group permission, then create with
  no realm-management role. Add only query-users before collection reconciliation
  and subsequent operations, reissue the service token, and verify the effective
  role list contains exactly query-users. Final-scoped creation is also exercised
  by duplicates and the separate expiry-test user.
- A synthetic public verification client has one exact HTTPS return URI. Direct
  Access Grants and implicit grants remain disabled. The harness does not redeem
  authorization codes or implement a login flow.
- The realm default-role composite includes citizen. Creation inherits citizen;
  the registration actor does not assign roles from request data.

Group-bearing creation remains **harness-only**, using a fixed payload allowlist.
The runtime Admin adapter's group-free create method remains unchanged and is not
wired to any handler. Its existing reconciliation, enabling and verification-email
methods are exercised against the synthetic group-created accounts.

## Live results

| Case | Final observed result |
| --- | --- |
| Seven-character initial password | HTTP 400; observer found no account for that unique username |
| Exactly eight-character initial password | HTTP 201; observer found one disabled/unverified account with a password credential |
| Initial state | Fixed group, exact operation marker, VERIFY_EMAIL action and effective citizen role verified by observer |
| Duplicate username with changed email/password/marker | HTTP 409; original user representation, subject/marker and credential metadata unchanged |
| Duplicate email with a different username/password/marker | HTTP 409; one account for the email, original user representation/subject/marker and credential metadata unchanged |
| Marker reconciliation | Returned the expected subject through the scoped service account |
| Enable | Observer confirmed enabled=true, emailVerified=false and VERIFY_EMAIL retained |
| Verification request and delivery | Adapter request succeeded; matching message and action link read privately from the new Mailpit inbox |
| Invalid signature link | HTTP 400 error page; observer confirmed email remained unverified |
| Valid link | Fresh cookie session received a confirmation page; harness followed only the same-origin action-token confirmation. Final response HTTP 302; observer confirmed emailVerified=true and VERIFY_EMAIL removed. External application redirect was not followed. |
| Reused original link in another fresh cookie session | HTTP 200 already-verified informational page; observer confirmed the complete user representation was unchanged |
| Expired link for a separate user | One-second action lifetime, then a three-second wait: HTTP 400 error page; observer confirmed email remained unverified |
| TLS unknown CA | Actual connection rejected with x509 unknown-authority error |
| TLS wrong hostname with trusted CA | Actual connection rejected with x509 hostname error |

Do not describe reuse as HTTP 400/410 or universal one-time rejection: the observed
behavior was a harmless already-verified info response for the tested account.
Replay after email changes or administrator edits was not tested. Generic stale-
link error pages may mention expired or already-completed actions; the harness
reports HTTP error pages conservatively rather than mistaking that wording for
successful verification. Claims about account state come from the separate observer.

The policy test establishes the configured eight-character length boundary only,
not every composition/history/breach policy. The observer's no-account result
supports rejection atomicity for this tested case; it is not a general distributed
rollback guarantee. No mutation is automatically retried after an uncertain result.

An initial provisioning attempt returned 403 for both password-boundary requests
and was correctly recorded as failed, not as password-policy evidence. The harness
was then aligned with the previously tested provisioning order above, without
adding broad privileges. That sequence completed successfully in repeated fresh
environments. Changing provisioning order needs retesting; these results are not
permission to apply settings to an existing/shared realm.

## Evidence limits and persistence dependencies

Duplicate tests prove unchanged Keycloak subject, operation marker and credential
metadata. **They do not prove durable PostgreSQL profile ownership or NIC uniqueness.**
There is still no profile/journal/NIC adapter, cross-process registration lock,
transactional reservation or recovery worker. Synthetic identity tests and this
live realm are not substitutes for disposable PostgreSQL concurrency/crash tests.

Passwords are transient in Go and verified/stored only by Keycloak. No user password
login was performed to test duplicates; credential metadata was compared privately
before/after. No login, JWT middleware, refresh/logout or recovery feature was added.
The test navigates verification action pages only, with bounded reads, a fresh
in-memory cookie jar, exact origin/realm checks and no administrative Authorization
header forwarded to those pages. Action links are never printed or queued.

Still pending: durable profile/journal/NIC uniqueness and recovery contracts;
reviewed group-bearing adapter configuration before runtime use; registration
orchestration/rate limits; real web/mobile integration; login, JWT/chat authorization,
refresh/logout; deployed TLS/SMTP configuration. Notification requirements remain
separate; Keycloak security emails do not use RabbitMQ.

Smallest next implementation step: define the server-only fixed-group adapter
configuration and group-bearing create request contract, with negative tests for
caller group/role overrides, while keeping routes disconnected. Durable profile/
journal reservations remain required before real registration can be enabled.

## Code and checks

New: `internal/permissionlab/lifecycle.go` and `lifecycle_test.go`. Extended only
the opt-in command and shared disposable launcher for `-experiment lifecycle`.
The historical permission experiment functions/tests retain their unsafe assertions.
Backend plan/README documents record this prototype acceptance and evidence.

Offline tests check action origin/realm restrictions, signature tampering, bounded
same-origin confirmation navigation with cookies, no administrative header
forwarding, existing-directory refusal and conservative visible-page reporting.
These are distinct from the live results above. After the final code changes,
gofmt, `go test ./...`, `go vet ./...`, `go test -race ./...` and
`git diff --check` passed. The final live command completed with
`LAB_LIFECYCLE PASS`; it did not rerun or change the historical FAIL_UNSAFE
permission assertions. The initial failed setup and subsequent successful fresh
runs are described above, rather than counted as uniform successes.

Pinned references consulted for the verification action flow:

- [26.8.1 verify-email action handler](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/authentication/actiontoken/verifyemail/VerifyEmailActionTokenHandler.java)
- [26.8.1 confirmation info template](https://github.com/keycloak/keycloak/blob/26.8.1/themes/src/main/resources/theme/base/login/info.ftl)
