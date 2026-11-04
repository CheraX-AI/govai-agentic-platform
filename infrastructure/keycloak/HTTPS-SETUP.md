<!-- PostgreSQL cutover completed 2026-10-31. Start the dedicated keycloak-postgres project first; see ../postgres/keycloak/README.md. H2-only startup is rollback-only. -->
# Local GovAI HTTPS and registration

Later login update: the [mobile/Postman login API](../../backend/services/auth-service/docs/GA-48-login.md)
now has a separate non-admin client and private `.env.login`. Provision it with
`go run ./tools/dev-auth configure-login` from the auth-service directory, then
restart the API. Registration manual evidence below predates this login slice;
real web/mobile integration and browser sessions remain pending.

Configured and checked locally on 2026-10-31. Existing GovAI users and Keycloak's
named volume were preserved. PostgreSQL remains the separate application database;
Keycloak itself still uses its development dev-file store.

| Surface | Current local URL |
| --- | --- |
| Go readiness | https://localhost:5001/api/ready |
| Registration JSON POST | https://localhost:5001/api/v1/registrations |
| Keycloak admin console | https://localhost:8443/admin/ |
| Citizen account console / dev verification callback | https://localhost:8443/realms/GovAI/account/ |
| Mailpit UI | http://localhost:8025 |

Keycloak's HTTP listener is disabled in HTTPS mode, so the old 8180 URLs no longer
work. The Compose overlay retains the old port mapping but there is no HTTP
listener behind it. Old HTTP-issuer tokens/action links should be replaced; they
are not interchangeable with the new HTTPS issuer. Existing user IDs/passwords
were not migrated or reset.
PostgreSQL bindings for another issuer are not rewritten by this tool; plan an
explicit identity-link migration before changing an issuer with existing real data.

## Start existing setup

Run in PowerShell with Docker Desktop running:

```powershell
cd C:\Users\LENOVO\Desktop\govai-agentic-platform\infrastructure\postgres\auth
docker compose --env-file .env up -d --wait

cd ..\keycloak
docker compose --env-file .env --env-file .env.postgres -f compose.yaml -f compose.https.yaml -f compose.postgres.yaml up -d
docker start govai-mailpit

cd ..\..\..\backend\services\auth-service
go run ./tools/dev-auth check
go run ./cmd/api
```

Allow Keycloak to finish startup before running check. If port 5001 is already
served by an older Go process, stop that process with Ctrl+C first. Do not stop
the unrelated application on port 5000.

From a second terminal in the auth-service directory:

```powershell
go run ./tools/dev-auth smoke
```

This sends a **synthetic** registration through the actual TLS Go API, checks
completed SQL journal/profile ownership, observes the real Keycloak citizen and
checks delivery to existing Mailpit. Its private `.local/registration-smoke.json`
retains synthetic NIC/email/idempotency data, never a password or token. Repeating
it reuses the same key/input; it does not replace credentials or blindly create
another account. Source/email rate limits still apply. The synthetic account and
profile are intentionally retained; no automatic destructive cleanup occurs.

## Local certificate trust

Private files live in ignored `infrastructure/keycloak/tls/`:

- `ca.pem`: public development CA certificate; safe to inspect/import as trust.
- `keycloak-cert.pem`, `api-cert.pem`: public server certificates.
- `keycloak-key.pem`, `api-key.pem`: **private keys; never share or commit**.

Certificates cover localhost, 127.0.0.1 and ::1 and last 90 days. The CA signing
key was discarded after generation; no system/browser trust changes were made.
The Go API and dev-auth command trust this CA explicitly and keep hostname and
certificate verification enabled. Do not disable TLS verification in Postman or
click through certificate warnings as a substitute for trust setup.

For Postman Desktop, add `ca.pem` under Settings > Certificates > CA Certificates
while keeping SSL certificate verification on. To use browsers without warnings,
the user must deliberately trust the development CA in their browser/Windows user
certificate store. Trust **only** `ca.pem`, never either private key. Verify its
SHA-256 fingerprint before importing:

```text
d2cca5c5bde5902dc53368912308bb74721c02cdc8726b470bfb93c6eee4f954
```

