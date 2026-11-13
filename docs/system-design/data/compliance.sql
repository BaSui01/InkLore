-- 既有基线 DDL；PostgreSQL 16 方言。在 platform.sql 之后执行。
-- Validate JSONB through api/compliance-schemas.json. Never join business-domain tables.
BEGIN;

CREATE TABLE compliance_rule_sets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  name varchar(120) NOT NULL CHECK(char_length(name)>0),
  version_no bigint NOT NULL CHECK(version_no>0),
  state text NOT NULL DEFAULT 'draft' CHECK(state IN ('draft','published','retired')),
  checksum char(64) CHECK(checksum ~ '^[0-9a-f]{64}$'),
  created_by uuid NOT NULL REFERENCES platform_users(id),
  published_at timestamptz,
  UNIQUE(tenant_id,version_no),
  CHECK(state='draft' OR (checksum IS NOT NULL AND published_at IS NOT NULL)),
  UNIQUE (tenant_id,id)
);

CREATE UNIQUE INDEX compliance_rule_sets_current ON compliance_rule_sets(tenant_id) WHERE state='published';
CREATE INDEX compliance_rule_sets_list ON compliance_rule_sets(tenant_id,updated_at DESC,id DESC);

CREATE TABLE compliance_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  rule_set_id uuid NOT NULL,
  kind text NOT NULL CHECK(kind IN ('sensitive_literal','ai_phrase','ai_pattern')),
  expression varchar(256) NOT NULL CHECK(char_length(expression)>0),
  severity text NOT NULL CHECK(severity IN ('warning','block')),
  category varchar(60) NOT NULL CHECK(char_length(category)>0),
  advice varchar(2000) NOT NULL DEFAULT '',
  weight integer NOT NULL DEFAULT 10 CHECK(weight BETWEEN 1 AND 100),
  enabled boolean NOT NULL DEFAULT true,
  FOREIGN KEY(tenant_id,rule_set_id) REFERENCES compliance_rule_sets(tenant_id,id),
  UNIQUE(tenant_id,rule_set_id,kind,expression),
  CHECK(kind='ai_pattern' OR char_length(expression)<=64),
  UNIQUE (tenant_id,id)
);

CREATE INDEX compliance_rules_list ON compliance_rules(tenant_id,rule_set_id,updated_at DESC,id DESC);

CREATE TABLE compliance_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  novel_id uuid NOT NULL,
  chapter_id uuid NOT NULL,
  chapter_revision bigint NOT NULL CHECK(chapter_revision>0),
  content_hash char(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  source_text text NOT NULL CHECK(char_length(source_text)<=200000),
  rule_set_id uuid NOT NULL,
  mode text NOT NULL CHECK(mode IN ('rules','deep')),
  state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','running','succeeded','failed','cancelled')),
  run_id uuid,
  execution_key uuid,
  execution_fence bigint NOT NULL DEFAULT 0 CHECK(execution_fence>=0),
  total_tokens integer NOT NULL DEFAULT 30000 CHECK(total_tokens BETWEEN 1000 AND 120000),
  max_output_tokens integer NOT NULL DEFAULT 4000 CHECK(max_output_tokens BETWEEN 256 AND 8000 AND max_output_tokens<=total_tokens),
  max_calls integer NOT NULL DEFAULT 4 CHECK(max_calls BETWEEN 1 AND 6),
  used_tokens integer NOT NULL DEFAULT 0 CHECK(used_tokens>=0),
  reserved_tokens integer NOT NULL DEFAULT 0 CHECK(reserved_tokens>=0),
  calls_used integer NOT NULL DEFAULT 0 CHECK(calls_used>=0 AND calls_used<=max_calls),
  summary text NOT NULL DEFAULT '' CHECK(char_length(summary)<=4000),
  ai_flavor_score integer CHECK(ai_flavor_score BETWEEN 0 AND 100),
  formula_version text NOT NULL DEFAULT 'r1' CHECK(formula_version='r1'),
  risk_count integer NOT NULL DEFAULT 0 CHECK(risk_count BETWEEN 0 AND 10000),
  open_count integer NOT NULL DEFAULT 0 CHECK(open_count>=0 AND open_count<=risk_count),
  block_count integer NOT NULL DEFAULT 0 CHECK(block_count>=0 AND block_count<=risk_count),
  failure_code varchar(120),
  expire_at timestamptz NOT NULL DEFAULT (now()+interval '30 days'),
  redacted_at timestamptz,
  FOREIGN KEY(tenant_id,rule_set_id) REFERENCES compliance_rule_sets(tenant_id,id),
  CHECK(used_tokens+reserved_tokens<=total_tokens),
  UNIQUE (tenant_id,id)
);

