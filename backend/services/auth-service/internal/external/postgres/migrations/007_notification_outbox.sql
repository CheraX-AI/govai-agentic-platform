-- Account events are auth-owned; this table is not the notification inbox.

CREATE TABLE auth.notification_outbox (
  event_id uuid PRIMARY KEY,
  action_id uuid NOT NULL,
  event_type text NOT NULL CHECK (event_type IN (
    'citizen.login.succeeded',
    'citizen.password.changed',
    'citizen.email.verified'
  )),
  event_version integer NOT NULL CHECK (event_version = 1),
  issuer text NOT NULL CHECK (length(issuer) BETWEEN 1 AND 2048),
  subject text NOT NULL CHECK (subject ~ '^[A-Za-z0-9_-]{1,128}$'),
  occurred_at timestamptz NOT NULL CHECK (
    occurred_at >= '1970-01-01T00:00:00Z'::timestamptz AND isfinite(occurred_at)
  ),
  priority integer GENERATED ALWAYS AS (
    CASE WHEN event_type = 'citizen.email.verified' THEN 1 ELSE 10 END
  ) STORED,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  last_error_code text NOT NULL DEFAULT '' CHECK (last_error_code IN (
    '', 'broker_unavailable', 'publish_unknown', 'unroutable'
  )),
  lease_token uuid,
  lease_expires_at timestamptz,
  published_at timestamptz,
  UNIQUE (issuer, event_type, action_id),
  CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
  CHECK (published_at IS NULL OR lease_token IS NULL)
);

CREATE INDEX notification_outbox_pending
  ON auth.notification_outbox (priority DESC, next_attempt_at, created_at, event_id)
  WHERE published_at IS NULL;

REVOKE ALL ON auth.notification_outbox FROM PUBLIC, govai_auth_app;
GRANT SELECT ON auth.notification_outbox TO govai_auth_app;
GRANT INSERT (
  event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox TO govai_auth_app;
GRANT UPDATE (
  next_attempt_at, attempt_count, last_error_code,
  lease_token, lease_expires_at, published_at
) ON auth.notification_outbox TO govai_auth_app;
