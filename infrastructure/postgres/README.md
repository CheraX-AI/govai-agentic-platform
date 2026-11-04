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

Existing containers retain old bind-mount source paths until explicitly recreated
with the relocated Compose files and original project names. Use a controlled
restart/recreation before relying on the new mounts; do not use `down -v` or delete
volumes. No container restart or live data migration was performed for this move.
Historical files inside ignored `.local/` backups intentionally retain old paths.
