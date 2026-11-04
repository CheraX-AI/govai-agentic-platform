# GA-48 authentication PostgreSQL implementation

The authentication database now has an explicit versioned migration, pgx pool,
encrypted persistence adapters and server readiness integration. This database
stores application profiles and registration recovery, not passwords or Keycloak
internal tables. Login, JWT authorization and real-client integration remain
separate work.

## Schema

| Table | Contents | Enforced ownership |
| --- | --- | --- |
| `auth.registration_operations` | Idempotency key, operation marker, HMAC request fingerprint, unique NIC digest, encrypted pending profile, phase/revision and email retry state | Unique issuer+key; globally unique normalized NIC digest; immutable reservation; single subject binding |
| `auth.citizen_profiles` | Encrypted full name/original NIC/international mobile; Keycloak issuer+subject | Unique operation, NIC digest and issuer+subject; composite foreign key to journal owner |
| `public.govai_auth_migrations` | Version/checksum/applied time | Migration owner writes; runtime reads |

NIC reservation and journal insertion are **one row/one atomic INSERT**. No
check-then-insert or expiring claim is used. Uppercase normalization of old NIC
suffixes is preserved; V/X and different old/new numbers remain distinct. This
does not verify real citizenship or detect different NIC numbers for one person.

Full name, original NIC and mobile are JSON-encrypted with AES-256-GCM, fresh
nonces, and issuer+operation-bound authenticated data. Only explicit allowlisted
fields are serialized; the DTO's redacted JSON methods are never used as storage.
The NIC index uses domain-separated HMAC-SHA256 with an independent stable key.
The request fingerprint comes from the existing service and excludes passwords.
Passwords, email, access/refresh tokens and reset links have no storage columns.

Keys have no automated rotation in this version. Keep them stable and backed up;
plan an explicit coordinated reindex/re-encryption migration before rotation.
Database encryption does not hide profiles from a compromised application with
both runtime database access and the encryption key.

## Transactions and operation ownership

`postgres.Store` implements `RegistrationJournal` and `ProfileStore`.
Each operation obtains a PostgreSQL session advisory lock derived from issuer+key
on a dedicated pool connection. No transaction remains open during Keycloak HTTP
calls. Lock acquisition is cancelable and journal checkpoints commit independently
with a revision compare-and-swap. Database triggers reject reservation mutation,
invalid phase/email transitions, subject rebinding and advancing before a profile
exists. A failed/uncertain Save poisons that session: close/reopen and reload.

Profile persistence uses a short transaction with a journal row lock, verifies
the recorded subject and exact decrypted pending data, and inserts the profile.
Identical replay succeeds; another owner/data never overwrites it. Profile commit
and the following journal checkpoint are intentionally separate and recoverable.

One shared Store per runtime pool limits held operation connections to four;
the pool has twelve connections, leaving room for profile transactions. Do not
create multiple stores sharing one pool to bypass this bound. A 200ms heartbeat
with a 500ms ping deadline cancels the operation on detected connection loss.
Session closure explicitly unlocks, or destroys a broken connection. Cleanup has
a bounded context and does not depend on the canceled request context.

Connection-loss detection has finite latency. PostgreSQL cannot fence an already
in-flight HTTP request at Keycloak during a network partition. The orchestration's
durable `creating`/`email_sending` intents prevent automatic create/send replay
on takeover; they do not guarantee zero overlap of all external operations.
An application/database/Keycloak distributed transaction is not claimed.

## Server and migrations

`go run ./cmd/db migrate` loads a separate private `.env.database-admin` and uses
**MIGRATION_DATABASE_URL** explicitly. The API never loads that file,
never auto-migrates and never uses that URL. Migration version 1 is transactionally
applied under an advisory lock with a SHA-256 checksum. Edit future schema through
new versioned migrations plus runner support; do not edit an applied migration.
There is deliberately no destructive down command.

When DATABASE_URL is set, `cmd/api` opens a bounded pool, verifies the migration,
checks restricted runtime privileges, validates persistence keys and constructs
the real store. The owner connection is rejected for runtime readiness. Startup fails with a
safe error if it cannot do so. `/api/ready` checks database/migration availability
and returns 503 on failure. `/health` and `/api/health` stay liveness checks.
Without DATABASE_URL, the original health-only mode remains available and
`/api/ready` is absent. DSNs and SQL error details are never logged.

`.env.database` is loaded before `.env`, with process variables winning. Remote
database URLs must use `sslmode=verify-full`; plaintext is allowed only on loopback.
Only sslmode/sslrootcert query settings are accepted, preventing host overrides.

The registration HTTP route in cmd/api is still disconnected. In addition to the
real registration service tests with a synthetic identity adapter, a separate
[live integration](GA-48-postgresql-keycloak-integration.md) now connects real
PostgreSQL and HTTPS Keycloak/SMTP through a private test HTTP handler. Public
runtime configuration/route activation remains pending; it is not implied by a
green database readiness response.

## Verification

Unit tests cover encryption round trips, operation binding, tamper rejection,
NIC normalization/indexing, independent keys and configuration errors.
Opt-in real PostgreSQL tests cover migration replay, app privilege denials,
same-key replay/conflict, canceled lock acquisition, competing NIC claims from
independent pools, no orphan after collision, profile ownership/idempotency,
profile-before-enable enforcement, reopening committed checkpoints, connection
termination/cancellation and the real orchestration with synthetic identity calls.

Pool reopen and backend termination tests are not a full process-kill campaign
at every checkpoint, full-disk fault injection or a backup/restore drill. The
separately documented real PostgreSQL + Keycloak/SMTP test now passes; distributed
rate limiting and full verification/login lifecycle remain pending.
Existing accepted group-wide Keycloak administrative authority and
historical permission experiment results are unchanged.

Verified locally on 2026-10-31: explicit migration and replay, runtime database
check, full unit suite, vet and race suite (including opt-in real PostgreSQL tests).
The actual Go server was started on test port 5001: liveness/readiness returned 200;
stopping only this database kept liveness at 200 and changed readiness to 503;
restarting it restored readiness to 200 with migration metadata preserved.
This database was left running; the temporary test API was shut down. No Keycloak
settings, shared services, frontend/mobile/Python files, commits or pushes changed.

Run instructions: [local database guide](../../../dev/auth-postgres/README.md).
References: [PostgreSQL advisory locks](https://www.postgresql.org/docs/16/explicit-locking.html#ADVISORY-LOCKS),
[PostgreSQL privileges](https://www.postgresql.org/docs/16/ddl-priv.html),
[pgx v5 pool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool).
