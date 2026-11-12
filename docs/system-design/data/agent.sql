-- INK-141 既有基线 DDL；PostgreSQL 16 方言。在 platform.sql 之后执行。领域 Owner：agent。
-- Mutable updates require application CAS; immutable versions/events/artifacts INSERT-only.
BEGIN;

CREATE TABLE agent_definitions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  definition_key varchar(64) NOT NULL CHECK (definition_key ~ '^[a-z][a-z0-9_]*$'),
  title varchar(120) NOT NULL CHECK (length(title)>0), enabled boolean NOT NULL DEFAULT false,
  current_version_id uuid, deleted_at timestamptz,
  UNIQUE (tenant_id,definition_key)
);

CREATE TABLE agent_definition_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,id),
  definition_id uuid NOT NULL, version integer NOT NULL CHECK(version>0),
  system_instruction text NOT NULL CHECK(length(system_instruction) BETWEEN 1 AND 8000),
  tool_version_ids uuid[] NOT NULL DEFAULT '{}', task_kinds text[] NOT NULL,
  FOREIGN KEY(tenant_id,definition_id) REFERENCES agent_definitions(tenant_id,id),
  UNIQUE(tenant_id,definition_id,version)
);

ALTER TABLE agent_definitions ADD FOREIGN KEY(tenant_id,current_version_id) REFERENCES agent_definition_versions(tenant_id,id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE agent_mcp_servers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  title varchar(120) NOT NULL, endpoint varchar(2048) NOT NULL CHECK(endpoint LIKE 'https://%'),
  env_ref varchar(128) NOT NULL CHECK(env_ref ~ '^[A-Z][A-Z0-9_]*$'),
  allowed_methods text[] NOT NULL CHECK(allowed_methods <@ ARRAY['tools/list','tools/call']::text[]),
  enabled boolean NOT NULL DEFAULT false, deleted_at timestamptz
);

CREATE TABLE agent_tools (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  tool_key varchar(64) NOT NULL CHECK(tool_key ~ '^[a-z][a-z0-9_]*$'), title varchar(120) NOT NULL,
  enabled boolean NOT NULL DEFAULT false, current_version_id uuid, deleted_at timestamptz,
  UNIQUE(tenant_id,tool_key)
);

CREATE TABLE agent_tool_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,id),
  tool_id uuid NOT NULL, version integer NOT NULL CHECK(version>0),
  input_schema jsonb NOT NULL CHECK(jsonb_typeof(input_schema)='object'),
  output_schema jsonb NOT NULL CHECK(jsonb_typeof(output_schema)='object'),
  side_effect text NOT NULL CHECK(side_effect IN ('read_only','transactional','external')),
  approval_required boolean NOT NULL, timeout_seconds integer NOT NULL CHECK(timeout_seconds BETWEEN 1 AND 120),
  mcp_server_id uuid, remote_tool_name varchar(120),
  CHECK(side_effect<>'external' OR approval_required),
  CHECK((mcp_server_id IS NULL)=(remote_tool_name IS NULL)),
  FOREIGN KEY(tenant_id,tool_id) REFERENCES agent_tools(tenant_id,id),
  FOREIGN KEY(tenant_id,mcp_server_id) REFERENCES agent_mcp_servers(tenant_id,id),
  UNIQUE(tenant_id,tool_id,version)
);

ALTER TABLE agent_tools ADD FOREIGN KEY(tenant_id,current_version_id) REFERENCES agent_tool_versions(tenant_id,id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE agent_workflows (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  workflow_key varchar(64) NOT NULL CHECK(workflow_key ~ '^[a-z][a-z0-9_]*$'), title varchar(120) NOT NULL,
  enabled boolean NOT NULL DEFAULT false, current_version_id uuid, deleted_at timestamptz,
  UNIQUE(tenant_id,workflow_key)
);

CREATE TABLE agent_workflow_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,id),
  workflow_id uuid NOT NULL, version integer NOT NULL CHECK(version>0),
  manifest jsonb NOT NULL CHECK(jsonb_typeof(manifest)='object'), manifest_hash char(64) NOT NULL,
  FOREIGN KEY(tenant_id,workflow_id) REFERENCES agent_workflows(tenant_id,id),
  UNIQUE(tenant_id,workflow_id,version)
);

