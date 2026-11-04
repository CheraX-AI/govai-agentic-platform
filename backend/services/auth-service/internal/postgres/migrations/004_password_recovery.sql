-- Optional GA-91 extension; applied explicitly, never at API startup.
CREATE TABLE auth.password_recovery (
 token_hash bytea PRIMARY KEY CHECK(octet_length(token_hash)=32),
 issuer text NOT NULL CHECK(length(issuer) BETWEEN 1 AND 2048),
 subject text NOT NULL CHECK(length(subject) BETWEEN 1 AND 128),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','claimed','completed','attention')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '30 minutes')
);
CREATE INDEX password_recovery_expiry ON auth.password_recovery(expires_at);
REVOKE ALL ON auth.password_recovery FROM PUBLIC;
GRANT SELECT ON auth.password_recovery TO govai_auth_app;
GRANT INSERT(token_hash,issuer,subject,expires_at) ON auth.password_recovery TO govai_auth_app;
GRANT UPDATE(state) ON auth.password_recovery TO govai_auth_app;
CREATE FUNCTION auth.check_recovery_transition() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT ((OLD.state='pending' AND NEW.state='claimed' AND OLD.expires_at>clock_timestamp())
      OR (OLD.state='claimed' AND NEW.state IN ('completed','attention'))) THEN
  RAISE EXCEPTION 'invalid recovery transition';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION auth.check_recovery_transition() FROM PUBLIC;
CREATE TRIGGER recovery_transition BEFORE UPDATE ON auth.password_recovery
 FOR EACH ROW EXECUTE FUNCTION auth.check_recovery_transition();