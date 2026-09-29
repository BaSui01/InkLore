-- 000010：制品正文存储表（INK-047，ADR-0008 的落地）
--
-- 背景：artifactstore 的 Store 端口此前只有 MongoStore（已删除，见 INK-046）与 MemoryStore。
-- 本表承载 PostgresStore 的制品正文：JSONB 落 payload、content_hash 供去重、
-- storage_object_id 与 artifact_refs.storage_object_id 对应。
--
-- 体积策略（ADR-0008）：单条制品超过实现层阈值时必须明确报错，不做静默截断；
-- 更大的二进制留给后续对象存储任务，不在此内联。
-- 时间列统一 TIMESTAMPTZ（AGENTS.md 北京时间契约）。

CREATE TABLE artifact_documents (
    id                    TEXT PRIMARY KEY,
    artifact_type         TEXT NOT NULL,
    source                TEXT NOT NULL DEFAULT '',
    content_hash          TEXT NOT NULL,
    backend               TEXT NOT NULL DEFAULT 'postgres',
    payload               JSONB,
    payload_json          TEXT NOT NULL DEFAULT '',
    text_content          TEXT NOT NULL DEFAULT '',
    metadata              JSONB NOT NULL DEFAULT '{}',
    owner_user_id         BIGINT,
    agent_run_id          BIGINT,
    agent_node_run_id     BIGINT,
    workflow_id           BIGINT,
    task_id               BIGINT,
    artifact_ref_id       BIGINT,
    payload_size_bytes    BIGINT NOT NULL DEFAULT 0,
    retention_expires_at  TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifact_documents_hash ON artifact_documents (content_hash);
CREATE INDEX idx_artifact_documents_run ON artifact_documents (agent_run_id);
CREATE INDEX idx_artifact_documents_type_created ON artifact_documents (artifact_type, created_at DESC);