CREATE INDEX compliance_reports_list ON compliance_reports(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE INDEX compliance_reports_chapter ON compliance_reports(tenant_id,owner_user_id,chapter_id,updated_at DESC,id DESC);
CREATE UNIQUE INDEX compliance_reports_execution ON compliance_reports(tenant_id,execution_key) WHERE execution_key IS NOT NULL;
CREATE INDEX compliance_reports_gc ON compliance_reports(expire_at,id) WHERE redacted_at IS NULL;

CREATE TABLE compliance_issues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  report_id uuid NOT NULL,
  rule_id uuid,
  detector text NOT NULL CHECK(detector IN ('rule','llm')),
  kind text NOT NULL CHECK(kind IN ('sensitive','ai_flavor')),
  severity text NOT NULL CHECK(severity IN ('warning','block')),
  start_offset integer NOT NULL CHECK(start_offset>=0),
  end_offset integer NOT NULL CHECK(end_offset>start_offset AND end_offset<=200000),
  excerpt varchar(500) NOT NULL,
  excerpt_truncated boolean NOT NULL DEFAULT false,
  message varchar(2000) NOT NULL,
  suggestion varchar(2000) NOT NULL DEFAULT '',
  confidence numeric(5,4) NOT NULL CHECK(confidence BETWEEN 0 AND 1),
  state text NOT NULL DEFAULT 'open' CHECK(state IN ('open','ignored')),
  FOREIGN KEY(tenant_id,report_id) REFERENCES compliance_reports(tenant_id,id),
  FOREIGN KEY(tenant_id,rule_id) REFERENCES compliance_rules(tenant_id,id),
  UNIQUE(tenant_id,report_id,id),
  UNIQUE (tenant_id,id)
);

CREATE INDEX compliance_issues_list ON compliance_issues(tenant_id,report_id,updated_at DESC,id DESC);
CREATE INDEX compliance_issues_open ON compliance_issues(tenant_id,report_id,state);
CREATE UNIQUE INDEX compliance_issues_dedupe ON compliance_issues(tenant_id,report_id,rule_id,start_offset,end_offset) WHERE rule_id IS NOT NULL;

CREATE TABLE compliance_issue_decisions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  report_id uuid NOT NULL,
  issue_id uuid NOT NULL,
  actor_user_id uuid NOT NULL REFERENCES platform_users(id),
  action text NOT NULL CHECK(action IN ('ignore','reopen')),
  reason varchar(500) NOT NULL CHECK(char_length(reason)>0),
  issue_revision bigint NOT NULL CHECK(issue_revision>0),
  FOREIGN KEY(tenant_id,report_id,issue_id) REFERENCES compliance_issues(tenant_id,report_id,id),
  UNIQUE (tenant_id,id)
);

CREATE INDEX compliance_issue_decisions_list ON compliance_issue_decisions(tenant_id,issue_id,created_at DESC,id DESC);

CREATE TABLE compliance_reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  report_id uuid NOT NULL,
  report_revision bigint NOT NULL CHECK(report_revision>0),
  chapter_id uuid NOT NULL,
  chapter_revision bigint NOT NULL CHECK(chapter_revision>0),
  content_hash char(64) NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
  rule_set_id uuid NOT NULL,
  actor_user_id uuid NOT NULL REFERENCES platform_users(id),
  acknowledged_risk boolean NOT NULL,
  note varchar(1000) NOT NULL DEFAULT '',
  FOREIGN KEY(tenant_id,report_id) REFERENCES compliance_reports(tenant_id,id),
  FOREIGN KEY(tenant_id,rule_set_id) REFERENCES compliance_rule_sets(tenant_id,id),
  UNIQUE (tenant_id,id)
);

CREATE INDEX compliance_reviews_list ON compliance_reviews(tenant_id,report_id,created_at DESC,id DESC);

CREATE TABLE compliance_rewrites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  report_id uuid NOT NULL,
  report_revision bigint NOT NULL CHECK(report_revision>0),
  selected_issue_ids uuid[] NOT NULL CHECK(cardinality(selected_issue_ids) BETWEEN 1 AND 100),
  state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','running','succeeded','failed','cancelled')),
  max_rounds integer NOT NULL CHECK(max_rounds BETWEEN 1 AND 3),
  rounds_used integer NOT NULL DEFAULT 0 CHECK(rounds_used>=0 AND rounds_used<=max_rounds),
  total_tokens integer NOT NULL CHECK(total_tokens BETWEEN 1000 AND 120000),
  max_output_tokens integer NOT NULL CHECK(max_output_tokens BETWEEN 256 AND 8000 AND max_output_tokens<=total_tokens),
  max_calls integer NOT NULL CHECK(max_calls BETWEEN 1 AND 6),
  used_tokens integer NOT NULL DEFAULT 0 CHECK(used_tokens>=0),
  reserved_tokens integer NOT NULL DEFAULT 0 CHECK(reserved_tokens>=0),
  calls_used integer NOT NULL DEFAULT 0 CHECK(calls_used>=0 AND calls_used<=max_calls),
  run_id uuid,
  execution_key uuid,
  execution_fence bigint NOT NULL DEFAULT 0 CHECK(execution_fence>=0),
  patches jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(patches)='array' AND jsonb_array_length(patches)<=100),
  candidate_text text CHECK(char_length(candidate_text)<=200000),
  candidate_hash char(64) CHECK(candidate_hash ~ '^[0-9a-f]{64}$'),
  remaining_risk_count integer CHECK(remaining_risk_count>=0),
  stop_reason text CHECK(stop_reason IN ('clean','max_rounds','budget','no_improvement')),
  failure_code varchar(120),
  redacted_at timestamptz,
  FOREIGN KEY(tenant_id,report_id) REFERENCES compliance_reports(tenant_id,id),
  CHECK(used_tokens+reserved_tokens<=total_tokens),
  UNIQUE (tenant_id,id)
);

CREATE INDEX compliance_rewrites_list ON compliance_rewrites(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE INDEX compliance_rewrites_report ON compliance_rewrites(tenant_id,report_id,updated_at DESC,id DESC);
CREATE UNIQUE INDEX compliance_rewrites_execution ON compliance_rewrites(tenant_id,execution_key) WHERE execution_key IS NOT NULL;
COMMIT;
