CREATE TABLE auth.web_revocations (
 job_id text PRIMARY KEY CHECK (length(job_id)=43),
 client_hash bytea NOT NULL CHECK (octet_length(client_hash)=32),
 ciphertext bytea NOT NULL CHECK (octet_length(ciphertext)=0 OR octet_length(ciphertext) BETWEEN 29 AND 20000),
 state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','running','done','attention')),
 revision bigint NOT NULL DEFAULT 0 CHECK (revision>=0),
 attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 8),
 next_attempt timestamptz NOT NULL DEFAULT clock_timestamp(),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX web_revocations_due ON auth.web_revocations(client_hash,next_attempt) WHERE state IN ('pending','running');
REVOKE ALL ON auth.web_revocations FROM PUBLIC;
GRANT SELECT ON auth.web_revocations TO govai_auth_app;
GRANT INSERT(job_id,client_hash,ciphertext) ON auth.web_revocations TO govai_auth_app;
GRANT UPDATE(ciphertext,state,revision,attempts,next_attempt,updated_at) ON auth.web_revocations TO govai_auth_app;
