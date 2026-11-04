# Local PostgreSQL infrastructure

## Infrastructure folder map

- `auth/`: isolated application authentication PostgreSQL setup (project `govai-auth-db`).
- `keycloak/`: isolated Keycloak PostgreSQL setup (project `govai-keycloak-db`).
- `../keycloak/`: Keycloak server, HTTPS and local recovery-mail configuration.
- Root `docker-compose.yml`: existing shared PostgreSQL/Redis/storage/ChromaDB setup,
  unchanged by this folder move. Do not substitute it for either authentication database.

Application SQL migrations stay in
`backend/services/auth-service/internal/postgres/migrations/`; Go queries stay in
that service. Keycloak manages its own database schema. Moving configuration did
not move or merge database contents, change ports, or rename Docker projects,
networks or volumes. Private environment files, TLS material and local backups
moved with their setups and remain ignored. Never regenerate existing credentials.

Local containers were recreated from the relocated paths on 2026-11-04 with the original project/volume names. Mount paths and health checks were verified. Other developers must use their own reviewed local startup; do not use `down -v` or delete volumes. No database-content migration was performed during the folder move.
Historical files inside ignored `.local/` backups intentionally retain old paths.
