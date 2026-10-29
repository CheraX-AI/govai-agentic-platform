# GA-48 server-configured registration group

Follow-up: the [combined integration test](GA-48-combined-integration.md) now verifies
this real adapter with registration orchestration against disposable HTTPS Keycloak,
using synthetic stores only. The config-change checks below describe their original
offline scope; no real persistence or deployment-readiness guarantee follows.

Implemented 2026-10-30. RegistrationConfig now requires RegistrationGroupPath.
CreateDisabled sends the exact representation established by the isolated
Keycloak 26.8.1 group/lifecycle experiments:

```json
"groups": ["/ga48-registration"]
```

This is one absolute **group path**, not a group UUID, role, bare name or
caller-provided list. The disabled/unverified state, non-temporary password,
VERIFY_EMAIL action and protected operation marker remain unchanged.

Set this only when constructing the administrative registration adapter:

```go
cfg.RegistrationGroupPath = "/ga48-registration"
admin, err := keycloak.NewRegistrationAdmin(cfg, trustedHTTPClient)
```

All existing TLS, credentials, timeouts and action-client configuration is still
required. No environment loader or runtime startup wiring was added. Do not put
administrative configuration or secrets in web/mobile code or public JSON.

The constructor accepts only one top-level path: slash followed by 1–128 ASCII
letters, digits, underscore or hyphen. It rejects missing/bare/empty paths, nested
or trailing slashes, dot segments, escaping, whitespace, control characters,
Unicode, queries and fragments. This deliberately narrow syntax covers the tested
model; it does not claim support for every Keycloak group-name/hierarchy format.
No trimming, conversion, fallback/default group or group-free production mode.
Invalid configuration returns the existing safe AdminInvalid error before any
network request, without echoing the bad value.

The config is copied at construction. Caller registration input remains the
existing five fields only. The parser rejects group/role configuration fields;
the adapter's explicit serialization allowlist never forwards app-profile text
as group/role/attribute JSON. It assigns no roles. Citizen remains a realm-default
role inherited under the reviewed realm policy.

## Connection to the tested permission model

Previously the live harness supplied the group itself, while the adapter's create
payload was group-free. This change closes that **payload/configuration gap**.
Injecting the configured adapter into the existing registration service now selects
the same group-bearing representation used by those live tests; no orchestration
or persistence interface changed.

The target realm must already contain the intended group and the service account's
fine-grained permission must target its correct resource ID. The tested model uses
query-users plus view-members/manage-members/manage-membership on that group, with
no broad manage-users grant. The constructor validates local configuration syntax,
not remote group existence, role inheritance or actual permissions. These remain
deployment prerequisites verified in isolated integration tests, not permission
to change GovAI. There is no automatic provisioning, permission grant or retry
using a broader account if creation fails.

Signature/TLS verification, response bounds, deadlines/cancellation, redirect
rejection, creation Location validation, safe error mapping, duplicate outcomes,
operation-marker ownership checks and no-retry ambiguous-create handling are
unchanged. Configuration/request logging and JSON serialization remain redacted.

**Accepted prototype limitation remains:** the administrative credential can
manage other users and replace credentials across the registration group.
Membership is not ownership by a particular registration operation, and this is
not create-only isolation or public-user permission. Historical FAIL_UNSAFE
assertions remain intact.

## Harness compatibility and evidence

The group/lifecycle harness constructors now provide the same fixed server path.
Their existing test-only creation probes and permission assertions are preserved.
The historical group-free baseline uses an explicit seven-field test-infrastructure
payload helper for its create probe, with the separate observer resolving the exact
operation marker. It does not enable group-free creation in the production adapter.
The baseline helper has a regression test asserting no groups field. Historical
permission settings and negative assertions were not relaxed.

Synthetic trusted-TLS HTTP tests cover the exact groups array, absence of roles and
profile fields, different server-selected path, config copying, missing/invalid
group configuration without network calls, request override rejection and config
redaction. Existing tests retain TLS rejection, ownership, duplicate, timeout,
cancellation and unsafe Location/ambiguous-create coverage.

No live Keycloak experiment was rerun for this change; the earlier pinned-version
experiments prove the representation/model, while these tests prove the updated
adapter emits it. A combined live adapter/orchestration test with durable stores
has not been performed. Remaining dependencies: real PostgreSQL profile/journal/
normalized-NIC constraints and session locking, recovery/operator policy, secret
and deployment configuration, rate limits, and real client integration. Public
routes remain disconnected. No production settings, permissions, database,
frontend/mobile/Python/RAG files, commits or pushes changed.

Checks completed from the auth-service module: gofmt, `go test ./...`,
`go vet ./...`, `go test -race ./...` and `git diff --check` passed.
The test runs were synthetic/offline; no live permissions or environments changed.