ALTER TABLE agent_workflows ADD FOREIGN KEY(tenant_id,current_version_id) REFERENCES agent_workflow_versions(tenant_id,id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE agent_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  platform_job_id uuid NOT NULL,
  UNIQUE(tenant_id,platform_job_id),
  FOREIGN KEY(tenant_id,platform_job_id) REFERENCES platform_jobs(tenant_id,id),
  kind text NOT NULL CHECK(kind IN ('creator','diagnostic')), task_kind varchar(64) NOT NULL,
  diagnostic_probe_key text CHECK(diagnostic_probe_key IN ('text_connectivity','structured_validation')),
  task_key varchar(64) NOT NULL, resource_type varchar(32), resource_id uuid,
  novel_id uuid, chapter_id uuid, source_revision bigint CHECK(source_revision>0), source_hash char(64),
  instruction text NOT NULL CHECK(length(instruction)<=8000),
  definition_version_id uuid, workflow_version_id uuid, policy_version_id uuid NOT NULL,
  status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','waiting_approval','waiting_clarification','cancelling','succeeded','failed','cancelled')),
  next_seq bigint NOT NULL DEFAULT 1 CHECK(next_seq>0), retained_from_seq bigint NOT NULL DEFAULT 1,
  deadline_at timestamptz NOT NULL, started_at timestamptz, finished_at timestamptz,
  error_code varchar(120), terminal_reason varchar(120), retryable boolean NOT NULL DEFAULT false,
  usage_receipt_ids uuid[] NOT NULL DEFAULT '{}', deleted_at timestamptz,
  CHECK((kind='diagnostic' AND novel_id IS NULL AND resource_id IS NULL AND diagnostic_probe_key IS NOT NULL) OR (kind='creator' AND diagnostic_probe_key IS NULL AND source_revision IS NOT NULL AND resource_id IS NOT NULL AND (task_kind='inspiration_analyze' OR novel_id IS NOT NULL))),
  CHECK((status IN ('succeeded','failed','cancelled'))=(finished_at IS NOT NULL)),
  FOREIGN KEY(tenant_id,definition_version_id) REFERENCES agent_definition_versions(tenant_id,id),
  FOREIGN KEY(tenant_id,workflow_version_id) REFERENCES agent_workflow_versions(tenant_id,id)
);

CREATE TABLE agent_run_nodes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  run_id uuid NOT NULL, node_key varchar(64) NOT NULL CHECK(node_key ~ '^[a-z][a-z0-9_]*$'), role varchar(32) NOT NULL CHECK(role IN ('supervisor','structure','writer','polisher','reviewer','tool')),
  depends_on text[] NOT NULL DEFAULT '{}', required boolean NOT NULL DEFAULT true,
  definition_version_id uuid, tool_version_id uuid,
  status text NOT NULL CHECK(status IN ('pending','ready','running','waiting_approval','waiting_clarification','retry_wait','succeeded','failed','cancelled','skipped')),
  phase text NOT NULL DEFAULT 'think' CHECK(phase IN ('think','act','observe')),
  iteration integer NOT NULL DEFAULT 0 CHECK(iteration BETWEEN 0 AND 15),
  attempt integer NOT NULL DEFAULT 0 CHECK(attempt BETWEEN 0 AND 4),
  lease_owner uuid, lease_until timestamptz, fencing_token bigint NOT NULL DEFAULT 0 CHECK(fencing_token>=0),
  job_epoch bigint NOT NULL DEFAULT 0 CHECK(job_epoch>=0),
  CHECK(status<>'running' OR job_epoch>0),
  next_attempt_at timestamptz NOT NULL DEFAULT now(), checkpoint jsonb NOT NULL DEFAULT '{}'::jsonb,
  error_code varchar(120), started_at timestamptz, finished_at timestamptz,
  FOREIGN KEY(tenant_id,run_id) REFERENCES agent_runs(tenant_id,id),
  FOREIGN KEY(tenant_id,definition_version_id) REFERENCES agent_definition_versions(tenant_id,id),
  FOREIGN KEY(tenant_id,tool_version_id) REFERENCES agent_tool_versions(tenant_id,id),
  UNIQUE(tenant_id,run_id,node_key), UNIQUE(tenant_id,run_id,id)
);

