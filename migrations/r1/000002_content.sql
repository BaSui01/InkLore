-- INK-141 / R1 frozen target; PostgreSQL 16. Apply after platform.sql.
-- UUIDs supplied by application must be UUIDv4; no cross-domain business FK.
BEGIN;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE TABLE content_novels (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_id UUID NOT NULL REFERENCES platform_users(id),
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary)<=2000),
  category TEXT NOT NULL DEFAULT '' CHECK (char_length(category)<=60),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived','deleted')),
  previous_status TEXT CHECK (previous_status IN ('active','archived')),
  target_words INTEGER NOT NULL DEFAULT 0 CHECK (target_words BETWEEN 0 AND 100000000),
  word_count BIGINT NOT NULL DEFAULT 0 CHECK (word_count BETWEEN 0 AND 400000000),
  deleted_at TIMESTAMPTZ,
  CHECK ((status='deleted')=(deleted_at IS NOT NULL)),
  CHECK ((status='deleted')=(previous_status IS NOT NULL)),
  UNIQUE(tenant_id,id)
);
CREATE INDEX content_novels_shelf ON content_novels(tenant_id,owner_id,status,updated_at DESC,id DESC);
CREATE INDEX content_novels_expiry ON content_novels(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE TABLE content_volumes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL,
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary)<=2000),
  position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 99),
  deleted_at TIMESTAMPTZ,
  active_position INTEGER GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN position END) STORED,
  FOREIGN KEY(tenant_id,novel_id) REFERENCES content_novels(tenant_id,id),
  UNIQUE(tenant_id,novel_id,id),
  UNIQUE(tenant_id,novel_id,active_position) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX content_volumes_list ON content_volumes(tenant_id,novel_id,updated_at DESC,id DESC);
