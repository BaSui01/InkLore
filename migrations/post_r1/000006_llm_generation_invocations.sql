-- Bind persisted caller operations to the UUIDv4 used by the existing budget
-- and usage ledgers. No prompts, credentials or duplicate accounting are stored.
CREATE TABLE llm_generation_invocations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(substring(id::text,15,1)='4'),
 tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
 operation_key varchar(256) NOT NULL CHECK(length(operation_key)>0),
 request_hash char(64) NOT NULL CHECK(request_hash ~ '^[0-9a-f]{64}$'),
 response jsonb CHECK(response IS NULL OR jsonb_typeof(response)='object'),
 usage_record_id uuid,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,operation_key),
 FOREIGN KEY(tenant_id,usage_record_id) REFERENCES llm_usage_records(tenant_id,id),
 CHECK((response IS NULL) = (usage_record_id IS NULL))
);
