#!/bin/sh
set -eu
# Fresh dedicated cluster only. No tracing; psql quotes the password as a literal.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
\getenv keycloak_password KEYCLOAK_DB_APP_PASSWORD
CREATE ROLE govai_keycloak LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT PASSWORD :'keycloak_password';
REVOKE ALL ON DATABASE govai_keycloak FROM PUBLIC;
GRANT CONNECT,TEMPORARY ON DATABASE govai_keycloak TO govai_keycloak;
REVOKE ALL ON DATABASE postgres FROM PUBLIC;
REVOKE ALL ON DATABASE template1 FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
CREATE SCHEMA keycloak AUTHORIZATION govai_keycloak;
SQL
