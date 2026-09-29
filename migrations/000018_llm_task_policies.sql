-- 000018：任务级模型策略（INK-066）
--
-- 背景：创作任务（大纲、正文、审校、摘要、命名、去 AI 味）此前没有各自的模型与参数档位，
-- 只能用同一个模型跑完所有环节——要么贵（审校用写作模型），要么差（正文用便宜模型）。
-- 行业标准做法是分档：推理模型做规划/审校，写作模型做正文，便宜模型做摘要。
--
-- 设计：
--   * task_key 是任务标识（outline/draft/review/summary/naming/deai），不是模型名；
--   * model_key 指向 llm_models.model_key，运行时经注册表解析出供应商与模型 ID
--     （复用 INK-050 的解析链路），因此改模型不需要改代码；
--   * 每租户一份（tenant_id + task_key 唯一），单机部署下全落默认租户。

CREATE TABLE llm_task_policies (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    task_key TEXT NOT NULL,
    model_key TEXT NOT NULL DEFAULT '',
    temperature DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    max_tokens INTEGER NOT NULL DEFAULT 0,
    timeout_seconds INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, task_key)
);

CREATE INDEX idx_llm_task_policies_tenant ON llm_task_policies (tenant_id, task_key);

-- 用量审计按任务维度聚合（INK-066 验收 2）：没有这一列就无法回答
-- 「摘要任务花了多少钱、用的哪个模型」。
ALTER TABLE llm_usage_records ADD COLUMN task_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_llm_usage_task ON llm_usage_records (tenant_id, task_key, created_at DESC);

-- +migrate Down
DROP INDEX IF EXISTS idx_llm_usage_task;
ALTER TABLE llm_usage_records DROP COLUMN IF EXISTS task_key;
DROP TABLE IF EXISTS llm_task_policies;
