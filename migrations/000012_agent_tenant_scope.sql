-- 000012：智能体新表补齐租户作用域（INK-049）
--
-- 背景：000007 建的 10 张表只有 user_id/owner_user_id，没有 tenant_id；
-- 000010 的 artifact_documents 同样缺少。多租户部署下，相同用户 ID 在不同租户
-- 之间会互相可见，与「认证了但数据未隔离」是同一类缺陷。
--
-- 约束：这些表的写入代码不会提供 tenant_id，因此必须带 NOT NULL DEFAULT 'default'：
-- 未改造完成的写入路径落到显式默认租户，而不是直接报错；随后逐个仓储落地作用域。
--
-- 注意：user_agent_preferences / role_agent_access / role_model_access 的主键仍是
-- user_id / role_id；把它们改成 (tenant_id, ...) 复合主键会影响既有唯一约束语义，
-- 本迁移只补列与索引，主键演进留待这三张表被真正装配时再评估。

ALTER TABLE agent_run_nodes           ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE artifact_refs             ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agent_artifacts           ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agent_activity_logs       ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agent_tool_approvals      ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agent_mcp_servers         ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agent_mcp_tool_cache      ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE user_agent_preferences    ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE role_agent_access         ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE role_model_access         ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE artifact_documents        ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';

CREATE INDEX idx_agent_run_nodes_tenant       ON agent_run_nodes (tenant_id, run_id);
CREATE INDEX idx_artifact_refs_tenant         ON artifact_refs (tenant_id, updated_at);
CREATE INDEX idx_agent_artifacts_tenant       ON agent_artifacts (tenant_id, run_id);
CREATE INDEX idx_agent_activity_logs_tenant   ON agent_activity_logs (tenant_id, run_id);
CREATE INDEX idx_agent_tool_approvals_tenant  ON agent_tool_approvals (tenant_id, user_id, status);
CREATE INDEX idx_agent_mcp_servers_tenant     ON agent_mcp_servers (tenant_id, user_id, updated_at DESC);
CREATE INDEX idx_user_agent_prefs_tenant      ON user_agent_preferences (tenant_id);
CREATE INDEX idx_role_agent_access_tenant     ON role_agent_access (tenant_id);
CREATE INDEX idx_role_model_access_tenant     ON role_model_access (tenant_id);
CREATE INDEX idx_artifact_documents_tenant    ON artifact_documents (tenant_id, artifact_type, created_at DESC);

-- MCP 工具缓存的唯一键必须带上 tenant_id：否则两个租户的同名 Server 会互相覆盖。
DROP INDEX IF EXISTS idx_agent_mcp_tool_cache_key;
CREATE UNIQUE INDEX idx_agent_mcp_tool_cache_key
    ON agent_mcp_tool_cache (tenant_id, user_id, server_name, server_url, auth_fingerprint) NULLS NOT DISTINCT;