CREATE TABLE agent_executions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  run_id uuid NOT NULL, node_id uuid NOT NULL, tool_version_id uuid NOT NULL,
  execution_key uuid NOT NULL CHECK(substring(execution_key::text,15,1)='4'),
  iteration integer NOT NULL CHECK(iteration BETWEEN 1 AND 15), call_index integer NOT NULL CHECK(call_index BETWEEN 0 AND 7),
  arguments_hash char(64) NOT NULL, frozen_arguments jsonb NOT NULL,
  status text NOT NULL CHECK(status IN ('prepared','dispatching','succeeded','failed','unknown')),
  fencing_token bigint NOT NULL CHECK(fencing_token>0), job_epoch bigint NOT NULL CHECK(job_epoch>0), result_file_id uuid, result_hash char(64), upstream_receipt varchar(256),
  error_code varchar(120), dispatched_at timestamptz, finished_at timestamptz,
  FOREIGN KEY(tenant_id,run_id) REFERENCES agent_runs(tenant_id,id),
  FOREIGN KEY(tenant_id,run_id,node_id) REFERENCES agent_run_nodes(tenant_id,run_id,id),
  FOREIGN KEY(tenant_id,tool_version_id) REFERENCES agent_tool_versions(tenant_id,id),
  UNIQUE(tenant_id,execution_key), UNIQUE(tenant_id,node_id,iteration,call_index),
  UNIQUE(tenant_id,run_id,node_id,id,tool_version_id)
);

CREATE TABLE agent_approvals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
  UNIQUE (tenant_id,id),
  owner_user_id uuid NOT NULL REFERENCES platform_users(id), run_id uuid NOT NULL, node_id uuid NOT NULL,
  execution_id uuid NOT NULL, tool_version_id uuid NOT NULL, arguments_hash char(64) NOT NULL,
  source_revision bigint NOT NULL CHECK(source_revision>0),
  status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','consumed','rejected','expired','cancelled')),
  expires_at timestamptz NOT NULL, decided_at timestamptz, decision_reason varchar(2000),
  FOREIGN KEY(tenant_id,run_id) REFERENCES agent_runs(tenant_id,id),
  FOREIGN KEY(tenant_id,run_id,node_id) REFERENCES agent_run_nodes(tenant_id,run_id,id),
  FOREIGN KEY(tenant_id,run_id,node_id,execution_id,tool_version_id) REFERENCES agent_executions(tenant_id,run_id,node_id,id,tool_version_id),
  FOREIGN KEY(tenant_id,tool_version_id) REFERENCES agent_tool_versions(tenant_id,id),
  UNIQUE(tenant_id,execution_id), CHECK(expires_at>created_at AND expires_at<=created_at+interval '15 minutes')
);

CREATE TABLE agent_artifacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,id),
  run_id uuid NOT NULL, node_id uuid NOT NULL, kind text NOT NULL CHECK(kind IN ('chapter_draft','rewrite_suggestion','outline_suggestion','worldview_suggestion','analysis_result','audit_result')),
  target_resource_id uuid, source_revision bigint NOT NULL CHECK(source_revision>0),
  content_hash char(64) NOT NULL, text_content text, file_id uuid, is_partial boolean NOT NULL DEFAULT false,
  CHECK((text_content IS NULL)<>(file_id IS NULL)), CHECK(length(text_content)<=200000),
  FOREIGN KEY(tenant_id,run_id) REFERENCES agent_runs(tenant_id,id),
  FOREIGN KEY(tenant_id,run_id,node_id) REFERENCES agent_run_nodes(tenant_id,run_id,id)
);

CREATE TABLE agent_context_sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,id),
  run_id uuid NOT NULL, source_domain varchar(32) NOT NULL, source_type varchar(64) NOT NULL,
  source_id uuid NOT NULL, source_revision bigint NOT NULL CHECK(source_revision>0), content_hash char(64) NOT NULL,
  memory_kind text NOT NULL CHECK(memory_kind IN ('confirmed_fact','owner_snapshot','derived_summary','external_suggestion','tool_observation')),
  token_count integer NOT NULL CHECK(token_count>=0), estimated boolean NOT NULL DEFAULT false,
  disposition text NOT NULL CHECK(disposition IN ('included','compressed','dropped')),
  memory_parent_ids uuid[] NOT NULL DEFAULT '{}',
  FOREIGN KEY(tenant_id,run_id) REFERENCES agent_runs(tenant_id,id)
);

