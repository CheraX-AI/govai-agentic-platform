-- Conversation IDs and the citizen (Keycloak subject) who owns each one.
-- Holds no messages: chat history and AI memory are not stored here.
CREATE SCHEMA core;
REVOKE ALL ON SCHEMA core FROM PUBLIC;

CREATE TABLE core.conversations (
    id uuid PRIMARY KEY,
    citizen_subject text NOT NULL CHECK (length(citizen_subject) BETWEEN 1 AND 255),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON core.conversations FROM PUBLIC;

-- The runtime role can only create conversations and look them up.
GRANT USAGE ON SCHEMA core TO govai_core_app;
GRANT SELECT, INSERT ON core.conversations TO govai_core_app;
