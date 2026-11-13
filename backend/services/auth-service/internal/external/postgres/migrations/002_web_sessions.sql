CREATE TABLE auth.web_sessions (
 session_hash bytea PRIMARY KEY CHECK (octet_length(session_hash)=32),
 ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) BETWEEN 29 AND 65536),
 revision bigint NOT NULL DEFAULT 0 CHECK (revision>=0),
 state text NOT NULL DEFAULT 'active' CHECK (state IN ('active','refreshing','revoked')),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX web_sessions_expiry ON auth.web_sessions(expires_at);
REVOKE ALL ON auth.web_sessions FROM PUBLIC;
GRANT SELECT ON auth.web_sessions TO govai_auth_app;
GRANT INSERT (session_hash,ciphertext,expires_at) ON auth.web_sessions TO govai_auth_app;
GRANT UPDATE (ciphertext,revision,state) ON auth.web_sessions TO govai_auth_app;
