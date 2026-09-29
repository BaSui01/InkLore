-- 000007：补齐智能体持久化 schema（INK-045）
--
-- 背景：移植进 internal/agent/infrastructure/persistence 的持久化代码引用了 10 张本项目
-- 尚未创建的表，同时与已有 agent_runs 表存在 3 处约束冲突。本迁移按 docs/backlog.md
-- INK-045 落地；列清单由代码实际读写位置提取（见 CHANGELOG 的证据与 ADR-0008）。
--
-- 落型约定（重要）：
--   * 只有 agentrun/recorder.go 对 *_snapshot 列显式绑定 ?::jsonb；其余 JSON 语义的列
--     在 SQL 中按 string 绑定，若落 jsonb 会在 pgx 扩展协议下报 text→jsonb 无隐式转换，
--     故这些列一律 TEXT。
--   * 所有 id 走 RETURNING id 并被 Scan 成 Go int64 → BIGSERIAL。
--   * 时间列统一 TIMESTAMPTZ（AGENTS.md 北京时间契约：UTC 存储）。
--
-- 历史迁移受校验和保护，本文件创建后不得修改。

-- ── 1. 已有表 agent_runs 的补齐与约束调整 ──────────────────────────────────
ALTER TABLE agent_runs
    ADD COLUMN input_json    TEXT NOT NULL DEFAULT '{}',
    ADD COLUMN output_json   TEXT NOT NULL DEFAULT '{}',
    ADD COLUMN "error"       TEXT NOT NULL DEFAULT '',
    ADD COLUMN started_at    TIMESTAMPTZ,
    ADD COLUMN finished_at   TIMESTAMPTZ,
    ADD COLUMN execution_key TEXT NOT NULL DEFAULT '';

-- CreateRun 的 INSERT 不含 request_id，必须给默认值。
ALTER TABLE agent_runs ALTER COLUMN request_id SET DEFAULT '';
-- repository.go 用 NULLIF(?, '') 表达「无租户写 NULL」，NOT NULL 会导致空租户写入失败。
ALTER TABLE agent_runs ALTER COLUMN tenant_id DROP NOT NULL;
-- CreateRun 直接传 agent_id（未设置时为 0），保留外键会让该路径 100% 违约。
ALTER TABLE agent_runs DROP CONSTRAINT IF EXISTS agent_runs_agent_id_fkey;

-- BindExecutionKey 依赖「部分唯一索引」实现跨进程互斥；全量唯一索引会让多行默认 '' 互斥。
CREATE UNIQUE INDEX idx_agent_runs_execution_key
    ON agent_runs (execution_key) WHERE execution_key <> '';
CREATE INDEX idx_agent_runs_session
    ON agent_runs (user_id, session_id, created_at DESC, id DESC);

