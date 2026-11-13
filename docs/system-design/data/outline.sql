-- 既有基线 DDL；PostgreSQL 16 方言。在 platform.sql 之后执行。
-- UUIDs supplied by application must be UUIDv4; no cross-domain business FK.
BEGIN;
CREATE TABLE outline_nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, parent_id UUID,
  level TEXT NOT NULL CHECK(level IN ('master','volume','chapter')),
  target_id UUID NOT NULL,
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  summary TEXT NOT NULL DEFAULT '' CHECK(char_length(summary)<=20000),
  content_hash CHAR(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 1999),
  status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','ready','archived')),
  deleted_at TIMESTAMPTZ,
  active_position INTEGER GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN position END) STORED,
  UNIQUE(tenant_id,novel_id,id),
  FOREIGN KEY(tenant_id,novel_id,parent_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  UNIQUE(tenant_id,novel_id,parent_id,active_position) DEFERRABLE INITIALLY DEFERRED,
  CHECK((level='master' AND parent_id IS NULL AND target_id=novel_id AND position=0) OR (level<>'master' AND parent_id IS NOT NULL))
);
CREATE UNIQUE INDEX outline_node_target ON outline_nodes(tenant_id,novel_id,level,target_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX outline_master_one ON outline_nodes(tenant_id,novel_id) WHERE level='master' AND deleted_at IS NULL;
CREATE INDEX outline_nodes_list ON outline_nodes(tenant_id,novel_id,updated_at DESC,id DESC);
CREATE TABLE outline_beats (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, node_id UUID NOT NULL,
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  description TEXT NOT NULL DEFAULT '' CHECK(char_length(description)<=2000),
  beat_type TEXT NOT NULL CHECK(beat_type IN ('setup','conflict','turn','climax','resolution')),
  hook_type TEXT NOT NULL DEFAULT 'none' CHECK(hook_type IN ('none','danger','revelation','emotion')),
  tension INTEGER NOT NULL CHECK(tension BETWEEN 0 AND 100),
  expected_words INTEGER NOT NULL CHECK(expected_words BETWEEN 0 AND 200000),
  position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 199),
  status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','written')),
  deleted_at TIMESTAMPTZ,
  active_position INTEGER GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN position END) STORED,
  FOREIGN KEY(tenant_id,novel_id,node_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  UNIQUE(tenant_id,node_id,active_position) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX outline_beats_list ON outline_beats(tenant_id,novel_id,node_id,updated_at DESC,id DESC);
CREATE TABLE outline_scenes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, node_id UUID NOT NULL,
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  summary TEXT NOT NULL DEFAULT '' CHECK(char_length(summary)<=2000),
  pov_entity_id UUID, location_entity_id UUID,
  position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 199),
  status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','written')),
  source_revision BIGINT CHECK(source_revision>0), source_hash CHAR(64) CHECK(source_hash ~ '^[0-9a-f]{64}$'),
  start_offset INTEGER, end_offset INTEGER, deleted_at TIMESTAMPTZ,
  active_position INTEGER GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN position END) STORED,
  CHECK((source_revision IS NULL AND source_hash IS NULL AND start_offset IS NULL AND end_offset IS NULL) OR (source_revision IS NOT NULL AND source_hash IS NOT NULL AND start_offset IS NOT NULL AND end_offset IS NOT NULL AND start_offset>=0 AND end_offset>=start_offset AND end_offset<=200000)),
  FOREIGN KEY(tenant_id,novel_id,node_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  UNIQUE(tenant_id,node_id,active_position) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX outline_scenes_list ON outline_scenes(tenant_id,novel_id,node_id,updated_at DESC,id DESC);
CREATE TABLE outline_clues (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL,
  title TEXT NOT NULL CHECK(char_length(title) BETWEEN 1 AND 120),
  description TEXT NOT NULL DEFAULT '' CHECK(char_length(description)<=2000),
  status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','planted','developing','revealed','closed','abandoned')),
  planted_node_id UUID, planned_reveal_node_id UUID, last_node_id UUID,
  reason TEXT CHECK(char_length(reason) BETWEEN 1 AND 1000), deleted_at TIMESTAMPTZ,
  FOREIGN KEY(tenant_id,novel_id,planted_node_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  FOREIGN KEY(tenant_id,novel_id,planned_reveal_node_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  FOREIGN KEY(tenant_id,novel_id,last_node_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  UNIQUE(tenant_id,novel_id,id),
  CHECK(status NOT IN ('planted','developing','revealed','closed') OR (planted_node_id IS NOT NULL AND last_node_id IS NOT NULL)),
  CHECK(status<>'abandoned' OR reason IS NOT NULL)
);
CREATE INDEX outline_clues_list ON outline_clues(tenant_id,novel_id,status,updated_at DESC,id DESC);
CREATE INDEX outline_clues_target ON outline_clues(tenant_id,novel_id,planned_reveal_node_id) WHERE deleted_at IS NULL;
CREATE TABLE outline_clue_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  novel_id UUID NOT NULL, clue_id UUID NOT NULL,
  from_status TEXT NOT NULL CHECK(from_status IN ('planned','planted','developing','revealed','closed','abandoned')),
  to_status TEXT NOT NULL CHECK(to_status IN ('planted','developing','revealed','closed','abandoned')),
  node_id UUID, reason TEXT CHECK(char_length(reason) BETWEEN 1 AND 1000),
  actor_id UUID NOT NULL REFERENCES platform_users(id), clue_revision BIGINT NOT NULL CHECK(clue_revision>0),
  FOREIGN KEY(tenant_id,novel_id,clue_id) REFERENCES outline_clues(tenant_id,novel_id,id),
  FOREIGN KEY(tenant_id,novel_id,node_id) REFERENCES outline_nodes(tenant_id,novel_id,id),
  UNIQUE(tenant_id,clue_id,clue_revision)
);
CREATE INDEX outline_clue_history ON outline_clue_events(tenant_id,clue_id,created_at DESC,id DESC);
CREATE FUNCTION outline_reject_event_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'outline.immutable_event'; END; $$;
CREATE TRIGGER outline_clue_event_immutable BEFORE UPDATE ON outline_clue_events FOR EACH ROW EXECUTE FUNCTION outline_reject_event_update();
-- Runtime role: clue_events SELECT/INSERT only. Retention role may DELETE expired events.
-- Node level adjacency, membership scope and state transitions are Application invariants.
-- All outline mutations lock master node first and increment its revision once per transaction.
COMMIT;
