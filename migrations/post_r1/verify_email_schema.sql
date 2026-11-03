\set ON_ERROR_STOP on
-- Disposable PostgreSQL only. No production data or sender credentials.
INSERT INTO platform_mail_settings(id,provider,enabled) VALUES (1,'disabled',false);

DO $email_contract$
DECLARE
  challenge_id uuid;
  updated_rows int;
BEGIN
  IF (SELECT count(*) FROM schema_migrations) <> 17 THEN
    RAISE EXCEPTION 'frozen R1 migration history changed';
  END IF;
  IF (SELECT count(*) FROM platform_mail_settings) <> 1 THEN
    RAISE EXCEPTION 'mail singleton not persisted';
  END IF;
  IF NOT has_table_privilege('inklore_runtime','platform_mail_settings','SELECT')
      OR NOT has_table_privilege('inklore_runtime','platform_mail_settings','UPDATE')
      OR has_table_privilege('inklore_runtime','platform_mail_settings','DELETE') THEN
    RAISE EXCEPTION 'runtime mail privileges invalid';
  END IF;
  IF has_table_privilege('inklore_runtime','platform_email_challenges','DELETE') THEN
    RAISE EXCEPTION 'runtime must not delete OTP records';
  END IF;

  BEGIN
    UPDATE platform_mail_settings SET provider='smtp',enabled=true WHERE id=1;
    RAISE EXCEPTION 'enabled SMTP without TLS/credentials must fail';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;
  BEGIN
    UPDATE platform_mail_settings SET credential_kind='encrypted',credential_ciphertext='secret-plain',
      credential_key_version=1 WHERE id=1;
    RAISE EXCEPTION 'unencrypted credential incorrectly persisted';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;
  BEGIN
    UPDATE platform_mail_settings SET credential_kind='env' WHERE id=1;
    RAISE EXCEPTION 'env credential without locator must fail';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;
  BEGIN
    UPDATE platform_mail_settings SET credential_kind='encrypted',credential_key_version=1 WHERE id=1;
    RAISE EXCEPTION 'encrypted credential without ciphertext must fail';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;
  BEGIN
    UPDATE platform_mail_settings SET provider='smtp',enabled=true,smtp_host='smtp.example.net',
      smtp_username='noreply',sender_email='noreply@example.net',
      credential_kind='env',credential_locator='MAIL_SMTP_PASSWORD' WHERE id=1;
    RAISE EXCEPTION 'enabled SMTP with NULL port must fail';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;

  INSERT INTO platform_email_challenges(
    purpose,audience,recipient_hash,code_hash,status,sent_at,expires_at
  ) VALUES (
    'login','creator',repeat('a',64),repeat('b',64),'sent',clock_timestamp(),
    clock_timestamp()+interval '5 minutes'
  ) RETURNING id INTO challenge_id;

  UPDATE platform_email_challenges
    SET status='consumed',consumed_at=clock_timestamp()
    WHERE id=challenge_id AND status='sent' AND consumed_at IS NULL
      AND expires_at>clock_timestamp() AND attempts<5;
  GET DIAGNOSTICS updated_rows = ROW_COUNT;
  IF updated_rows<>1 THEN
    RAISE EXCEPTION 'OTP first consumption failed';
  END IF;
  UPDATE platform_email_challenges
    SET status='consumed',consumed_at=clock_timestamp()
    WHERE id=challenge_id AND status='sent' AND consumed_at IS NULL;
  GET DIAGNOSTICS updated_rows = ROW_COUNT;
  IF updated_rows<>0 THEN
    RAISE EXCEPTION 'OTP replay not rejected';
  END IF;
  IF (SELECT count(*) FROM platform_email_challenges WHERE status='consumed')<>1 THEN
    RAISE EXCEPTION 'OTP persisted state invalid';
  END IF;
  RAISE NOTICE 'INK-273 isolated mail schema, no plaintext secret, runtime privileges, single-use OTP: PASS';
END
$email_contract$;