-- ── 2. agent_run_nodes ────────────────────────────────────────────────────
CREATE TABLE agent_run_nodes (
    id          BIGSERIAL PRIMARY KEY,
    run_id      BIGINT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    node_id     TEXT NOT NULL,
    node_type   TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL,
    input_json  TEXT NOT NULL DEFAULT '{}',
    output_json TEXT NOT NULL DEFAULT '{}',
    "error"     TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_run_nodes_run ON agent_run_nodes (run_id);
CREATE INDEX idx_agent_run_nodes_run_node_status ON agent_run_nodes (run_id, node_id, status);

-- ── 3. artifact_refs ─────────────────────────────────────────────────────
CREATE TABLE artifact_refs (
    id                  BIGSERIAL PRIMARY KEY,
    artifact_type       TEXT NOT NULL,
    owner_user_id       BIGINT,
    workflow_id         BIGINT,
    generation_task_id  BIGINT,
    agent_run_id        BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    agent_node_run_id   BIGINT,
    source              TEXT NOT NULL DEFAULT '',
    storage_object_id   TEXT NOT NULL DEFAULT '',
    storage_key         TEXT NOT NULL DEFAULT '',
    storage_backend     TEXT NOT NULL DEFAULT 'postgres',
    content_hash        TEXT NOT NULL DEFAULT '',
    size_bytes          BIGINT NOT NULL DEFAULT 0,
    status              TEXT NOT NULL,
    idempotency_key     TEXT NOT NULL DEFAULT '',
    attempt_count       INTEGER NOT NULL DEFAULT 1,
    last_error_category TEXT NOT NULL DEFAULT '',
    ready_at            TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Saga.Write 用 ON CONFLICT (idempotency_key) WHERE idempotency_key <> ''，
-- 缺少同谓词的部分唯一索引时 PostgreSQL 直接报 no unique or exclusion constraint。
CREATE UNIQUE INDEX idx_artifact_refs_idempotency
    ON artifact_refs (idempotency_key) WHERE idempotency_key <> '';
CREATE INDEX idx_artifact_refs_updated ON artifact_refs (updated_at, id);
CREATE INDEX idx_artifact_refs_agent_run ON artifact_refs (agent_run_id);
CREATE INDEX idx_artifact_refs_workflow ON artifact_refs (workflow_id);
CREATE INDEX idx_artifact_refs_type_status ON artifact_refs (artifact_type, status);

-- ── 4. agent_artifacts ───────────────────────────────────────────────────
CREATE TABLE agent_artifacts (
    id                 BIGSERIAL PRIMARY KEY,
    run_id             BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    node_run_id        BIGINT,
    owner_user_id      BIGINT,
    workflow_id        BIGINT,
    generation_task_id BIGINT,
    tool_name          TEXT NOT NULL DEFAULT '',
    artifact_type      TEXT NOT NULL,
    artifact_ref_id    BIGINT REFERENCES artifact_refs(id) ON DELETE SET NULL,
    storage_key        TEXT NOT NULL DEFAULT '',
    url                TEXT NOT NULL DEFAULT '',
    name               TEXT NOT NULL DEFAULT '',
    mime_type          TEXT NOT NULL DEFAULT '',
    size_bytes         BIGINT NOT NULL DEFAULT 0,
    storage_object_id  TEXT NOT NULL DEFAULT '',
    content_hash       TEXT NOT NULL DEFAULT '',
    status             TEXT NOT NULL DEFAULT 'active',
    metadata_json      TEXT NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_artifacts_run ON agent_artifacts (run_id);
CREATE INDEX idx_agent_artifacts_ref ON agent_artifacts (artifact_ref_id);

-- ── 5. agent_activity_logs ───────────────────────────────────────────────
CREATE TABLE agent_activity_logs (
    id            BIGSERIAL PRIMARY KEY,
    run_id        BIGINT REFERENCES agent_runs(id) ON DELETE CASCADE,
    parent_run_id BIGINT,
    agent_id      BIGINT,
    node_id       TEXT NOT NULL DEFAULT '',
    log_path      TEXT NOT NULL DEFAULT '',
    content       TEXT NOT NULL DEFAULT '',
    summary       TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_activity_logs_run ON agent_activity_logs (run_id);
CREATE INDEX idx_agent_activity_logs_parent ON agent_activity_logs (parent_run_id);

-- ── 6. agent_tool_approvals ──────────────────────────────────────────────
CREATE TABLE agent_tool_approvals (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    session_id  BIGINT,
    agent_id    BIGINT,
    run_id      BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    tool_name   TEXT NOT NULL DEFAULT '',
    args_hash   TEXT NOT NULL DEFAULT '',
    token_hash  TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- token 是审批凭据：GetByTokenHash 必须命中唯一一行。
CREATE UNIQUE INDEX idx_agent_tool_approvals_token ON agent_tool_approvals (token_hash);
CREATE INDEX idx_agent_tool_approvals_owner ON agent_tool_approvals (user_id, status, expires_at);
CREATE INDEX idx_agent_tool_approvals_run ON agent_tool_approvals (run_id);
CREATE INDEX idx_agent_tool_approvals_session ON agent_tool_approvals (session_id);
CREATE INDEX idx_agent_tool_approvals_updated ON agent_tool_approvals (updated_at);

-- ── 7. agent_mcp_servers ─────────────────────────────────────────────────
CREATE TABLE agent_mcp_servers (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL,
    name              TEXT NOT NULL DEFAULT '',
    server_url        TEXT NOT NULL DEFAULT '',
    auth_header       TEXT,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    quota_policy_json TEXT NOT NULL DEFAULT '{}',
    tools_json        TEXT NOT NULL DEFAULT '[]',
    last_test_status  TEXT NOT NULL DEFAULT '',
    last_test_error   TEXT NOT NULL DEFAULT '',
    last_test_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_mcp_servers_owner ON agent_mcp_servers (user_id, updated_at DESC, id DESC);
CREATE INDEX idx_agent_mcp_servers_enabled ON agent_mcp_servers (user_id, enabled);

-- ── 8. agent_mcp_tool_cache ──────────────────────────────────────────────
CREATE TABLE agent_mcp_tool_cache (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT,
    server_name      TEXT NOT NULL,
    server_url       TEXT NOT NULL,
    auth_fingerprint TEXT NOT NULL,
    tools_json       TEXT NOT NULL DEFAULT '[]',
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- NULLS NOT DISTINCT（PG15+）：默认唯一索引把 NULL 视为互不相等，会让「无用户」缓存
-- 每次都 INSERT 新行而 ON CONFLICT 永不触发，缓存无法收敛。
CREATE UNIQUE INDEX idx_agent_mcp_tool_cache_key
    ON agent_mcp_tool_cache (user_id, server_name, server_url, auth_fingerprint) NULLS NOT DISTINCT;
CREATE INDEX idx_agent_mcp_tool_cache_expires ON agent_mcp_tool_cache (expires_at);

-- ── 9. user_agent_preferences ────────────────────────────────────────────
CREATE TABLE user_agent_preferences (
    user_id          BIGINT PRIMARY KEY,
    default_agent_id BIGINT REFERENCES agent_definitions(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── 10. role_agent_access / role_model_access ────────────────────────────
CREATE TABLE role_agent_access (
    role_id                TEXT PRIMARY KEY,
    role_name              TEXT NOT NULL DEFAULT '',
    allowed_agent_ids_json TEXT NOT NULL DEFAULT '[]',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE role_model_access (
    role_id                TEXT PRIMARY KEY,
    role_name              TEXT NOT NULL DEFAULT '',
    allowed_model_ids_json TEXT NOT NULL DEFAULT '[]',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
