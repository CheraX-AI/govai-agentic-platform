-- Stable auth checkpoints contain no password, raw token or arbitrary payload.

CREATE TABLE auth.notification_checkpoints (
  event_id uuid PRIMARY KEY,
  event_type text NOT NULL CHECK (event_type IN (
    'citizen.password.changed', 'citizen.email.verified'
  )),
  source_key bytea NOT NULL CHECK (octet_length(source_key)=32),
  issuer text NOT NULL CHECK (length(issuer) BETWEEN 1 AND 2048),
  subject text NOT NULL CHECK (subject ~ '^[A-Za-z0-9_-]{1,128}$'),
  state text NOT NULL DEFAULT 'prepared' CHECK (
    state IN ('prepared','confirmed','uncertain','cancelled')
  ),
  confirmed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (issuer, event_type, source_key),
  CHECK ((state='confirmed') = (confirmed_at IS NOT NULL))
);

CREATE INDEX notification_checkpoints_attention
  ON auth.notification_checkpoints (issuer, created_at)
  WHERE state IN ('prepared','uncertain');

CREATE FUNCTION auth.check_notification_checkpoint() RETURNS trigger LANGUAGE plpgsql
SET search_path = pg_catalog, auth AS $$
BEGIN
  IF ROW(NEW.event_id, NEW.event_type, NEW.source_key, NEW.issuer, NEW.subject, NEW.created_at)
     IS DISTINCT FROM
     ROW(OLD.event_id, OLD.event_type, OLD.source_key, OLD.issuer, OLD.subject, OLD.created_at)
     OR OLD.state <> 'prepared'
     OR NEW.state NOT IN ('confirmed','uncertain','cancelled')
  THEN
    RAISE EXCEPTION 'invalid notification checkpoint transition' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END $$;

REVOKE ALL ON FUNCTION auth.check_notification_checkpoint() FROM PUBLIC;
CREATE TRIGGER notification_checkpoint_update
  BEFORE UPDATE ON auth.notification_checkpoints
  FOR EACH ROW EXECUTE FUNCTION auth.check_notification_checkpoint();

-- The first observer startup records a durable cutover, excluding older accounts.

CREATE TABLE auth.notification_observer_settings (
  issuer text PRIMARY KEY CHECK (length(issuer) BETWEEN 1 AND 2048),
  observe_since timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE auth.notification_email_watch (
  operation_id text PRIMARY KEY REFERENCES auth.citizen_profiles(operation_id),
  next_check_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  lease_token uuid,
  lease_expires_at timestamptz,
  verified_at timestamptz,
  CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
  CHECK (verified_at IS NULL OR lease_token IS NULL)
);

CREATE INDEX notification_email_watch_due
  ON auth.notification_email_watch (next_check_at, operation_id)
  WHERE verified_at IS NULL;

REVOKE ALL ON auth.notification_checkpoints, auth.notification_observer_settings,
  auth.notification_email_watch FROM PUBLIC, govai_auth_app;
GRANT SELECT ON auth.notification_checkpoints, auth.notification_observer_settings,
  auth.notification_email_watch TO govai_auth_app;
GRANT INSERT (event_id, event_type, source_key, issuer, subject)
  ON auth.notification_checkpoints TO govai_auth_app;
GRANT UPDATE (state, confirmed_at) ON auth.notification_checkpoints TO govai_auth_app;
GRANT INSERT (issuer) ON auth.notification_observer_settings TO govai_auth_app;
GRANT INSERT (operation_id) ON auth.notification_email_watch TO govai_auth_app;
GRANT UPDATE (next_check_at, lease_token, lease_expires_at, verified_at)
  ON auth.notification_email_watch TO govai_auth_app;
