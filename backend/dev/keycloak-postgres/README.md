# Dedicated Keycloak PostgreSQL — local prototype

Status: approved local H2-to-PostgreSQL cutover completed on 2026-10-31.


## Ownership and isolation

The authentication developer owns this separate Compose project. PostgreSQL 16.14
runs in `govai-keycloak-db`, with its own volume and internal-only network
`govai-keycloak-dbnet`. There is no host SQL port. This is separate from the Go
application database `govai-auth-db` on localhost 55432.

Database/schema/runtime role: `govai_keycloak` / `keycloak` / `govai_keycloak`.
Runtime owns only its dedicated schema, with CONNECT/TEMPORARY but without
superuser, create-role, create-database, replication or bypass-RLS. Bootstrap
credentials are separate and are never supplied to Go or Keycloak.

`setup.ps1` requires PowerShell 7+ and is for a fresh installation only: it generates ignored `.env` and
`../keycloak/.env.postgres` and refuses existing credentials. Keep credentials with
their volumes. Do not regenerate passwords for existing data or rerun SQL init by
removing a volume. Keycloak manages its own SQL tables; Go migrations must not.
Remote/shared SQL deployments additionally require verified TLS; this local SQL
connection stays inside the private Docker network.

## Normal startup

From `backend/dev/keycloak-postgres`:

```powershell
docker compose --env-file .env -f compose.yaml up -d --wait
cd ../keycloak
docker compose --env-file .env --env-file .env.postgres -f compose.yaml -f compose.https.yaml -f compose.postgres.yaml config --quiet
docker compose --env-file .env --env-file .env.postgres -f compose.yaml -f compose.https.yaml -f compose.postgres.yaml up -d
```

Then run `go run ./cmd/api` from `backend/services/auth-service`.
Keycloak issuer remains `https://localhost:8443/realms/GovAI`; API remains
`https://localhost:5001`. Never omit the PostgreSQL overlay during normal startup.
It retains the original SMTP network and mounts a new runtime data volume, leaving
the original H2 volume untouched. JDBC uses the internal `keycloak-postgres:5432`
host with explicit database/schema. Never use `down -v` or delete existing volumes.

## Local verification — 2026-10-31

Offline H2 backup, isolated PostgreSQL rehearsal, final import/export comparison,
container recreation and a separate SQL backup/restore smoke test passed. Original
realm/user IDs, credential material, client secrets, roles/groups, policies, SMTP
and issuer were preserved. Application-table fingerprints were unchanged before
API restart. Backend readiness, invalid-login/protected-route checks and web-session
checks passed. Rehearsal verified synthetic registration, SMTP delivery and both
login clients' token/refresh/revocation flows; it did not repeat human email-link
completion. The user confirmed existing-account login after cutover returned 200 with an access token. Real web/mobile integration remains pending.

The comparison explicitly accounts for Keycloak's generated internal
admin-permissions service account/UMA role, absent userinfo flags becoming false,
new secret timestamps and reordered policy scope arrays. Internal full-scope and
client-scope defaults were restored. Added logout defaults were cleared except on
three protected internal clients with no redirect URIs. Existing explicit callbacks
and permission values remain checked. Accepted group-wide registration authority
is unchanged. Private H2/rehearsal/restore artifacts are retained outside Git.
## Backup, transition and rollback

The approved transition stopped API/Keycloak, backed up H2 offline, exported from
a separate H2 copy, rehearsed import in a separate SQL database, compared users,
credential material and configuration, then imported the empty final target.
Private artifacts are under ignored `.local/`; never commit or share raw exports,
logs, environment files, password hashes, keys or database dumps.

The original H2 volume and private original configuration are retained. Before
new writes, rollback can stop PostgreSQL-backed Keycloak and restore the original
H2-only configuration and volume. After registrations or identity changes, these
stores diverge: stop writes and reconcile deliberately instead of switching back.
Do not run both as competing identity stores. No destructive cleanup is included.

Full SQL backup uses `pg_dump -Fc`; restoration was tested in a separate database.
Protect both database backup and credentials, and repeat backup/restore procedures
before future changes. Realm export is not a full disaster-recovery backup and
excludes sessions, events and other documented runtime state. Users should log in
again; existing signed JWTs can remain valid until their expiry. Restarting the API
does not make old tokens instantly invalid.

References: [Keycloak database configuration](https://www.keycloak.org/server/db),
[offline import/export and limitations](https://www.keycloak.org/server/importExport).
Pinned Keycloak remains 26.8.1; this transition does not upgrade it.