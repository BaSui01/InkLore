-- 000017：章节与分卷摘要（INK-070）
--
-- 背景：Context Pack 的「前章摘要」此前直接用章节的 outline（brief）字段，那是作者写
-- 的一句话纲要，不是对**正文**的压缩。章节一多，生成第 N 章时模型拿不到「前面到底发生了什么」，
-- 这正是「写到第五章就忘第一章」的根因。
--
-- 设计：
--   * scope + ref_id 一张表承载两种粒度：chapter（ref_id = 章节 ID）与 volume（ref_id = 卷 ID）；
--   * source 记录摘要来源：llm（模型生成）或 extractive（本地确定性抽取回退）。
--     必须可分辨——不然作者会以为抽出来的前三句是模型写的；
--   * 唯一键含 tenant_id，与其它作品域表一致（ADR-0012：数据全部落库）；
--   * 只保留最新一份（Upsert）：摘要的中间版本对作者没有价值，章节正文的历史由
--     novel_chapter_versions 承担（INK-069）。

CREATE TABLE novel_summaries (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    scope TEXT NOT NULL,
    ref_id UUID NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'extractive',
    model TEXT NOT NULL DEFAULT '',
    source_word_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, scope, ref_id)
);

CREATE INDEX idx_novel_summaries_novel ON novel_summaries (tenant_id, novel_id, scope);

-- +migrate Down
DROP TABLE IF EXISTS novel_summaries;
