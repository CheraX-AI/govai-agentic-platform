# Local authentication PostgreSQL

This setup belongs only to the Go authentication service. It creates a new
`govai_auth` database in an isolated Docker Compose project, with a named volume
and loopback port **55432**. It does not connect to, migrate or replace the root
Compose PostgreSQL or Keycloak's current dev-file database.

Run these commands in **PowerShell**, with Docker Desktop running.

## First-time setup

```powershell
cd C:\Users\LENOVO\Desktop\govai-agentic-platform\infrastructure\postgres\auth
.\setup.ps1
docker compose --env-file .env config --quiet
docker compose --env-file .env up -d --wait

cd ..\..\..\backend\services\auth-service
go run ./tools/db migrate
go run ./tools/db check
go test ./...
go run ./cmd/api
```

In a second terminal:

```powershell
Invoke-RestMethod http://localhost:5000/api/health
Invoke-RestMethod http://localhost:5000/api/ready
```

Expected responses are `status: ok` and `status: ready`. Stop an older Go server
with Ctrl+C before starting this one if port 5000 is already used.

`setup.ps1` generates independent random passwords and 32-byte AES/HMAC keys. It
writes this directory's `.env`, the auth service's `.env.database`, and a separate
`.env.database-admin` loaded only by the migration command. It refuses to
overwrite existing files. All are ignored by Git. Do not paste their
contents into chat, screenshots or commits. Existing `.env` files are preserved.
Process environment settings take precedence, followed by `.env.database`, then
the service `.env`. Clear an old DATABASE_URL from your shell if it points elsewhere.

## Privileges

| Account | Purpose | Authority |
| --- | --- | --- |
| `govai_auth_owner` | Bootstrap and explicit migration command | Local administrative account; do not use for server requests |
| `govai_auth_app` | Go runtime | Connect; read auth tables; insert approved columns; update journal checkpoints only |

The app cannot create/drop tables, create roles, read PostgreSQL password hashes,
delete/truncate records, modify migration history or overwrite profile rows.
Citizen accounts are Keycloak identities, **not PostgreSQL login roles**.
For deployment, provision a non-superuser migration owner and a secret-managed
runtime role separately; the Docker bootstrap owner here is local development only.

## Subsequent starts / stops

```powershell
cd C:\Users\LENOVO\Desktop\govai-agentic-platform\infrastructure\postgres\auth
docker compose --env-file .env up -d --wait
```

Stop without deleting data:

```powershell
docker compose --env-file .env stop
```

Do not run `down -v` or regenerate secrets against an existing volume. PostgreSQL
initialization scripts and passwords apply only when the volume is first created.

## Database tests

From the auth-service folder, after starting this database:

```powershell
$env:GOVAI_DB_TEST = "1"
go test ./internal/postgres -count=1 -v
Remove-Item Env:GOVAI_DB_TEST
```

Tests create randomly named `govai_auth_test_*` databases on **127.0.0.1:55432**,
use synthetic data, then drop only those newly created test databases. They do not
reset `govai_auth`. Tests refuse any other host/port. Ordinary `go test ./...`
skips live database checks. The fixture reads the private runtime and admin settings.

## Data and recovery

See [database design](../../../backend/services/auth-service/docs/GA-48-postgresql.md).
Back up the volume/database **and** the private settings files to restricted
storage outside Git. Losing the data key makes encrypted profiles unrecoverable.
Changing the NIC index key without a coordinated migration breaks uniqueness.
No automatic key rotation, retention deletion or schema downgrade is provided.
The API never loads `.env.database-admin`. In deployment, run migrations as a
separate job and deny the runtime OS identity access to migration secrets; local
development files under your own Windows account do not provide that OS boundary.

For a local logical backup without PowerShell corrupting binary output:

```powershell
docker compose --env-file .env exec -T postgres pg_dump -U govai_auth_owner -d govai_auth -Fc -f /tmp/govai-auth.dump
docker compose --env-file .env cp postgres:/tmp/govai-auth.dump ./auth-backup.dump
```

The dump is ignored by Git and still contains sensitive metadata; keep it in
restricted backup storage. Restore testing should target a fresh database and use
the matching keys, never overwrite shared data. No restore has been claimed here.