This fingerprint describes this workstation's current generated CA; teammates
generate their own and will get a different fingerprint. Never copy these private
keys to teammates. Renew through a reviewed replacement/backup plan; the tool
refuses to overwrite the existing TLS directory and does not delete it for you.

## Provisioning from a fresh local setup

After the base Keycloak `.env` and database setup exist:

```powershell
# From backend/services/auth-service; only when the tls directory does not exist:
go run ./tools/dev-auth certs
# Start Keycloak using the HTTPS Compose overlay, then:
go run ./tools/dev-auth inspect
go run ./tools/dev-auth configure
```

The tool is hardcoded to the local HTTPS address and GovAI realm. It logs in as
the local bootstrap administrator read from the ignored Keycloak `.env`; those
credentials are never copied into Go runtime settings. If that administrator's
password has changed, correct the local operator credentials rather than resetting
existing users. The existing bootstrap-admin warning should be addressed separately
by creating a permanent operator account before a deployed environment is used.

Before mutation it writes a private snapshot of changed realm flags/profile
definitions and previous user count under the auth service `.local/` directory.
It never exports passwords, user representations or complete realm secrets.
Previously existing user IDs are checked for preservation after configuration.

It configures:

- Hosted registration off; verify-email on; fine-grained administration enabled.
- First/last names optional in Keycloak, retaining all existing values. Full legal
  name remains application-owned encrypted PostgreSQL profile data.
- Protected, single-valued, admin-only `govai_registration_operation` attribute.
- Existing citizen/default-role relationships preserved, adding citizen if absent.
- Owned fixed `/ga48-registration` group and `govai-registration-service-dev` client.
- Service actor with **query-users only**, plus a group-bound permission granting
  `view`, `view-members`, `manage-members` and `manage-membership` on that group.
  No manage-users/realm-admin or broad all-user policy is granted.
- Public `govai-registration-verify-dev` client with an exact HTTPS account-console
  callback. This is a **development** verification destination, not an implemented
  frontend/mobile integration. Password login remains separate work.
- Ignored `.env.registration` with stable independent HMAC keys, scoped secret,
  API certificate paths, CA file, HTTPS issuer and port 5001. Existing keys are
  preserved; activation follows final configuration checks.

Clients/groups are tagged as tool-owned. Existing untagged conflicting resources
are refused. Reruns preserve owned clients/secrets and update only the owned group
permission and intended realm/profile configuration; inspect snapshots/diffs first
if another teammate has changed these resources. The tool is not a general-purpose
realm migration framework or a production deployment utility.

## Actual result and correction

The first actual API attempt returned 503 with a safely recorded
`admin_permission_denied` outcome; its journal stayed reserved and no citizen was
created. The prior three-scope disposable experiment did not establish sufficient
group resolution in this existing realm. Adding **view on this fixed group**
resolved it, consistent with the version-specific Keycloak administration guide.
Historical three-scope experiment results remain unchanged; they are not a claim
that view is universally redundant.

The subsequent actual HTTPS API smoke passed: durable completed journal, encrypted
profile, one real enabled/unverified citizen with the protected operation marker,
citizen inheritance and verification email in Mailpit. Reuse keeps the same key
and account. The original three users were preserved during setup. The added
service account and synthetic citizen are expected additions.

Application dependency diagnostics now log only fixed operation/outcome enums and
uncertainty flags. They never log raw errors, URLs, subjects, operation IDs,
passwords, tokens or request bodies. SMTP verification completion by the user,
login, real clients, JWT/RBAC and refresh/logout remain separate checks.

The accepted prototype limitation persists: this secret can administer other
users inside the registration group. It is not create-only/per-operation isolation.
Local TLS does not change this authority or the documented Direct Grant login
architecture limitation.

References: [Keycloak TLS](https://www.keycloak.org/server/enabletls),
[fixed hostname](https://www.keycloak.org/server/hostname),
[Keycloak 26.8.1 group permissions](https://www.keycloak.org/docs/26.8.1/server_admin/).
## User-confirmed manual evidence, 2026-10-31

The user confirmed a Postman registration request, receipt of its verification
email in local Mailpit, successful email verification, and subsequent account
console login. These are user-confirmed manual results, separate from the
independently run automated/API smoke evidence above. Account-console login does
not implement or prove the planned Go password-login API. GA-48 remains In Progress;
login and real web/mobile integration remain pending.
