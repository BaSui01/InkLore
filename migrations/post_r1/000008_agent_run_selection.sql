-- Keep source selection immutable with the original Run across reloads.
-- Old runs remain null-scoped and are never assumed to be whole-chapter.
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS selection_start_offset BIGINT;
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS selection_end_offset BIGINT;
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS selection_source_hash TEXT;
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS selection_scope_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_runs ADD CONSTRAINT agent_runs_selection_binding_check CHECK (
  (selection_start_offset IS NULL AND selection_end_offset IS NULL AND selection_source_hash IS NULL)
  OR (selection_start_offset BETWEEN 0 AND 199999
      AND selection_end_offset > selection_start_offset AND selection_end_offset <= 200000
      AND selection_source_hash = source_hash)
);
