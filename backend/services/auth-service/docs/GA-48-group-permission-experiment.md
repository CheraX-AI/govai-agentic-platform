# GA-48 group-scoped permission experiment

Subsequent prototype decision: the user accepted the documented group-wide
administrative authority and authorized isolated [registration lifecycle tests](GA-48-registration-lifecycle.md).
The historical FAIL_UNSAFE verdict and tests below remain unchanged: this is not
per-operation or create-only isolation. Acceptance does not expose public routes.

Live Keycloak **26.8.1**, 2026-10-29. The smallest functional model among those
tested scopes account management to a registration group, **not to a registration
operation**. The service can replace credentials of another operation's user in
that group. The strict release verdict remains **FAIL_UNSAFE**.

No runtime adapter, public route, GovAI permission, login flow, PostgreSQL or client
code changed. Group-bearing creation exists only in this opt-in experiment.

## Reproduce

Start Docker Desktop's Linux engine, then from the repository root:

```powershell
cd backend/services/auth-service
$ga48GroupState = Join-Path $env:TEMP ('govai-ga48-groups-' + [guid]::NewGuid().ToString('N'))
go run ./cmd/permission-lab -allow-disposable-experiment -experiment groups -state-dir $ga48GroupState
```

The state directory must be new, private, outside Git, and have an existing parent.
The command never accepts an existing Keycloak endpoint or overwrites previous
state. Without `-experiment groups`, the original baseline experiment still runs.
Ordinary `go test ./...` never starts live infrastructure.

Each group run creates a fresh labelled Keycloak container, volume and network,
two random test realms and a separate Mailpit container with its own volume/inbox.
The registration group has the fixed synthetic path `/ga48-registration`; it is
not a default group. A separate `ga48-control` group is used only for denial tests.
All accounts and emails are generated synthetic data. Existing resources, including
previous experiment volumes and GovAI's Mailpit, are never reused or deleted.

Keycloak's random published HTTPS port binds only to loopback; its per-run CA is
trusted only by the harness. No HTTP identity endpoint or TLS verification bypass.
The new Mailpit API binds only to a random loopback port; SMTP is confined to the
new network. Mailpit API/SMTP use plaintext solely for this isolated synthetic
test; identity/admin/token calls remain verified HTTPS. The Mailpit image is pinned:
`axllent/mailpit@sha256:0790692be935ea4420d085694f7985b1f692ca6a8771ab0e0bd8ba75ac180498`.

The harness stops only its newly created containers on completion, without removing
containers, volumes, messages, networks or TLS assets. The private `resources.json`
manifest records their names. Existing Mailpit messages are untouched.
Keep the temporary directory private: it includes a server TLS private key.
Tokens/passwords/client secrets never enter source, reports or stdout. As documented
for the baseline, Docker environment metadata contains the random bootstrap client
secret, accessible to Docker administrators. Never print full inspect/configuration
output, realm exports, HTTP bodies or message contents/action links.

Output contains safe case IDs/results. Exit 2 is a completed but unsafe experiment;
`go run` typically exits 1 while printing `exit status 2`. Exit 1 from the harness
means blocked/ambiguous work. Failed isolation assertions are not converted into
passes just because broad authority is expected.

## Tested functional model

The `lab-group` service-account client has no `manage-users`, `view-users`,
realm-admin, impersonation, client/realm management or role-mapping grant.
The observer checks effective realm-management roles; only **query-users** is
assigned after its necessity is demonstrated.

Fine-grained v2 policy: User policy selects exactly the service-account subject.
One scope permission targets only the registration group's resource ID:

```json
{
  "name": "lab-group-permission",
  "type": "scope",
  "resourceType": "Groups",
  "resources": ["<disposable-registration-group-id>"],
  "scopes": ["view-members", "manage-members", "manage-membership"],
  "policies": ["<service-account-user-policy-id>"],
  "decisionStrategy": "UNANIMOUS"
}
```

No all-user permission, explicit group `manage`, user `map-roles`, user
`manage-group-membership` or explicit all-user `reset-password` permission is added.
Password setting nevertheless works through Keycloak's manage/reset fallback for
group members. This is significant residual authority, not a credential restriction.

The initial four-scope candidate also included group `view`. Removing it still
allowed create/read/reconciliation. The final run repeated the **entire** probe set
with only the three scopes shown above, plus query-users. This is the smallest
functional model demonstrated within this experiment, not a proof over every
possible Keycloak authorization design.

The service submits one fixed group path in the synthetic create payload, along
with enabled=false, emailVerified=false, VERIFY_EMAIL, a non-temporary initial
password and one random operation marker. The observer verifies the resulting
credential type, disabled/unverified state, required action, exact group, marker
and effective default realm `citizen` role. Citizen is inherited through the realm
default-role composite, not assigned by the registration actor.

The existing application adapter still sends **no group** on creation. This is
not a released adapter change. Its existing read/reconcile/enable/send-verification
methods are exercised unchanged against the experimental group-created account.

## Live evidence

The trusted setup/observer principal is separate from the tested service account.
It reads authoritative user/group/credential/role state before and after attacks.
The candidate does not provide its own oracle. Password replacement evidence is
successful status plus changed credential metadata; no password login is performed.

