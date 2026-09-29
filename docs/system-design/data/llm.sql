-- INK-141 R1. PG16. Apply after platform.sql. Domain owner: llm.
-- No plaintext credentials. Immutable versions/pricing/usage are INSERT-only.
BEGIN;

CREATE TABLE llm_providers(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 code varchar(64) NOT NULL CHECK(code ~ '^[a-z][a-z0-9_]*$'), title varchar(120) NOT NULL,
 protocol text NOT NULL CHECK(protocol IN ('openai','deepseek','anthropic','gemini','ollama')),
 enabled boolean NOT NULL DEFAULT false, deleted_at timestamptz, UNIQUE(tenant_id,code)
);

CREATE TABLE llm_models(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 provider_id uuid NOT NULL, upstream_name varchar(120) NOT NULL, title varchar(120) NOT NULL,
 capabilities text[] NOT NULL CHECK(cardinality(capabilities)>0 AND capabilities <@ ARRAY['text','tools','structured','embedding']::text[]),
 context_window integer NOT NULL CHECK(context_window BETWEEN 1 AND 2000000),
 max_output_tokens integer NOT NULL CHECK(max_output_tokens BETWEEN 1 AND 16384 AND max_output_tokens<context_window),
 embedding_dimensions integer CHECK(embedding_dimensions=1536),
 enabled boolean NOT NULL DEFAULT false, deleted_at timestamptz,
 CHECK(('embedding'=ANY(capabilities))=(embedding_dimensions IS NOT NULL)),
 FOREIGN KEY(tenant_id,provider_id) REFERENCES llm_providers(tenant_id,id), UNIQUE(tenant_id,provider_id,upstream_name),
 UNIQUE(tenant_id,provider_id,id)
);

CREATE TABLE llm_channels(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 provider_id uuid NOT NULL, model_id uuid NOT NULL, title varchar(120) NOT NULL,
 base_url varchar(2048) NOT NULL, upstream_model varchar(120) NOT NULL,
 priority integer NOT NULL DEFAULT 0 CHECK(priority BETWEEN 0 AND 100), weight integer NOT NULL CHECK(weight BETWEEN 1 AND 1000),
 rpm integer NOT NULL CHECK(rpm BETWEEN 1 AND 100000), concurrency integer NOT NULL CHECK(concurrency BETWEEN 1 AND 1000),
 enabled boolean NOT NULL DEFAULT false, deleted_at timestamptz,
 FOREIGN KEY(tenant_id,provider_id) REFERENCES llm_providers(tenant_id,id),
 FOREIGN KEY(tenant_id,provider_id,model_id) REFERENCES llm_models(tenant_id,provider_id,id)
);

CREATE TABLE llm_keys(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 channel_id uuid NOT NULL, title varchar(120) NOT NULL,
 secret_kind text NOT NULL CHECK(secret_kind IN ('env','secret_store')), secret_locator varchar(256) NOT NULL,
 secret_version varchar(64) NOT NULL, fingerprint char(64),
 weight integer NOT NULL CHECK(weight BETWEEN 1 AND 1000), rpm integer NOT NULL CHECK(rpm BETWEEN 1 AND 100000),
 concurrency integer NOT NULL CHECK(concurrency BETWEEN 1 AND 1000),
 health text NOT NULL DEFAULT 'unknown' CHECK(health IN ('unknown','healthy','degraded','invalid')),
 enabled boolean NOT NULL DEFAULT false, deleted_at timestamptz,
 CHECK(NOT enabled OR fingerprint IS NOT NULL),
 FOREIGN KEY(tenant_id,channel_id) REFERENCES llm_channels(tenant_id,id)
);

CREATE TABLE llm_task_policies(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 task_key varchar(64) NOT NULL CHECK(task_key ~ '^[a-z][a-z0-9_]*$'), title varchar(120) NOT NULL,
 current_version_id uuid, enabled boolean NOT NULL DEFAULT false, deleted_at timestamptz, UNIQUE(tenant_id,task_key)
);