CREATE TABLE agent_run_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,id),
  run_id uuid NOT NULL, seq bigint NOT NULL CHECK(seq>0),
  event_name text NOT NULL CHECK(event_name IN ('run.started','text.delta','reasoning.summary','tool.started','tool.completed','approval.required','question.required','question.answered','node.updated','usage.updated','run.succeeded','run.failed','run.cancelled')),
  payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object'),
  expires_at timestamptz NOT NULL DEFAULT now()+interval '7 days',
  FOREIGN KEY(tenant_id,run_id) REFERENCES agent_runs(tenant_id,id), UNIQUE(tenant_id,run_id,seq)
);

CREATE UNIQUE INDEX agent_run_one_terminal_event ON agent_run_events(tenant_id,run_id) WHERE event_name IN ('run.succeeded','run.failed','run.cancelled');
CREATE INDEX agent_runs_owner_cursor ON agent_runs(tenant_id,owner_user_id,updated_at DESC,id DESC);
CREATE INDEX agent_runs_audit_cursor ON agent_runs(tenant_id,updated_at DESC,id DESC);
CREATE INDEX agent_nodes_claim ON agent_run_nodes(status,next_attempt_at,lease_until) WHERE status IN ('ready','retry_wait','running');
CREATE INDEX agent_approvals_owner_pending ON agent_approvals(tenant_id,owner_user_id,status,expires_at);
CREATE INDEX agent_artifacts_run ON agent_artifacts(tenant_id,run_id,created_at DESC,id DESC);
CREATE INDEX agent_event_expiry ON agent_run_events(expires_at);
CREATE INDEX agent_context_run ON agent_context_sources(tenant_id,run_id);
CREATE INDEX agent_definitions_cursor ON agent_definitions(tenant_id,updated_at DESC,id DESC);
CREATE INDEX agent_tools_cursor ON agent_tools(tenant_id,updated_at DESC,id DESC);
CREATE INDEX agent_workflows_cursor ON agent_workflows(tenant_id,updated_at DESC,id DESC);
CREATE INDEX agent_mcp_servers_cursor ON agent_mcp_servers(tenant_id,updated_at DESC,id DESC);
CREATE TABLE agent_run_questions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
  run_id uuid NOT NULL, node_id uuid NOT NULL, owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  question_version integer NOT NULL DEFAULT 1 CHECK(question_version=1),
  policy_version_id uuid NOT NULL, definition_version_id uuid NOT NULL,
  source_revision bigint NOT NULL CHECK(source_revision>0), source_hash char(64) NOT NULL CHECK(source_hash ~ '^[0-9a-f]{64}$'),
  title_key varchar(160) NOT NULL CHECK(title_key ~ '^[a-zA-Z0-9_.-]+$'),
  help_key varchar(160) NOT NULL CHECK(help_key ~ '^[a-zA-Z0-9_.-]+$'),
  context_summary text NOT NULL CHECK(length(context_summary)<=1000),
  response_schema jsonb NOT NULL CHECK(jsonb_typeof(response_schema)='object' AND octet_length(response_schema::text)<=8192),
  options jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(options)='array' AND jsonb_array_length(options)<=20 AND octet_length(options::text)<=4096),
  status text NOT NULL DEFAULT 'open' CHECK(status IN ('open','answered','expired','cancelled')),
  expires_at timestamptz NOT NULL CHECK(expires_at>created_at AND expires_at<=created_at+interval '15 minutes'),
  answer jsonb CHECK(answer IS NULL OR (jsonb_typeof(answer) IN ('string','array') AND octet_length(answer::text)<=4096)),
  answer_hash char(64), answered_by uuid REFERENCES platform_users(id), answered_at timestamptz,
  FOREIGN KEY(tenant_id,run_id,node_id) REFERENCES agent_run_nodes(tenant_id,run_id,id),
  UNIQUE(tenant_id,run_id,id),
  CHECK((status='answered')=(answer IS NOT NULL AND answer_hash IS NOT NULL AND answered_by IS NOT NULL AND answered_at IS NOT NULL)),
  CHECK((status='open')=(answered_at IS NULL))
);
CREATE UNIQUE INDEX agent_run_one_open_question ON agent_run_questions(tenant_id,run_id) WHERE status='open';
CREATE UNIQUE INDEX agent_run_question_version ON agent_run_questions(tenant_id,run_id,question_version,id);
CREATE INDEX agent_run_questions_owner_status ON agent_run_questions(tenant_id,owner_user_id,status,expires_at,id);
CREATE INDEX agent_run_questions_expiry ON agent_run_questions(expires_at) WHERE status='open';

