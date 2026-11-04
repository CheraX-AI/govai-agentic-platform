# GovAI local Keycloak infrastructure

Current setup: Keycloak 26.8.1, HTTPS `https://localhost:8443`, realm `GovAI`
(case-sensitive), with a dedicated PostgreSQL database. The historical H2-to-
PostgreSQL transition was completed; do not use the base H2-only Compose command
as the normal startup procedure.

## Responsibilities

Keycloak manages identities, password hashes, roles, verification and identity
sessions. Go owns application profiles and application browser-session custody
in its own PostgreSQL database. Go does not modify Keycloak database tables.

The approved prototype uses custom forms through Go: administrative registration
and token-endpoint password login. This is not Authorization Code with PKCE;
HTTPS does not remove the documented password-grant security limitation. Group-
scoped registration credentials retain authority over other users in that group.
Browser identity tokens stay server-side; native clients use their separate flow.

## Existing local setup startup

Run from repository root with Docker Desktop running:

```powershell
docker compose --project-directory infrastructure/postgres/auth -f infrastructure/postgres/auth/compose.yaml up -d --wait
docker compose --project-directory infrastructure/postgres/keycloak -f infrastructure/postgres/keycloak/compose.yaml up -d --wait
docker compose --project-directory infrastructure/keycloak --env-file infrastructure/keycloak/.env --env-file infrastructure/keycloak/.env.postgres -f infrastructure/keycloak/compose.yaml -f infrastructure/keycloak/compose.https.yaml -f infrastructure/keycloak/compose.postgres.yaml up -d
```

Start the existing verification-mail container, if provisioned: `docker start
govai-mailpit`. It is not created by the Keycloak base Compose file. Optional
custom recovery mail uses `compose.recovery-mail.yaml` in this directory.

These commands assume existing private settings, volumes, certificates and realm
configuration. Fresh setup needs deliberate provisioning; do not regenerate
credentials for an existing volume. See [Keycloak PostgreSQL guide](../postgres/keycloak/README.md),
[application database guide](../postgres/auth/README.md), and the
[auth-service guide](../../backend/services/auth-service/README.md).

## HTTPS and addresses

- Admin console: `https://localhost:8443/admin/`
- Issuer: `https://localhost:8443/realms/GovAI`
- Account console: `https://localhost:8443/realms/GovAI/account/`
- Configured Go auth API: `https://localhost:5001`
- Verification-mail inbox: `http://localhost:8025`
- Optional password-recovery inbox: `http://localhost:8026`

Generate certificates locally and trust only the public development CA (`tls/ca.pem`)
through the approved browser/OS procedure. Keep TLS verification enabled; never
share or commit server private keys. Private `.env`, `tls/` and backups are ignored.
Older 8180 HTTP instructions are historical and are not this setup's issuer.

## Data and checks

Compose projects remain `govai-keycloak-dev`, `govai-keycloak-db` and `govai-auth-db`.
The PostgreSQL runtime overlay uses `keycloak_pg_runtime_data`; the old H2 volume
is retained for recovery and is not the active identity store. Preserve volume
names and the private `govai-keycloak-dbnet` network. Never delete volumes to fix
startup or assume H2 data migrates automatically.

Local verification on 2026-11-04 confirmed recreation from these infrastructure
paths with the same named volumes, healthy PostgreSQL containers, trusted-HTTPS
Keycloak discovery and Go readiness 200. User-confirmed Postman native and web
login/refresh/logout checks passed. These checks do not establish real UI integration.

Historical procedures/evidence in HTTPS-SETUP.md must be read in context; its old
H2/foundation snapshots are not the current database/startup specification.