CREATE TABLE llm_task_policy_versions(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 policy_id uuid NOT NULL, version integer NOT NULL CHECK(version>0),
 primary_model_id uuid NOT NULL, fallback_model_ids uuid[] NOT NULL DEFAULT '{}' CHECK(cardinality(fallback_model_ids)<=3),
 max_output_tokens integer NOT NULL CHECK(max_output_tokens BETWEEN 1 AND 16384), temperature numeric(3,2) NOT NULL CHECK(temperature BETWEEN 0 AND 2),
 timeout_seconds integer NOT NULL CHECK(timeout_seconds BETWEEN 1 AND 180), max_retries integer NOT NULL CHECK(max_retries BETWEEN 0 AND 3),
 required_capabilities text[] NOT NULL CHECK(required_capabilities <@ ARRAY['text','tools','structured','embedding']::text[]),
 output_mode text NOT NULL CHECK(output_mode IN ('text','structured','embedding')),
 task_kinds text[] NOT NULL DEFAULT '{}' CHECK(cardinality(task_kinds)<=16),
 FOREIGN KEY(tenant_id,policy_id) REFERENCES llm_task_policies(tenant_id,id),
 FOREIGN KEY(tenant_id,primary_model_id) REFERENCES llm_models(tenant_id,id), UNIQUE(tenant_id,policy_id,version)
);

ALTER TABLE llm_task_policies ADD FOREIGN KEY(tenant_id,current_version_id) REFERENCES llm_task_policy_versions(tenant_id,id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE llm_pricing_revisions(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 model_id uuid NOT NULL, version integer NOT NULL CHECK(version>0), currency char(3) NOT NULL DEFAULT 'CNY' CHECK(currency='CNY'),
 input_micros_per_million bigint NOT NULL CHECK(input_micros_per_million BETWEEN 0 AND 1000000000000),
 output_micros_per_million bigint NOT NULL CHECK(output_micros_per_million BETWEEN 0 AND 1000000000000),
 cache_micros_per_million bigint NOT NULL CHECK(cache_micros_per_million BETWEEN 0 AND 1000000000000),
 effective_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,model_id) REFERENCES llm_models(tenant_id,id),
 UNIQUE(tenant_id,model_id,version), UNIQUE(tenant_id,model_id,effective_at)
);

CREATE TABLE llm_budgets(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 daily_limit_micros bigint NOT NULL CHECK(daily_limit_micros BETWEEN 0 AND 9000000000000000), UNIQUE(tenant_id)
);

CREATE TABLE llm_budget_days(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 business_date date NOT NULL, limit_snapshot_micros bigint NOT NULL CHECK(limit_snapshot_micros>=0),
 reserved_micros bigint NOT NULL DEFAULT 0 CHECK(reserved_micros>=0), spent_micros bigint NOT NULL DEFAULT 0 CHECK(spent_micros>=0),
 over_budget boolean NOT NULL DEFAULT false, UNIQUE(tenant_id,business_date)
);

CREATE TABLE llm_reservations(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 UNIQUE(tenant_id,id),
 invocation_id uuid NOT NULL CHECK(substring(invocation_id::text,15,1)='4'), attempt integer NOT NULL CHECK(attempt BETWEEN 1 AND 4),
 business_date date NOT NULL, policy_version_id uuid NOT NULL, model_id uuid NOT NULL, channel_id uuid, key_id uuid,
 pricing_revision_id uuid NOT NULL, reserved_micros bigint NOT NULL CHECK(reserved_micros>=0),
 status text NOT NULL CHECK(status IN ('reserved','dispatched','settled','released')),
 dispatched_at timestamptz, lease_until timestamptz NOT NULL, finished_at timestamptz,
 FOREIGN KEY(tenant_id,business_date) REFERENCES llm_budget_days(tenant_id,business_date),
 FOREIGN KEY(tenant_id,policy_version_id) REFERENCES llm_task_policy_versions(tenant_id,id),
 FOREIGN KEY(tenant_id,model_id) REFERENCES llm_models(tenant_id,id),
 FOREIGN KEY(tenant_id,channel_id) REFERENCES llm_channels(tenant_id,id),
 FOREIGN KEY(tenant_id,key_id) REFERENCES llm_keys(tenant_id,id),
 FOREIGN KEY(tenant_id,pricing_revision_id) REFERENCES llm_pricing_revisions(tenant_id,id),
 UNIQUE(tenant_id,invocation_id,attempt)
);

