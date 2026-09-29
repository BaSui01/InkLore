-- 000002：补齐渠道密钥/映射表（INK-043）与租户列（INK-044）
--
-- 设计约束：
--   1. 密钥明文**不入库**（ADR-0004）。llm_channel_keys 只保存 secret_ref ——
--      即"持有密钥的环境变量名"或 Secret Store 路径，运行时由基础设施层解析。
--   2. 租户列同时落在 novels 与其子表上：novels 是权威，子表的 tenant_id 用于
--      免 join 的作用域过滤；写入时必须与所属 novel 的租户一致。

-- ── 渠道密钥（routing.Pool 的候选集所需）─────────────────────
CREATE TABLE llm_channel_keys (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES llm_channels(id) ON DELETE CASCADE,
    key_name TEXT NOT NULL,
    -- secret_ref 只存"引用"，绝不存明文密钥（ADR-0004）
    secret_ref TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    daily_limit INTEGER NOT NULL DEFAULT 0,
    rate_limit INTEGER NOT NULL DEFAULT 0,
    concurrency_limit INTEGER NOT NULL DEFAULT 0,
    daily_used INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT llm_channel_keys_limit_non_negative
        CHECK (daily_limit >= 0 AND rate_limit >= 0 AND concurrency_limit >= 0),
    UNIQUE (channel_id, key_name)
);

CREATE INDEX idx_llm_channel_keys_channel ON llm_channel_keys (channel_id, status);

-- ── 渠道-模型映射（routing.ChannelModelMapping 所需）─────────
CREATE TABLE llm_channel_models (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES llm_channels(id) ON DELETE CASCADE,
    model_id BIGINT NOT NULL REFERENCES llm_models(id) ON DELETE CASCADE,
    -- upstream_model 为空表示直接使用 llm_models.model_key
    upstream_model TEXT NOT NULL DEFAULT '',
    weight INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    capability_status TEXT NOT NULL DEFAULT 'active',
    route_profile_hash TEXT NOT NULL DEFAULT '',
    pricing_snapshot_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT llm_channel_models_weight_non_negative CHECK (weight >= 0),
    UNIQUE (channel_id, model_id)
);

CREATE INDEX idx_llm_channel_models_model ON llm_channel_models (model_id, status);

-- ── 租户隔离（INK-044）───────────────────────────────────────
-- 默认值 'default' 让既有数据在升级瞬间即满足 NOT NULL；
-- 随后应通过一次性回填脚本按实际归属修正，再考虑移除默认值。
ALTER TABLE novels
    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX idx_novels_tenant ON novels (tenant_id, updated_at DESC);

ALTER TABLE novel_chapters
    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX idx_novel_chapters_tenant ON novel_chapters (tenant_id, novel_id);

ALTER TABLE novel_characters
    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX idx_novel_characters_tenant ON novel_characters (tenant_id, novel_id);

ALTER TABLE fact_ledger
    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX idx_fact_ledger_tenant ON fact_ledger (tenant_id, novel_id, category);