CREATE TABLE content_chapters (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, volume_id UUID NOT NULL,
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 1999),
  status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','confirmed')),
  body TEXT NOT NULL DEFAULT '' CHECK(char_length(body)<=200000 AND position(E'\r' in body)=0),
  current_version_id UUID NOT NULL, confirmed_version_id UUID,
  content_hash CHAR(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  word_count INTEGER NOT NULL DEFAULT 0 CHECK(word_count BETWEEN 0 AND 200000),
  deleted_at TIMESTAMPTZ,
  active_position INTEGER GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN position END) STORED,
  FOREIGN KEY(tenant_id,novel_id,volume_id) REFERENCES content_volumes(tenant_id,novel_id,id),
  UNIQUE(tenant_id,novel_id,id), UNIQUE(tenant_id,id),
  UNIQUE(tenant_id,volume_id,active_position) DEFERRABLE INITIALLY DEFERRED,
  CHECK(status<>'confirmed' OR current_version_id=confirmed_version_id)
);
CREATE INDEX content_chapters_list ON content_chapters(tenant_id,novel_id,updated_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX content_chapters_body_search ON content_chapters USING gin(body gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX content_chapters_title_search ON content_chapters USING gin(title gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE TABLE content_chapter_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  novel_id UUID NOT NULL, chapter_id UUID NOT NULL,
  chapter_revision BIGINT NOT NULL CHECK(chapter_revision>0),
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  body TEXT NOT NULL CHECK(char_length(body)<=200000 AND position(E'\r' in body)=0),
  content_hash CHAR(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  word_count INTEGER NOT NULL CHECK(word_count BETWEEN 0 AND 200000),
  source TEXT NOT NULL CHECK(source IN ('create','save','rollback','import','copy')),
  actor_id UUID REFERENCES platform_users(id), restored_from_version_id UUID,
  FOREIGN KEY(tenant_id,novel_id,chapter_id) REFERENCES content_chapters(tenant_id,novel_id,id),
  UNIQUE(tenant_id,chapter_id,id), UNIQUE(tenant_id,chapter_id,chapter_revision),
  FOREIGN KEY(tenant_id,chapter_id,restored_from_version_id) REFERENCES content_chapter_versions(tenant_id,chapter_id,id) DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE content_chapters ADD CONSTRAINT content_current_version_fk FOREIGN KEY(tenant_id,id,current_version_id) REFERENCES content_chapter_versions(tenant_id,chapter_id,id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE content_chapters ADD CONSTRAINT content_confirmed_version_fk FOREIGN KEY(tenant_id,id,confirmed_version_id) REFERENCES content_chapter_versions(tenant_id,chapter_id,id) DEFERRABLE INITIALLY DEFERRED;
CREATE INDEX content_versions_history ON content_chapter_versions(tenant_id,chapter_id,created_at DESC,id DESC);
CREATE TABLE content_writing_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  novel_id UUID NOT NULL, chapter_id UUID NOT NULL, version_id UUID NOT NULL,
  actor_id UUID REFERENCES platform_users(id),
  added_words INTEGER NOT NULL CHECK(added_words BETWEEN 0 AND 200000),
  removed_words INTEGER NOT NULL CHECK(removed_words BETWEEN 0 AND 200000),
  source TEXT NOT NULL CHECK(source IN ('save','rollback')),
  UNIQUE(tenant_id,version_id),
  FOREIGN KEY(tenant_id,novel_id,chapter_id) REFERENCES content_chapters(tenant_id,novel_id,id),
  FOREIGN KEY(tenant_id,chapter_id,version_id) REFERENCES content_chapter_versions(tenant_id,chapter_id,id)
);
CREATE INDEX content_writing_days ON content_writing_events(tenant_id,novel_id,created_at);
CREATE TABLE content_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, actor_id UUID NOT NULL REFERENCES platform_users(id),
  kind TEXT NOT NULL CHECK(kind IN ('import','export','copy')),
  status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','preview_ready','committing','cancelling','succeeded','failed','cancelled','expired')),
  stage TEXT NOT NULL DEFAULT 'queued' CHECK(stage IN ('queued','snapshot','parse','preview','commit','render','complete')),
  progress INTEGER NOT NULL DEFAULT 0 CHECK(progress BETWEEN 0 AND 100),
  source_file_id UUID, output_file_id UUID, target_novel_id UUID,
  format TEXT NOT NULL CHECK(format IN ('txt','markdown','docx','epub','copy')),
  source_revision BIGINT NOT NULL CHECK(source_revision>0),
  lease_until TIMESTAMPTZ, lease_token UUID,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(), expires_at TIMESTAMPTZ NOT NULL,
  error_code TEXT CHECK(char_length(error_code)<=120),
  result_chapter_count INTEGER NOT NULL DEFAULT 0 CHECK(result_chapter_count BETWEEN 0 AND 2000),
  copy_title TEXT CHECK(char_length(copy_title) BETWEEN 1 AND 120),
  include_planning BOOLEAN NOT NULL DEFAULT false, include_worldview BOOLEAN NOT NULL DEFAULT false,
  include_drafts BOOLEAN NOT NULL DEFAULT true, volume_ids UUID[] NOT NULL DEFAULT '{}',
  commit_volume_id UUID, commit_item_ids UUID[] NOT NULL DEFAULT '{}',
  review_ticket UUID, expected_novel_revision BIGINT,
  planning_snapshot_file_id UUID, worldview_snapshot_file_id UUID, organization_snapshot_file_id UUID,
  FOREIGN KEY(tenant_id,novel_id) REFERENCES content_novels(tenant_id,id),
  UNIQUE(tenant_id,id),
  CHECK(cardinality(volume_ids)<=100 AND cardinality(commit_item_ids)<=2000)
);
CREATE INDEX content_jobs_claim ON content_jobs(available_at,id) WHERE status IN ('queued','committing','running','cancelling');
CREATE INDEX content_jobs_list ON content_jobs(tenant_id,novel_id,actor_id,updated_at DESC,id DESC);
CREATE TABLE content_import_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  job_id UUID NOT NULL, ordinal INTEGER NOT NULL CHECK(ordinal BETWEEN 0 AND 1999),
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  body TEXT NOT NULL CHECK(char_length(body)<=200000 AND position(E'\r' in body)=0),
  word_count INTEGER NOT NULL CHECK(word_count BETWEEN 0 AND 200000),
  content_hash CHAR(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  selected BOOLEAN NOT NULL DEFAULT false, warnings TEXT[] NOT NULL DEFAULT '{}',
  CHECK(warnings <@ ARRAY['empty_chapter','heading_inferred']::TEXT[]),
  FOREIGN KEY(tenant_id,job_id) REFERENCES content_jobs(tenant_id,id),
  UNIQUE(tenant_id,job_id,ordinal), UNIQUE(tenant_id,job_id,id)
);
CREATE INDEX content_import_preview ON content_import_items(tenant_id,job_id,updated_at DESC,id DESC);
CREATE TABLE content_job_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  job_id UUID NOT NULL, chapter_id UUID NOT NULL, version_id UUID NOT NULL,
  volume_id UUID NOT NULL, volume_title TEXT NOT NULL CHECK(char_length(volume_title) BETWEEN 1 AND 120),
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  volume_position INTEGER NOT NULL CHECK(volume_position BETWEEN 0 AND 99),
  chapter_position INTEGER NOT NULL CHECK(chapter_position BETWEEN 0 AND 1999),
  chapter_revision BIGINT NOT NULL CHECK(chapter_revision>0),
  body TEXT NOT NULL CHECK(char_length(body)<=200000),
  content_hash CHAR(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  FOREIGN KEY(tenant_id,job_id) REFERENCES content_jobs(tenant_id,id),
  UNIQUE(tenant_id,job_id,chapter_id)
);
CREATE INDEX content_jobs_expiry ON content_jobs(expires_at) WHERE status='preview_ready';
CREATE FUNCTION content_reject_history_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'content.immutable_history'; END; $$;
CREATE TRIGGER content_version_immutable BEFORE UPDATE ON content_chapter_versions FOR EACH ROW EXECUTE FUNCTION content_reject_history_update();
CREATE TRIGGER content_writing_immutable BEFORE UPDATE ON content_writing_events FOR EACH ROW EXECUTE FUNCTION content_reject_history_update();
CREATE TRIGGER content_snapshot_immutable BEFORE UPDATE ON content_job_snapshots FOR EACH ROW EXECUTE FUNCTION content_reject_history_update();
-- API actor has SELECT/INSERT only on versions/writing_events/snapshots.
-- Dedicated retention capability erases expired content, with deferred FKs in one transaction.
COMMIT;
