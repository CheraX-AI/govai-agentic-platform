-- Optional recovery-aware web session guard. Enabling it rejects unbound legacy
-- cookies; all web instances must use the same stable identity-HMAC key.
CREATE SEQUENCE auth.recovery_fence_order AS bigint;
REVOKE ALL ON SEQUENCE auth.recovery_fence_order FROM PUBLIC;
GRANT USAGE ON SEQUENCE auth.recovery_fence_order TO govai_auth_app;
CREATE TABLE auth.web_recovery_fences(
 identity_hash bytea PRIMARY KEY CHECK(octet_length(identity_hash)=32),
 revoked_through bigint NOT NULL CHECK(revoked_through>0)
);
CREATE TABLE auth.web_recovery_sessions(
 session_hash bytea PRIMARY KEY REFERENCES auth.web_sessions(session_hash) ON DELETE CASCADE,
 identity_hash bytea NOT NULL CHECK(octet_length(identity_hash)=32),
 login_order bigint NOT NULL CHECK(login_order>0)
);
REVOKE ALL ON auth.web_recovery_fences,auth.web_recovery_sessions FROM PUBLIC;
GRANT SELECT,INSERT ON auth.web_recovery_fences TO govai_auth_app;
GRANT UPDATE(revoked_through) ON auth.web_recovery_fences TO govai_auth_app;
GRANT SELECT,INSERT ON auth.web_recovery_sessions TO govai_auth_app;
-- One-time rollout ends legacy active/refreshing cookies, preserving users/data.
UPDATE auth.web_sessions SET state='revoked',revision=revision+1 WHERE state IN ('active','refreshing');
