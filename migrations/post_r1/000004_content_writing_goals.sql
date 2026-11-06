-- Writing progress counts current confirmed text only. Calendar goals have no time of day.
ALTER TABLE content_novels ADD COLUMN IF NOT EXISTS target_completion_date DATE
  CHECK (target_completion_date BETWEEN DATE '0001-01-01' AND DATE '9999-12-31');

-- Existing save/rollback events remain immutable audit records. New statistics
-- use author-confirmed transitions, keyed by chapter revision rather than version:
-- a confirmed version can be deleted and restored several times.
ALTER TABLE content_writing_events ADD COLUMN IF NOT EXISTS chapter_revision BIGINT;
ALTER TABLE content_writing_events DROP CONSTRAINT IF EXISTS content_writing_events_source_check;
ALTER TABLE content_writing_events ADD CONSTRAINT content_writing_events_source_check
  CHECK (source IN ('save','rollback','confirm','delete','restore'));
ALTER TABLE content_writing_events DROP CONSTRAINT IF EXISTS content_writing_events_tenant_id_version_id_key;
ALTER TABLE content_writing_events DROP CONSTRAINT IF EXISTS content_writing_events_revision_check;
ALTER TABLE content_writing_events ADD CONSTRAINT content_writing_events_revision_check
  CHECK ((source IN ('save','rollback') AND chapter_revision IS NULL)
    OR (source IN ('confirm','delete','restore') AND chapter_revision IS NOT NULL AND chapter_revision BETWEEN 1 AND 9007199254740991));
CREATE UNIQUE INDEX IF NOT EXISTS content_confirmed_writing_revision
  ON content_writing_events(tenant_id,chapter_id,chapter_revision) WHERE chapter_revision IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS content_legacy_writing_version
  ON content_writing_events(tenant_id,version_id) WHERE chapter_revision IS NULL;

-- Recompute, do not increment: both an upgrade and a rerun produce the same
-- totals. Soft-deleted chapters and never-confirmed drafts contribute zero.
-- Historical draft activity cannot reveal the date of past confirmations, so
-- no fabricated daily events are inserted during this backfill.
UPDATE content_novels n SET word_count = COALESCE((
  SELECT SUM(v.word_count) FROM content_chapters c
  JOIN content_chapter_versions v ON v.tenant_id=c.tenant_id AND v.chapter_id=c.id AND v.id=c.confirmed_version_id
  WHERE c.tenant_id=n.tenant_id AND c.novel_id=n.id AND c.deleted_at IS NULL
),0);
