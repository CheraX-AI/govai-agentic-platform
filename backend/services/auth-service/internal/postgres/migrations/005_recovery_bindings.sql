-- Immutable delivery/operation binding for GA-91 service challenges.
-- Legacy hash-only rows have no metadata and cannot be claimed by the service.
CREATE TABLE auth.recovery_bindings (
 token_hash bytea PRIMARY KEY REFERENCES auth.password_recovery(token_hash),
 operation_id text NOT NULL CHECK(operation_id ~ '^[A-Za-z0-9_-]{1,128}$'),
 email_tag bytea NOT NULL CHECK(octet_length(email_tag)=32)
);
REVOKE ALL ON auth.recovery_bindings FROM PUBLIC;
GRANT SELECT,INSERT ON auth.recovery_bindings TO govai_auth_app;