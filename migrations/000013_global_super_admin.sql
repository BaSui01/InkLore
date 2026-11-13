-- ADR-0030: platform-wide Admin authority remains separate from tenant roles.
BEGIN;

ALTER TABLE platform_users
  ADD COLUMN global_roles text[] NOT NULL DEFAULT '{}'::text[];
ALTER TABLE platform_users
  ADD CONSTRAINT platform_users_global_roles_check
  CHECK (global_roles <@ ARRAY['super_admin']::text[]);

COMMIT;
