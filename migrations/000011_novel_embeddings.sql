-- 000011：作品语义索引（INK-017）
--
-- 背景：M5 要求「历史章节/人物记忆向量化入库，相似片段可被生成流程调用」。
-- 采用单表承载所有语义片段：章节与人物共用同一索引，召回时按 kind 区分，
-- 避免给每张业务表都加一列 embedding（novel_characters.embedding 保留但不再使用）。
--
-- 维度固定 1536：与 OpenAI text-embedding-3-small 及既有 novel_characters.embedding 一致。
-- 相似度使用余弦距离（<=>），索引用 HNSW（pgvector 0.8.6 支持）。
-- 时间列统一 TIMESTAMPTZ（AGENTS.md 北京时间契约）。

CREATE TABLE novel_embeddings (
    id          UUID PRIMARY KEY,
    novel_id    UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    kind        TEXT NOT NULL,
    ref_id      TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    embedding   vector(1536) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, novel_id, kind, ref_id)
);

CREATE INDEX idx_novel_embeddings_scope ON novel_embeddings (tenant_id, novel_id, kind);
CREATE INDEX idx_novel_embeddings_vec ON novel_embeddings USING hnsw (embedding vector_cosine_ops);
