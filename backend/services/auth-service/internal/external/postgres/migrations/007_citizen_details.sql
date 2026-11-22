-- GA-252 extends the existing identity mapping; registration fields stay in
-- citizen_profiles and credentials stay exclusively in Keycloak.
CREATE TABLE auth.citizen_details (
  issuer text NOT NULL,
  subject text NOT NULL,
  -- Encrypted JSON contains name_si, name_ta, date_of_birth, gender and address.
  -- Application validation runs before encryption; SQL constrains the envelope.
  details_ciphertext bytea CHECK (octet_length(details_ciphertext) BETWEEN 29 AND 8192),
  photo_object_key text CHECK (
    char_length(photo_object_key) <= 512
    AND photo_object_key ~ '^[A-Za-z0-9][A-Za-z0-9/_.-]*$'
    AND position('..' in photo_object_key) = 0
  ),
  revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (issuer, subject),
  FOREIGN KEY (issuer, subject) REFERENCES auth.citizen_profiles (issuer, subject)
);

-- Cover citizens registered before this optional extension is installed.
-- Block registrations until both the backfill and insert trigger are installed,
-- so concurrent registration cannot fall between them without a details row.
LOCK TABLE auth.citizen_profiles IN SHARE ROW EXCLUSIVE MODE;
INSERT INTO auth.citizen_details (issuer, subject)
SELECT issuer, subject FROM auth.citizen_profiles;

CREATE FUNCTION auth.initialize_citizen_details() RETURNS trigger LANGUAGE plpgsql
SET search_path = pg_catalog, auth AS $$
BEGIN
  INSERT INTO auth.citizen_details (issuer, subject) VALUES (NEW.issuer, NEW.subject);
  RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION auth.initialize_citizen_details() FROM PUBLIC;
CREATE TRIGGER initialize_citizen_details AFTER INSERT ON auth.citizen_profiles
  FOR EACH ROW EXECUTE FUNCTION auth.initialize_citizen_details();

CREATE FUNCTION auth.check_citizen_details_update() RETURNS trigger LANGUAGE plpgsql
SET search_path = pg_catalog, auth AS $$
BEGIN
  IF ROW(NEW.issuer, NEW.subject, NEW.created_at)
     IS DISTINCT FROM ROW(OLD.issuer, OLD.subject, OLD.created_at)
     OR NEW.revision <> OLD.revision + 1
  THEN RAISE EXCEPTION 'invalid citizen details revision' USING ERRCODE = '23514'; END IF;
  NEW.updated_at := clock_timestamp();
  RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION auth.check_citizen_details_update() FROM PUBLIC;
CREATE TRIGGER citizen_details_update BEFORE UPDATE ON auth.citizen_details
  FOR EACH ROW EXECUTE FUNCTION auth.check_citizen_details_update();

REVOKE ALL ON auth.citizen_details FROM PUBLIC;
GRANT SELECT ON auth.citizen_details TO govai_auth_app;
GRANT INSERT (issuer, subject) ON auth.citizen_details TO govai_auth_app;
GRANT UPDATE (details_ciphertext, photo_object_key, revision)
  ON auth.citizen_details TO govai_auth_app;
