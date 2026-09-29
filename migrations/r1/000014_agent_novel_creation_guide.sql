-- ADR-0032: persistent Agent-guided novel creation sessions and idempotent turns.
BEGIN;

CREATE TABLE agent_novel_creation_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
  tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
  owner_user_id uuid NOT NULL,
  start_key uuid NOT NULL, start_hash char(64) NOT NULL CHECK(start_hash ~ '^[0-9a-f]{64}$'),
  definition_version_id uuid NOT NULL, policy_version_id uuid NOT NULL,
  system_instruction text NOT NULL CHECK(length(system_instruction) BETWEEN 1 AND 8000),
  model_name varchar(120) NOT NULL, max_output_tokens integer NOT NULL CHECK(max_output_tokens BETWEEN 1 AND 16384),
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
