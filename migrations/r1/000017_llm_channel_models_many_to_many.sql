-- INK-230 / R1.x: Decouple Channel and Model into many-to-many relationship with llm_channel_models mapping table.
BEGIN;

-- 1. 创建渠道-模型多对多映射表
CREATE TABLE IF NOT EXISTS llm_channel_models (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK(id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    tenant_id uuid NOT NULL REFERENCES platform_tenants(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    channel_id uuid NOT NULL,
    model_id uuid NOT NULL,
    upstream_model varchar(120) NOT NULL DEFAULT '',
    priority integer NOT NULL DEFAULT 0 CHECK(priority BETWEEN 0 AND 100),
    weight integer NOT NULL DEFAULT 100 CHECK(weight BETWEEN 1 AND 1000),
    enabled boolean NOT NULL DEFAULT true,
    deleted_at timestamptz,
    FOREIGN KEY (tenant_id, channel_id) REFERENCES llm_channels(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, model_id) REFERENCES llm_models(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, channel_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_llm_channel_models_model ON llm_channel_models (tenant_id, model_id, enabled) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_llm_channel_models_channel ON llm_channel_models (tenant_id, channel_id) WHERE deleted_at IS NULL;

-- 2. 将既有 llm_channels 中的单模型关联平滑回填至映射表中
INSERT INTO llm_channel_models (tenant_id, channel_id, model_id, upstream_model, priority, weight, enabled, created_at, updated_at)
SELECT tenant_id, id, model_id, upstream_model, priority, weight, enabled, created_at, updated_at
FROM llm_channels
WHERE model_id IS NOT NULL
ON CONFLICT (tenant_id, channel_id, model_id) DO NOTHING;

-- 3. 解除 llm_channels 对 model_id 与 upstream_model 的非空约束
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (
        SELECT conname
        FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE t.relname = 'llm_channels'
          AND c.contype = 'f'
          AND pg_get_constraintdef(c.oid) LIKE '%model_id%'
    ) LOOP
        EXECUTE 'ALTER TABLE llm_channels DROP CONSTRAINT ' || quote_ident(r.conname);
    END LOOP;
END $$;

ALTER TABLE llm_channels ALTER COLUMN model_id DROP NOT NULL;
ALTER TABLE llm_channels ALTER COLUMN upstream_model DROP NOT NULL;
ALTER TABLE llm_channels ALTER COLUMN upstream_model SET DEFAULT '';

COMMIT;
