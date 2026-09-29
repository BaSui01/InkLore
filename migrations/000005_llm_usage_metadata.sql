-- 000005：llm_usage_records 补齐审计所需的元数据列
--
-- 背景：原表只有 request_id / model_key / channel_id / tokens / latency_ms / status，
-- 缺少租户作用域与失败原因，无法回答"哪个租户在什么情况下消耗了多少"。
-- 同样不修改已应用的 000001（迁移执行器以校验和保护历史脚本）。

ALTER TABLE llm_usage_records
    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default',
    ADD COLUMN error_text TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_llm_usage_tenant_created ON llm_usage_records (tenant_id, created_at DESC);
CREATE INDEX idx_llm_usage_request ON llm_usage_records (request_id);