| Probe | Final three-scope model result |
| --- | --- |
| Disabled creation with initial password, operation marker, VERIFY_EMAIL and citizen | PASS, observer-confirmed |
| Creation without any group | PASS denial: 403 and observer found no account |
| Creation into the unrelated control group | PASS denial: 403 and observer found no account |
| Read by subject and marker reconciliation | PASS after query-users; without query-users direct read passed but reconciliation was denied |
| Enable owned synthetic account | PASS, observer confirmed enabled and still unverified |
| Verification-email request | PASS; unchanged adapter sent the request and the separate Mailpit API confirmed a verification message addressed to the synthetic account |
| Direct read/search of unrelated and synthetic realm-admin users | PASS: direct reads 403, exact searches filtered to no visible users |
| Disable unrelated/admin users | PASS: 403 and unchanged observer state |
| Replace unrelated/admin passwords | PASS: 403 and unchanged observer credential metadata |
| Add unrelated/admin users to registration group | PASS: 403 and unchanged membership |
| Remove a group member's registration membership | PASS: 403 and unchanged membership |
| Rename registration group | PASS: 403 and unchanged group |
| Create a child, move the control group under registration, move registration under control | PASS: all 403, observer hierarchy unchanged |
| Rewrite the actor's permission to remove its resource restriction | PASS: 403 and observer permission unchanged |
| Assign realm-management/realm-admin or manage-users to a group member | PASS: 403 and observer role mappings unchanged |
| Add realm-admin role to the group for inherited privileges | PASS: 403 and observer group role mappings unchanged |
| Read users or create an account in the second realm | PASS: 403; observer confirmed no cross-realm account was created |
| Go ReadOwned with the wrong operation marker | PASS: application ownership check rejected the mismatch |
| Direct read of a user created for a different registration operation in the same group | **FAIL_UNSAFE: allowed** |
| Disable that different operation's user | **FAIL_UNSAFE: allowed and observer-confirmed** |
| Replace that different operation's user's password | **FAIL_UNSAFE: allowed and observer-confirmed** |

The email check proves request acceptance and delivery into the new local Mailpit
inbox only. The harness never opens the verification link, dumps the message body,
or completes verification. Link expiry/reuse, inbox delivery outside Mailpit, login
and full lifecycle are not tested in this permission slice.

Three group runs were made: the first stopped when collection reconciliation was
denied without query-users; the second completed the four-scope candidate; the
third completed the reduced three-scope candidate and additional creation/search
denials. No existing environment was reused. All newly created containers are
stopped; their storage/messages remain. Existing GovAI/Mailpit stayed stopped.

### Scope-removal evidence

The harness creates fresh synthetic users for each probe; it never retries an
uncertain creation. These deliberately reduced configurations retain failing
required-operation assertions rather than relabelling the failure as success.

| Removed permission | Observation |
| --- | --- |
| query-users | Direct owned read works; collection reconciliation is 403 |
| group view | Four-scope exploratory candidate still creates, reads and reconciles; confirmed unnecessary by the final full reduced-model run |
| view-members | Creation works, but required read/reconciliation fails |
| manage-members | Creation denied 403; observer confirms no new user |
| manage-membership | Creation denied 403; observer confirms no new user |

## Verdict and next decision

This model narrows the demonstrated blast radius to registration-group members.
It does **not** enforce ownership by issuer/subject plus registration operation.
Keycloak authorizes the service principal through group membership, not through
which operation created the target. A direct API caller holding that credential
can manage credentials across the group's users; the different-operation probe
demonstrates that bypass despite the correctly working Go marker check.

Group-member authority can also be affected by group hierarchy and future
membership changes. The experiment denied actor-driven hierarchy changes; it did
not test every future administrator-created descendant, composite role or feature.
Do not treat those untested combinations as denied. Likewise, two named admin-role
denials are not a proof that every possible privilege-assignment path is denied.

The model is functionally workable only if **administration of all registration
group members** is an explicitly accepted trust boundary. It remains unsuitable
for a create-only or operation-owned credential. Public registration remains gated.

Smallest next step: review that residual-authority decision before changing the
adapter. If all-member credential authority is unacceptable, evaluate a constrained
server-side registration bridge/provider or hosted registration. A group-bearing
adapter change alone cannot supply operation-level administrative isolation.
No routes, production permissions, persistence, login or lifecycle work is authorized
by this result. No commits or pushes were made.

## Implementation and references

Changed only backend experiment code (`cmd/permission-lab`, `internal/permissionlab`)
and backend documentation. `groups_test.go` covers fixed membership payloads,
resource-scoped permissions, separate observer use, strict denials and refusing an
existing state directory. Standard formatting, unit, vet and race checks are
recorded separately from the live unsafe verdict.

Checks actually completed after the final code changes: gofmt,
`go test ./...`, `go vet ./...`, `go test -race ./...` and `git diff --check` passed.
These passing code checks do not change the live `FAIL_UNSAFE` verdict or establish
verification completion, password-policy boundaries, login or real client behavior.

- [Pinned creation/group/credential authorization checks](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/services/resources/admin/UsersResource.java)
- [Pinned query and member permissions/reset fallback](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/services/resources/admin/fgap/UserPermissionsV2.java)
- [Pinned user update, membership and email endpoints](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/services/resources/admin/UserResource.java)
