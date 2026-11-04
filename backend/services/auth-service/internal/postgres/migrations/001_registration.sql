CREATE SCHEMA auth;
REVOKE ALL ON SCHEMA auth FROM PUBLIC;

CREATE TABLE auth.registration_operations (
  operation_id text PRIMARY KEY CHECK (operation_id ~ '^[A-Za-z0-9_-]{1,128}$'),
  issuer text NOT NULL CHECK (length(issuer) BETWEEN 1 AND 2048),
  idempotency_key uuid NOT NULL,
  fingerprint bytea NOT NULL CHECK (octet_length(fingerprint) = 32),
  nic_digest bytea NOT NULL UNIQUE CHECK (octet_length(nic_digest) = 32),
  pending_ciphertext bytea NOT NULL CHECK (octet_length(pending_ciphertext) BETWEEN 29 AND 8192),
  revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
  phase text NOT NULL DEFAULT 'reserved' CHECK (phase IN
    ('reserved','creating','subject_recorded','profile_ready','enabling','enabled',
     'email_sending','completed','duplicate','rejected','attention')),
  subject text CHECK (subject ~ '^[A-Za-z0-9_-]{1,128}$'),
  email_attempts integer NOT NULL DEFAULT 0 CHECK (email_attempts BETWEEN 0 AND 10),
  next_email_attempt timestamptz,
  email_state text NOT NULL DEFAULT '' CHECK (email_state IN ('','request_accepted','already_verified')),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (issuer, idempotency_key),
  UNIQUE (issuer, subject),
  UNIQUE (operation_id, issuer, subject, nic_digest),
  CHECK ((phase IN ('subject_recorded','profile_ready','enabling','enabled','email_sending','completed','attention')) = (subject IS NOT NULL)),
  CHECK ((phase = 'completed') = (email_state <> ''))
);

CREATE TABLE auth.citizen_profiles (
  operation_id text PRIMARY KEY,
  issuer text NOT NULL,
  subject text NOT NULL,
  nic_digest bytea NOT NULL UNIQUE,
  profile_ciphertext bytea NOT NULL CHECK (octet_length(profile_ciphertext) BETWEEN 29 AND 8192),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (issuer, subject),
  FOREIGN KEY (operation_id, issuer, subject, nic_digest)
    REFERENCES auth.registration_operations (operation_id, issuer, subject, nic_digest)
);

CREATE INDEX registration_recovery ON auth.registration_operations (phase, updated_at)
  WHERE phase NOT IN ('completed','duplicate','rejected');

CREATE FUNCTION auth.check_operation_update() RETURNS trigger LANGUAGE plpgsql
SET search_path = pg_catalog, auth AS $$
BEGIN
  IF ROW(NEW.operation_id,NEW.issuer,NEW.idempotency_key,NEW.fingerprint,NEW.nic_digest,
         NEW.pending_ciphertext,NEW.created_at)
     IS DISTINCT FROM ROW(OLD.operation_id,OLD.issuer,OLD.idempotency_key,OLD.fingerprint,
         OLD.nic_digest,OLD.pending_ciphertext,OLD.created_at)
     OR NEW.revision <> OLD.revision + 1
     OR (OLD.subject IS NOT NULL AND NEW.subject IS DISTINCT FROM OLD.subject)
     OR (OLD.subject IS NULL AND NEW.subject IS NOT NULL AND NOT (OLD.phase='creating' AND NEW.phase='subject_recorded'))
  THEN RAISE EXCEPTION 'invalid registration checkpoint' USING ERRCODE='23514'; END IF;
  IF NOT (
    (OLD.phase='reserved' AND NEW.phase='creating') OR
    (OLD.phase='creating' AND NEW.phase IN ('subject_recorded','duplicate','rejected','reserved')) OR
    (OLD.phase='subject_recorded' AND NEW.phase='profile_ready') OR
    (OLD.phase='profile_ready' AND NEW.phase='enabling') OR
    (OLD.phase='enabling' AND NEW.phase='enabled') OR
    (OLD.phase='enabled' AND NEW.phase IN ('email_sending','completed','attention')) OR
    (OLD.phase='email_sending' AND NEW.phase IN ('completed','enabled'))
  ) THEN RAISE EXCEPTION 'invalid registration transition' USING ERRCODE='23514'; END IF;
  IF NEW.email_attempts <> OLD.email_attempts + (CASE WHEN OLD.phase='enabled' AND NEW.phase='email_sending' THEN 1 ELSE 0 END)
    OR (NEW.next_email_attempt IS DISTINCT FROM OLD.next_email_attempt AND NOT (OLD.phase='email_sending' AND NEW.phase='enabled'))
    OR (NEW.email_state IS DISTINCT FROM OLD.email_state AND NEW.phase <> 'completed')
  THEN RAISE EXCEPTION 'invalid email checkpoint' USING ERRCODE='23514'; END IF;
  IF NEW.phase='profile_ready' AND NOT EXISTS
    (SELECT 1 FROM auth.citizen_profiles p WHERE p.operation_id=NEW.operation_id)
  THEN RAISE EXCEPTION 'profile required before enabling' USING ERRCODE='23514'; END IF;
  NEW.updated_at := clock_timestamp();
  RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION auth.check_operation_update() FROM PUBLIC;
CREATE TRIGGER operation_update BEFORE UPDATE ON auth.registration_operations
  FOR EACH ROW EXECUTE FUNCTION auth.check_operation_update();

REVOKE ALL ON ALL TABLES IN SCHEMA auth FROM PUBLIC;
GRANT USAGE ON SCHEMA auth TO govai_auth_app;
GRANT SELECT ON auth.registration_operations, auth.citizen_profiles TO govai_auth_app;
GRANT INSERT (operation_id,issuer,idempotency_key,fingerprint,nic_digest,pending_ciphertext)
  ON auth.registration_operations TO govai_auth_app;
GRANT INSERT (operation_id,issuer,subject,nic_digest,profile_ciphertext)
  ON auth.citizen_profiles TO govai_auth_app;
GRANT UPDATE (revision,phase,subject,email_attempts,next_email_attempt,email_state)
  ON auth.registration_operations TO govai_auth_app;