CREATE TABLE llm_usage_records(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 invocation_id uuid NOT NULL, attempt integer NOT NULL CHECK(attempt BETWEEN 1 AND 4), reservation_id uuid NOT NULL,
 actor_user_id uuid REFERENCES platform_users(id), policy_version_id uuid NOT NULL, model_id uuid NOT NULL, channel_id uuid, key_id uuid,
 pricing_revision_id uuid NOT NULL, secret_version varchar(64), business_date date NOT NULL,
 input_tokens bigint NOT NULL CHECK(input_tokens>=0), output_tokens bigint NOT NULL CHECK(output_tokens>=0),
 cached_input_tokens bigint NOT NULL DEFAULT 0 CHECK(cached_input_tokens BETWEEN 0 AND input_tokens),
 cost_micros bigint NOT NULL CHECK(cost_micros>=0), currency char(3) NOT NULL DEFAULT 'CNY' CHECK(currency='CNY'),
 price_snapshot jsonb NOT NULL CHECK(jsonb_typeof(price_snapshot)='object'),
 estimated boolean NOT NULL, estimate_reason text CHECK(estimate_reason IN ('missing_usage','disconnected','crash','invalid_usage','tokenizer_unavailable')),
 status text NOT NULL CHECK(status IN ('succeeded','failed','cancelled','unknown')), error_code varchar(120),
 duration_ms bigint NOT NULL CHECK(duration_ms>=0), trace_id char(32) NOT NULL CHECK(trace_id ~ '^[0-9a-f]{32}$'),
 FOREIGN KEY(tenant_id,reservation_id) REFERENCES llm_reservations(tenant_id,id),
 FOREIGN KEY(tenant_id,policy_version_id) REFERENCES llm_task_policy_versions(tenant_id,id),
 FOREIGN KEY(tenant_id,model_id) REFERENCES llm_models(tenant_id,id),
 FOREIGN KEY(tenant_id,channel_id) REFERENCES llm_channels(tenant_id,id),
 FOREIGN KEY(tenant_id,key_id) REFERENCES llm_keys(tenant_id,id),
 FOREIGN KEY(tenant_id,pricing_revision_id) REFERENCES llm_pricing_revisions(tenant_id,id),
 UNIQUE(tenant_id,invocation_id,attempt), UNIQUE(tenant_id,reservation_id),
 CHECK((estimated AND estimate_reason IS NOT NULL) OR (NOT estimated AND estimate_reason IS NULL))
);

CREATE TABLE llm_usage_adjustments(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 usage_id uuid NOT NULL, delta_micros bigint NOT NULL CHECK(delta_micros<>0), reason varchar(256) NOT NULL,
 evidence_hash char(64) NOT NULL, actor_user_id uuid REFERENCES platform_users(id),
 FOREIGN KEY(tenant_id,usage_id) REFERENCES llm_usage_records(tenant_id,id), UNIQUE(tenant_id,usage_id,evidence_hash)
);

CREATE INDEX llm_providers_cursor ON llm_providers(tenant_id,updated_at DESC,id DESC);
CREATE INDEX llm_models_cursor ON llm_models(tenant_id,updated_at DESC,id DESC);
CREATE INDEX llm_channels_cursor ON llm_channels(tenant_id,updated_at DESC,id DESC);
CREATE INDEX llm_keys_cursor ON llm_keys(tenant_id,updated_at DESC,id DESC);
CREATE INDEX llm_task_policies_cursor ON llm_task_policies(tenant_id,updated_at DESC,id DESC);
CREATE INDEX llm_channels_route ON llm_channels(tenant_id,model_id,enabled,priority);
CREATE INDEX llm_keys_route ON llm_keys(tenant_id,channel_id,enabled);
CREATE INDEX llm_pricing_effective ON llm_pricing_revisions(tenant_id,model_id,effective_at DESC,version DESC);
CREATE INDEX llm_reservations_recover ON llm_reservations(status,lease_until) WHERE status IN ('reserved','dispatched');
CREATE INDEX llm_usage_cursor ON llm_usage_records(tenant_id,created_at DESC,id DESC);
CREATE INDEX llm_usage_model_date ON llm_usage_records(tenant_id,model_id,business_date);
COMMIT;
