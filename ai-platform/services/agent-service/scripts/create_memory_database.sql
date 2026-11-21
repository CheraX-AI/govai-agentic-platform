-- One-time local setup: the conversation memory database and its two logins, on the
-- shared PostgreSQL server. Run once as a PostgreSQL superuser, passing new random
-- passwords as psql variables (never commit them):
--
--   psql -U postgres -h 127.0.0.1 -p <port> \
--     -v owner_password='<random>' -v app_password='<random>' \
--     -f scripts/create_memory_database.sql
--
-- govai_agent_owner owns the tables and is used only by scripts/setup_memory_tables.py.
-- govai_agent_app is the agent API's runtime login: it reads and writes memory rows only.
-- No other service's database or role is touched.

\set ON_ERROR_STOP on

CREATE ROLE govai_agent_owner LOGIN PASSWORD :'owner_password';
CREATE ROLE govai_agent_app LOGIN PASSWORD :'app_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

CREATE DATABASE govai_agent OWNER govai_agent_owner;
REVOKE ALL ON DATABASE govai_agent FROM PUBLIC;
GRANT CONNECT ON DATABASE govai_agent TO govai_agent_app;

\connect govai_agent
-- Only the owner may create tables; the runtime login may use the schema
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO govai_agent_app;
