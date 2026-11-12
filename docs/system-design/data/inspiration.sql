-- 既有基线 DDL；PostgreSQL 16 方言。在 platform.sql 之后执行。
-- All writes are tenant-scoped; mutable writes CAS revision and update updated_at.
-- JSONB payloads are validated by api/inspiration-schemas.json before persistence.
BEGIN;

CREATE TABLE inspiration_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  file_id uuid NOT NULL REFERENCES platform_files(id),
  title varchar(120) NOT NULL CHECK (char_length(title)>0),
  source_kind text NOT NULL CHECK (source_kind IN ('file','paste')),
  author varchar(120) NOT NULL DEFAULT '',
  source_url varchar(2048),
  source_note text NOT NULL CHECK (char_length(source_note) BETWEEN 1 AND 2000),
  rights_status text NOT NULL CHECK (rights_status IN ('owned','licensed','public_domain','unknown')),
  content_hash char(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
  codepoint_count integer NOT NULL CHECK (codepoint_count BETWEEN 1 AND 200000),
  state text NOT NULL DEFAULT 'ready' CHECK (state IN ('ready','deleted')),
  deleted_at timestamptz,
  redacted_at timestamptz,
  CHECK ((state='deleted')=(deleted_at IS NOT NULL)),
  UNIQUE (tenant_id,id)
);

CREATE UNIQUE INDEX inspiration_assets_dedupe ON inspiration_assets(tenant_id,owner_user_id,content_hash) WHERE deleted_at IS NULL;
CREATE INDEX inspiration_assets_list ON inspiration_assets(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE INDEX inspiration_assets_gc ON inspiration_assets(deleted_at,id) WHERE deleted_at IS NOT NULL AND redacted_at IS NULL;

CREATE TABLE inspiration_segments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  asset_id uuid NOT NULL,
  seq integer NOT NULL CHECK (seq>0),
  start_offset integer NOT NULL CHECK (start_offset>=0),
  end_offset integer NOT NULL CHECK (end_offset>start_offset AND end_offset-start_offset<=4000),
  FOREIGN KEY (tenant_id,asset_id) REFERENCES inspiration_assets(tenant_id,id),
  UNIQUE(tenant_id,asset_id,seq),
  UNIQUE (tenant_id,id)
);

CREATE TABLE inspiration_analyses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  novel_id uuid,
  asset_id uuid,
  asset_revision bigint CHECK (asset_revision>0),
  kind text NOT NULL CHECK (kind IN ('deconstruction','style','ideas')),
  brief text NOT NULL DEFAULT '' CHECK (char_length(brief)<=4000),
  state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','running','succeeded','failed','cancelled')),
  run_id uuid,
  execution_key uuid,
  execution_fence bigint NOT NULL DEFAULT 0 CHECK(execution_fence>=0),
  total_tokens integer NOT NULL CHECK(total_tokens BETWEEN 1000 AND 120000),
  max_output_tokens integer NOT NULL CHECK(max_output_tokens BETWEEN 256 AND 8000 AND max_output_tokens<=total_tokens),
  max_calls integer NOT NULL CHECK(max_calls BETWEEN 1 AND 20),
  used_tokens integer NOT NULL DEFAULT 0 CHECK(used_tokens>=0),
  reserved_tokens integer NOT NULL DEFAULT 0 CHECK(reserved_tokens>=0),
  calls_used integer NOT NULL DEFAULT 0 CHECK(calls_used>=0 AND calls_used<=max_calls),
  failure_code varchar(120),
  FOREIGN KEY(tenant_id,asset_id) REFERENCES inspiration_assets(tenant_id,id),
  CHECK ((asset_id IS NULL)=(asset_revision IS NULL)),
  CHECK (kind='ideas' OR asset_id IS NOT NULL),
  CHECK (used_tokens+reserved_tokens<=total_tokens),
  UNIQUE (tenant_id,id)
);

CREATE INDEX inspiration_analyses_list ON inspiration_analyses(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE UNIQUE INDEX inspiration_analyses_execution ON inspiration_analyses(tenant_id,execution_key) WHERE execution_key IS NOT NULL;
CREATE INDEX inspiration_analyses_asset ON inspiration_analyses(tenant_id,asset_id,state);
CREATE INDEX inspiration_analyses_unclaimed ON inspiration_analyses(created_at,id) WHERE state='pending';

CREATE TABLE inspiration_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  analysis_id uuid NOT NULL,
  summary text NOT NULL CHECK(char_length(summary)<=4000),
  findings jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(findings)='array' AND jsonb_array_length(findings)<=100),
  patterns jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(patterns)='array' AND jsonb_array_length(patterns)<=20),
  style_metrics jsonb CHECK(style_metrics IS NULL OR jsonb_typeof(style_metrics)='object'),
  ideas jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(ideas)='array' AND jsonb_array_length(ideas)<=10),
  redacted_at timestamptz,
  FOREIGN KEY(tenant_id,analysis_id) REFERENCES inspiration_analyses(tenant_id,id),
  UNIQUE(tenant_id,analysis_id),
  UNIQUE (tenant_id,id)
);

