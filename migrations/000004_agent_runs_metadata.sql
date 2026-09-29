-- 000004：agent_runs 补齐运行记录所需的元数据列
--
-- 背景：agent_runs 原本只有 id/agent_id/request_id/status/快照列，
-- 无法承载租户作用域、字符串 Agent 标识与运行收尾信息。
-- 说明：同目录的移植仓储（agentrun/repository.go）还需要 agent_run_nodes、
-- artifact_refs 等 10 张本项目尚未创建的表（见 INK-045），
-- 本迁移只补齐"运行主记录 + JSONB 事件流"这一层真正需要的列。

ALTER TABLE agent_runs
    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default',
    ADD COLUMN agent_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN session_id BIGINT,
    ADD COLUMN user_id BIGINT,
    ADD COLUMN error_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX idx_agent_runs_tenant_created ON agent_runs (tenant_id, created_at DESC);
CREATE INDEX idx_agent_runs_request ON agent_runs (request_id);
