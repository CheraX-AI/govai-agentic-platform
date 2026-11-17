-- One-time local setup: the core-service database and its two logins, on the
-- shared PostgreSQL server. Run once as a PostgreSQL superuser, passing new
-- random passwords as psql variables (never commit them):
--
--   psql -U postgres -h 127.0.0.1 -p <port> \
--     -v owner_password='<random>' -v app_password='<random>' \
--     -f internal/database/setup/create_database.sql
--
-- govai_core_owner owns the schema and is used only by `go run ./cmd/migrate`.
-- govai_core_app is the API's runtime login: connect, read and insert only.
-- No other service's database or role is touched.

\set ON_ERROR_STOP on

CREATE ROLE govai_core_owner LOGIN PASSWORD :'owner_password';
CREATE ROLE govai_core_app LOGIN PASSWORD :'app_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

CREATE DATABASE govai_core OWNER govai_core_owner;
REVOKE ALL ON DATABASE govai_core FROM PUBLIC;
GRANT CONNECT ON DATABASE govai_core TO govai_core_app;

\connect govai_core
-- Only the owner may create objects; the app reads the migration table in public.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO govai_core_app;
