-- INK-273: post-R1 extension. Do not modify or re-run frozen R1 000001-000017.
-- Apply once only to an explicitly selected, isolated application schema.
-- The deployment's migration runner must set search_path before execution.

CREATE TABLE platform_mail_settings (
  id smallint PRIMARY KEY DEFAULT 1 CHECK(id = 1),
  provider text NOT NULL DEFAULT 'disabled'
    CHECK(provider IN ('disabled','smtp','resend')),
  enabled boolean NOT NULL DEFAULT false,
  smtp_host varchar(253),
  smtp_port integer CHECK(smtp_port BETWEEN 1 AND 65535),
  smtp_security text CHECK(smtp_security IN ('starttls','implicit_tls')),
  smtp_username varchar(254),
  sender_email varchar(254),
  sender_name varchar(120),
  credential_kind text CHECK(credential_kind IN ('env','encrypted')),
  credential_locator text,
  credential_ciphertext text,
  credential_key_version integer CHECK(credential_key_version > 0),
  revision bigint NOT NULL DEFAULT 1 CHECK(revision > 0),
  updated_by uuid REFERENCES platform_users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT platform_mail_secret_exclusive CHECK (
    (
      (credential_kind IS NULL AND credential_locator IS NULL AND credential_ciphertext IS NULL AND credential_key_version IS NULL)
      OR (credential_kind = 'env' AND credential_locator ~ '^[A-Z_][A-Z0-9_]*$' AND credential_ciphertext IS NULL AND credential_key_version IS NULL)
      OR (credential_kind = 'encrypted' AND credential_locator IS NULL AND credential_ciphertext LIKE 'enc:v1:%' AND credential_key_version IS NOT NULL)
    ) IS TRUE
  ),
  CONSTRAINT platform_mail_enabled_config CHECK (
    (
      NOT enabled OR (
        provider = 'smtp'
        AND smtp_host IS NOT NULL AND length(btrim(smtp_host)) > 0
        AND smtp_port IN (465,587)
        AND smtp_security IS NOT NULL
        AND ((smtp_port = 465 AND smtp_security = 'implicit_tls')
             OR (smtp_port = 587 AND smtp_security = 'starttls'))
        AND smtp_username IS NOT NULL
        AND sender_email IS NOT NULL
        AND credential_kind IS NOT NULL
      ) OR (
        provider = 'resend'
        AND sender_email IS NOT NULL
        AND credential_kind IS NOT NULL
      )
    ) IS TRUE
  )
);
COMMENT ON TABLE platform_mail_settings IS 'Global Admin-only SMTP/HTTPS email configuration. Credentials encrypted or external env references; no plaintext.';
COMMENT ON COLUMN platform_mail_settings.credential_ciphertext IS 'Authenticated ciphertext only, encryption key stored outside PostgreSQL; never returned in API responses.';

CREATE TABLE platform_email_challenges (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  purpose text NOT NULL CHECK(purpose IN ('login','register','reset')),
  audience text NOT NULL CHECK(audience IN ('creator','admin')),
  tenant_id uuid REFERENCES platform_tenants(id),
  actor_id uuid REFERENCES platform_users(id),
  recipient_hash char(64) NOT NULL CHECK(recipient_hash ~ '^[0-9a-f]{64}$'),
  code_hash char(64) NOT NULL CHECK(code_hash ~ '^[0-9a-f]{64}$'),
  status text NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending','sent','failed','consumed','expired')),
  attempts smallint NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
  sent_at timestamptz,
  consumed_at timestamptz,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT platform_email_challenge_state CHECK (
    (status = 'sent' AND sent_at IS NOT NULL AND consumed_at IS NULL)
    OR (status = 'consumed' AND sent_at IS NOT NULL AND consumed_at IS NOT NULL)
    OR (status IN ('pending','failed','expired') AND consumed_at IS NULL)
  ),
  CONSTRAINT platform_email_challenge_expiry CHECK (expires_at > created_at)
);
CREATE INDEX platform_email_challenges_lookup
 ON platform_email_challenges(recipient_hash,purpose,audience,created_at DESC);
CREATE INDEX platform_email_challenges_expire
 ON platform_email_challenges(expires_at) WHERE status IN ('pending','sent');
COMMENT ON TABLE platform_email_challenges IS 'Single-use hashed email OTP state: no plaintext email/code, DB authority; Redis may only enforce cooldown.';

-- The application HTTP identity is deliberately limited to reading/updating
-- these two new tables; deletion is reserved for a future audited retention identity.
REVOKE ALL ON platform_mail_settings, platform_email_challenges FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE ON platform_mail_settings, platform_email_challenges TO inklore_runtime;
REVOKE DELETE ON platform_mail_settings, platform_email_challenges FROM inklore_runtime;
GRANT SELECT,DELETE ON platform_email_challenges TO inklore_retention;
REVOKE DELETE ON platform_mail_settings FROM inklore_retention;
