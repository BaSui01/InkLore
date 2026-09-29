-- ADR-0029: bind the reserved Admin username to one existing platform user.
BEGIN;

ALTER TABLE platform_users
  ADD COLUMN username varchar(32),
  ADD CONSTRAINT platform_users_username_check CHECK (username IS NULL OR username='root');
CREATE UNIQUE INDEX platform_users_username_unique
  ON platform_users(lower(username)) WHERE username IS NOT NULL;

COMMIT;
