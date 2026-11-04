#!/bin/sh
set -eu
# psql quotes the environment variable as a SQL literal; never interpolate it
# directly into SQL or enable shell tracing.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
\getenv app_password AUTH_DB_APP_PASSWORD
CREATE ROLE govai_auth_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD :'app_password';
REVOKE ALL ON DATABASE govai_auth FROM PUBLIC;
GRANT CONNECT ON DATABASE govai_auth TO govai_auth_app;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
SQL