CREATE TABLE inspiration_styles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  title varchar(120) NOT NULL CHECK(char_length(title)>0),
  current_version_id uuid NOT NULL,
  state text NOT NULL DEFAULT 'active' CHECK(state IN ('active','deleted')),
  deleted_at timestamptz,
  redacted_at timestamptz,
  CHECK ((state='deleted')=(deleted_at IS NOT NULL)),
  UNIQUE (tenant_id,id)
);

CREATE TABLE inspiration_style_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  style_id uuid NOT NULL,
  version_no integer NOT NULL CHECK(version_no>0),
  instructions text NOT NULL CHECK(char_length(instructions)<=4000),
  redacted_at timestamptz,
  CHECK(char_length(instructions)>0 OR redacted_at IS NOT NULL),
  metrics jsonb NOT NULL CHECK(jsonb_typeof(metrics)='object'),
  source_report_id uuid,
  FOREIGN KEY(tenant_id,style_id) REFERENCES inspiration_styles(tenant_id,id),
  FOREIGN KEY(tenant_id,source_report_id) REFERENCES inspiration_reports(tenant_id,id),
  UNIQUE(tenant_id,style_id,version_no),
  UNIQUE(tenant_id,style_id,id),
  UNIQUE (tenant_id,id)
);

ALTER TABLE inspiration_styles ADD CONSTRAINT inspiration_styles_head_fk FOREIGN KEY(tenant_id,id,current_version_id) REFERENCES inspiration_style_versions(tenant_id,style_id,id) DEFERRABLE INITIALLY DEFERRED;
CREATE INDEX inspiration_styles_list ON inspiration_styles(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE INDEX inspiration_style_versions_list ON inspiration_style_versions(tenant_id,style_id,created_at DESC,id DESC);

CREATE TABLE inspiration_cards (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  novel_id uuid,
  title varchar(120) NOT NULL CHECK(char_length(title)>0),
  category text NOT NULL CHECK(category IN ('opening','conflict','twist','cliffhanger','setting')),
  body text NOT NULL CHECK(char_length(body)<=4000),
  rationale text NOT NULL DEFAULT '' CHECK(char_length(rationale)<=2000),
  source_kind text NOT NULL CHECK(source_kind IN ('original','reference')),
  source_report_id uuid,
  evidence jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(evidence)='array' AND jsonb_array_length(evidence)<=20),
  fingerprint char(64) NOT NULL CHECK(fingerprint ~ '^[0-9a-f]{64}$'),
  state text NOT NULL DEFAULT 'suggested' CHECK(state IN ('suggested','archived','deleted')),
  deleted_at timestamptz,
  redacted_at timestamptz,
  FOREIGN KEY(tenant_id,source_report_id) REFERENCES inspiration_reports(tenant_id,id),
  CHECK ((state='deleted')=(deleted_at IS NOT NULL)),
  CHECK(char_length(body)>0 OR redacted_at IS NOT NULL),
  UNIQUE (tenant_id,id)
);

CREATE UNIQUE INDEX inspiration_cards_dedupe ON inspiration_cards(tenant_id,owner_user_id,novel_id,fingerprint) NULLS NOT DISTINCT WHERE deleted_at IS NULL;
CREATE INDEX inspiration_cards_list ON inspiration_cards(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE INDEX inspiration_cards_novel_list ON inspiration_cards(tenant_id,owner_user_id,novel_id,updated_at DESC,id DESC);

CREATE TABLE inspiration_adoptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  card_id uuid NOT NULL,
  actor_user_id uuid NOT NULL REFERENCES platform_users(id),
  target_domain text NOT NULL CHECK(target_domain IN ('content','outline')),
  target_id uuid NOT NULL,
  target_revision bigint NOT NULL CHECK(target_revision>0),
  target_hash char(64) NOT NULL CHECK(target_hash ~ '^[0-9a-f]{64}$'),
  state text NOT NULL DEFAULT 'active' CHECK(state IN ('active','revoked')),
  revoke_reason varchar(500),
  revoked_at timestamptz,
  FOREIGN KEY(tenant_id,card_id) REFERENCES inspiration_cards(tenant_id,id),
  CHECK((state='revoked')=(revoked_at IS NOT NULL)),
  CHECK(state<>'revoked' OR char_length(revoke_reason)>0),
  UNIQUE(tenant_id,card_id,target_domain,target_id,target_revision),
  UNIQUE (tenant_id,id)
);

CREATE INDEX inspiration_adoptions_list ON inspiration_adoptions(tenant_id,card_id,created_at DESC,id DESC);
COMMIT;