CREATE TABLE agent_novel_creation_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
  owner_user_id uuid NOT NULL,
  start_key uuid NOT NULL, start_hash char(64) NOT NULL CHECK(start_hash ~ '^[0-9a-f]{64}$'),
  definition_version_id uuid NOT NULL, policy_version_id uuid NOT NULL,
  model_name varchar(120) NOT NULL,
  system_instruction text NOT NULL CHECK(length(system_instruction) BETWEEN 1 AND 8000),
  max_output_tokens integer NOT NULL CHECK(max_output_tokens BETWEEN 1 AND 16384),
  temperature double precision NOT NULL CHECK(temperature BETWEEN 0 AND 2),
  initial_idea text NOT NULL CHECK(length(initial_idea)<=2000),
  turns jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(turns)='array' AND jsonb_array_length(turns)<=8 AND octet_length(turns::text)<=32768),
  status text NOT NULL CHECK(status IN ('processing','waiting_user','proposed','failed','completed')),
  current_question jsonb CHECK(current_question IS NULL OR (jsonb_typeof(current_question)='object' AND octet_length(current_question::text)<=4096)),
  proposal jsonb CHECK(proposal IS NULL OR (jsonb_typeof(proposal)='object' AND octet_length(proposal::text)<=8192)),
  start_response jsonb CHECK(start_response IS NULL OR (jsonb_typeof(start_response)='object' AND octet_length(start_response::text)<=16384)),
  created_novel_id uuid,
  error_code varchar(120),
  expires_at timestamptz NOT NULL,
  FOREIGN KEY(tenant_id,owner_user_id) REFERENCES platform_memberships(tenant_id,user_id),
  FOREIGN KEY(tenant_id,definition_version_id) REFERENCES agent_definition_versions(tenant_id,id),
  UNIQUE(tenant_id,id),
  UNIQUE(tenant_id,owner_user_id,start_key),
  CHECK((status='waiting_user')=(current_question IS NOT NULL)),
  CHECK((status IN ('proposed','completed'))=(proposal IS NOT NULL)),
  CHECK((status='completed')=(created_novel_id IS NOT NULL)),
  CHECK((status='failed')=(error_code IS NOT NULL))
);
CREATE INDEX agent_novel_creation_owner_active ON agent_novel_creation_sessions(tenant_id,owner_user_id,updated_at DESC,id DESC) WHERE status IN ('processing','waiting_user','proposed');
CREATE INDEX agent_novel_creation_expiry ON agent_novel_creation_sessions(expires_at);

CREATE TABLE agent_novel_creation_commands (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  session_id uuid NOT NULL,
  owner_user_id uuid NOT NULL REFERENCES platform_users(id),
  command_type text NOT NULL CHECK(command_type IN ('answer','complete')),
  idempotency_key uuid NOT NULL,
  request_hash char(64) NOT NULL CHECK(request_hash ~ '^[0-9a-f]{64}$'),
  status text NOT NULL CHECK(status IN ('processing','completed','failed')),
  response jsonb CHECK(response IS NULL OR (jsonb_typeof(response)='object' AND octet_length(response::text)<=16384)),
  error_code varchar(120),
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY(tenant_id,session_id) REFERENCES agent_novel_creation_sessions(tenant_id,id),
  UNIQUE(tenant_id,owner_user_id,session_id,command_type,idempotency_key),
  CHECK((status='completed')=(response IS NOT NULL)),
  CHECK((status='failed')=(error_code IS NOT NULL))
);
CREATE INDEX agent_novel_creation_commands_session ON agent_novel_creation_commands(tenant_id,session_id,created_at DESC);
COMMIT;
