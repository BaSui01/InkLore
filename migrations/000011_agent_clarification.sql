-- INK-227 R1.x additive migration. Apply after migrations/r1/000010_llm_policy_task_kinds.sql.
-- Upgrade is transactional and preserves all existing runs/events. Downgrade is intentionally unsupported:
-- answering questions and new event names are durable history and cannot be safely discarded.
BEGIN;

ALTER TABLE agent_runs DROP CONSTRAINT IF EXISTS agent_runs_status_check;
ALTER TABLE agent_runs ADD CONSTRAINT agent_runs_status_check CHECK(status IN ('queued','running','waiting_approval','waiting_clarification','cancelling','succeeded','failed','cancelled'));
ALTER TABLE agent_run_nodes DROP CONSTRAINT IF EXISTS agent_run_nodes_status_check;
ALTER TABLE agent_run_nodes ADD CONSTRAINT agent_run_nodes_status_check CHECK(status IN ('pending','ready','running','waiting_approval','waiting_clarification','retry_wait','succeeded','failed','cancelled','skipped'));
ALTER TABLE agent_run_events DROP CONSTRAINT IF EXISTS agent_run_events_event_name_check;
ALTER TABLE agent_run_events ADD CONSTRAINT agent_run_events_event_name_check CHECK(event_name IN ('run.started','text.delta','reasoning.summary','tool.started','tool.completed','approval.required','question.required','question.answered','node.updated','usage.updated','run.succeeded','run.failed','run.cancelled'));

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

COMMIT;
