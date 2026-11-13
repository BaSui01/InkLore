-- INK-141 / R1 frozen target; PostgreSQL 16. Apply after platform.sql.
-- UUIDs supplied by application must be UUIDv4; no cross-domain business FK.
BEGIN;
CREATE TABLE worldview_revisions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, last_seq BIGINT NOT NULL DEFAULT 0 CHECK(last_seq>=0),
  UNIQUE(tenant_id,novel_id)
);
CREATE TABLE worldview_entities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('character','lore')),
  name TEXT NOT NULL CHECK(char_length(name) BETWEEN 1 AND 120),
  description TEXT NOT NULL DEFAULT '' CHECK(char_length(description)<=20000),
  aliases TEXT[] NOT NULL DEFAULT '{}',
  role_type TEXT CHECK(role_type IN ('protagonist','supporting','antagonist','other')),
  life_status TEXT CHECK(life_status IN ('alive','missing','dead','revived')),
  realm_rank INTEGER CHECK(realm_rank BETWEEN 0 AND 10000),
  category TEXT CHECK(category IN ('faction','place','system','item','history','other')),
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','archived')),
  deleted_at TIMESTAMPTZ,
  CHECK(cardinality(aliases)<=20),
  CHECK((kind='character' AND char_length(name)<=60 AND role_type IS NOT NULL AND life_status IS NOT NULL AND realm_rank IS NOT NULL AND category IS NULL) OR (kind='lore' AND role_type IS NULL AND life_status IS NULL AND realm_rank IS NULL AND category IS NOT NULL AND cardinality(aliases)=0)),
  UNIQUE(tenant_id,novel_id,id)
);
CREATE INDEX worldview_entities_list ON worldview_entities(tenant_id,novel_id,kind,updated_at DESC,id DESC);
CREATE TABLE worldview_relations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, subject_id UUID NOT NULL, object_id UUID NOT NULL,
  predicate TEXT NOT NULL CHECK(char_length(predicate) BETWEEN 1 AND 60),
  description TEXT NOT NULL DEFAULT '' CHECK(char_length(description)<=2000),
  weight INTEGER NOT NULL CHECK(weight BETWEEN 0 AND 100),
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','retired')), deleted_at TIMESTAMPTZ,
  FOREIGN KEY(tenant_id,novel_id,subject_id) REFERENCES worldview_entities(tenant_id,novel_id,id),
  FOREIGN KEY(tenant_id,novel_id,object_id) REFERENCES worldview_entities(tenant_id,novel_id,id),
  CHECK(subject_id<>object_id)
);
CREATE UNIQUE INDEX worldview_relation_active ON worldview_relations(tenant_id,novel_id,subject_id,predicate,object_id) WHERE deleted_at IS NULL AND status='active';
CREATE INDEX worldview_relation_outgoing ON worldview_relations(tenant_id,novel_id,subject_id,updated_at DESC,id DESC);
CREATE INDEX worldview_relation_incoming ON worldview_relations(tenant_id,novel_id,object_id);
CREATE INDEX worldview_relations_list ON worldview_relations(tenant_id,novel_id,updated_at DESC,id DESC);
CREATE TABLE worldview_fact_ledger_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  novel_id UUID NOT NULL, seq BIGINT NOT NULL CHECK(seq>0), world_revision BIGINT NOT NULL CHECK(world_revision>0),
  subject_id UUID NOT NULL,
  predicate TEXT NOT NULL CHECK(predicate IN ('life_status','realm_rank','item_status','location','relationship','custom')),
  fact_key TEXT NOT NULL CHECK(fact_key ~ '^[a-z][a-z0-9_]{0,119}$'),
  action TEXT NOT NULL CHECK(action IN ('assert','supersede','retract')),
  target_event_id UUID, resolution TEXT NOT NULL CHECK(resolution IN ('none','correction','retcon')),
  source_chapter_id UUID, source_version_id UUID,
  payload_digest CHAR(64) NOT NULL CHECK(payload_digest ~ '^[0-9a-f]{64}$'),
  UNIQUE(tenant_id,novel_id,id), UNIQUE(tenant_id,novel_id,seq),
  FOREIGN KEY(tenant_id,novel_id,target_event_id) REFERENCES worldview_fact_ledger_events(tenant_id,novel_id,id),
  CHECK((action='assert' AND target_event_id IS NULL AND resolution='none') OR (action IN ('supersede','retract') AND target_event_id IS NOT NULL AND resolution IN ('correction','retcon'))),
  CHECK((source_chapter_id IS NULL)=(source_version_id IS NULL))
);
CREATE INDEX worldview_ledger_page ON worldview_fact_ledger_events(tenant_id,novel_id,seq DESC);
CREATE INDEX worldview_ledger_subject ON worldview_fact_ledger_events(tenant_id,novel_id,subject_id,seq DESC);
CREATE TABLE worldview_fact_payloads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  novel_id UUID NOT NULL, event_id UUID NOT NULL,
  payload_salt UUID NOT NULL DEFAULT gen_random_uuid() CHECK(substring(payload_salt::text,15,1)='4'),
  value_kind TEXT CHECK(value_kind IN ('text','number','boolean','entity')),
  value_text TEXT CHECK(char_length(value_text) BETWEEN 1 AND 2000),
  value_number INTEGER CHECK(value_number BETWEEN 0 AND 10000),
  value_boolean BOOLEAN, value_entity_id UUID,
  reason TEXT CHECK(char_length(reason) BETWEEN 1 AND 1000),
  actor_id UUID REFERENCES platform_users(id),
  source_content_hash CHAR(64) CHECK(source_content_hash ~ '^[0-9a-f]{64}$'),
  FOREIGN KEY(tenant_id,novel_id,event_id) REFERENCES worldview_fact_ledger_events(tenant_id,novel_id,id),
  UNIQUE(tenant_id,novel_id,event_id),
  CHECK(((value_kind IS NULL AND num_nonnulls(value_text,value_number,value_boolean,value_entity_id)=0) OR (value_kind='text' AND value_text IS NOT NULL AND num_nonnulls(value_text,value_number,value_boolean,value_entity_id)=1) OR (value_kind='number' AND value_number IS NOT NULL AND num_nonnulls(value_text,value_number,value_boolean,value_entity_id)=1) OR (value_kind='boolean' AND value_boolean IS NOT NULL AND num_nonnulls(value_text,value_number,value_boolean,value_entity_id)=1) OR (value_kind='entity' AND value_entity_id IS NOT NULL AND num_nonnulls(value_text,value_number,value_boolean,value_entity_id)=1)) IS TRUE)
);
CREATE TABLE worldview_fact_heads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
  tenant_id UUID NOT NULL REFERENCES platform_tenants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
  novel_id UUID NOT NULL, subject_id UUID NOT NULL,
  predicate TEXT NOT NULL CHECK(predicate IN ('life_status','realm_rank','item_status','location','relationship','custom')),
  fact_key TEXT NOT NULL CHECK(fact_key ~ '^[a-z][a-z0-9_]{0,119}$'),
  event_id UUID NOT NULL, status TEXT NOT NULL CHECK(status IN ('active','retracted')),
  FOREIGN KEY(tenant_id,novel_id,event_id) REFERENCES worldview_fact_ledger_events(tenant_id,novel_id,id),
  UNIQUE(tenant_id,novel_id,subject_id,predicate,fact_key)
);
CREATE INDEX worldview_heads_page ON worldview_fact_heads(tenant_id,novel_id,updated_at DESC,id DESC);
CREATE INDEX worldview_heads_subject ON worldview_fact_heads(tenant_id,novel_id,subject_id,status);
CREATE FUNCTION worldview_reject_ledger_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'worldview.immutable_ledger'; END; $$;
CREATE TRIGGER worldview_ledger_immutable BEFORE UPDATE OR DELETE ON worldview_fact_ledger_events FOR EACH ROW EXECUTE FUNCTION worldview_reject_ledger_mutation();
CREATE FUNCTION worldview_reject_payload_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'worldview.immutable_payload'; END; $$;
CREATE TRIGGER worldview_payload_immutable BEFORE UPDATE ON worldview_fact_payloads FOR EACH ROW EXECUTE FUNCTION worldview_reject_payload_update();
-- Fact payload rows may be DELETEd by retention worker after 30 days; ledger remains only UUID/key/digest proof.
-- Payloads are immutable during business use; heads and character state are replayable projections.
COMMIT;
