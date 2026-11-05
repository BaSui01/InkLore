-- #98: finish the existing persistent email foundation; never rewrite 000001.
INSERT INTO platform_mail_settings(id) VALUES(1) ON CONFLICT(id) DO NOTHING;
ALTER TABLE platform_users ADD COLUMN email_verified_at timestamptz;
ALTER TABLE platform_email_challenges
 ADD COLUMN requested_tenant_id uuid,
 ADD COLUMN request_key uuid,
 ADD COLUMN request_hash char(64) CHECK(request_hash ~ '^[0-9a-f]{64}$'),
 ADD COLUMN ip_hash char(64) CHECK(ip_hash ~ '^[0-9a-f]{64}$');
CREATE UNIQUE INDEX platform_email_challenge_request
 ON platform_email_challenges(request_key) WHERE request_key IS NOT NULL;
CREATE INDEX platform_email_challenge_ip_rate
 ON platform_email_challenges(ip_hash,created_at DESC);
ALTER TABLE platform_email_challenges ADD CONSTRAINT platform_email_challenge_runtime_binding
 CHECK ((request_key IS NULL AND request_hash IS NULL AND ip_hash IS NULL)
 OR (request_key IS NOT NULL AND request_hash IS NOT NULL AND ip_hash IS NOT NULL));
CREATE TABLE platform_mail_test_deliveries (
 id uuid PRIMARY KEY,
 actor_id uuid NOT NULL REFERENCES platform_users(id),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 request_key uuid NOT NULL,
 settings_revision bigint NOT NULL CHECK(settings_revision>0),
 status text NOT NULL CHECK(status IN ('pending','sent','failed')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 UNIQUE(actor_id,request_key),
 CHECK ((status='pending' AND completed_at IS NULL) OR (status<>'pending' AND completed_at IS NOT NULL))
);
CREATE INDEX platform_mail_test_rate ON platform_mail_test_deliveries(actor_id,created_at DESC);
REVOKE ALL ON platform_mail_test_deliveries FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE ON platform_mail_test_deliveries TO inklore_runtime;
GRANT SELECT,DELETE ON platform_mail_test_deliveries TO inklore_retention;
