-- 000014：作品域新增卷（volume）与场景（scene）结构层次（INK-067）
--
-- 背景：作品域此前只有 Novel -> Chapter 两级，长篇连载的最小结构单位
-- 「卷—章—场景」中的卷与场景无处落库：AI 生成的分卷目标、章节细纲、场景节拍
-- 只能停留在聊天记录里，下一次生成又要重贴一遍。
--
-- 设计约定：
--   * 两张新表都带 tenant_id（与 000002/000012 的隔离口径一致），仓储查询强制作用域；
--   * 章节的 order_index（列名 ordinal）仍是**全书连续序号**（第 1 章…第 30 章），
--     volume_id 只表达归属，因此不引入 (volume_id, ordinal) 复合唯一键，
--     否则跨卷拖拽章节会与既有 UNIQUE(novel_id, ordinal) 语义打架；
--   * 卷与场景的序号在各自父级内唯一（卷内按 novel 唯一，场景内按 chapter 唯一）。

CREATE TABLE novel_volumes (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    ordinal INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (novel_id, ordinal)
);

CREATE INDEX idx_novel_volumes_novel ON novel_volumes (tenant_id, novel_id, ordinal);

-- 章节归卷：可空（未分卷的散章仍然合法），删卷时章节保留并回到未分卷状态。
ALTER TABLE novel_chapters ADD COLUMN volume_id UUID REFERENCES novel_volumes(id) ON DELETE SET NULL;

CREATE INDEX idx_novel_chapters_volume ON novel_chapters (tenant_id, volume_id, ordinal);

CREATE TABLE novel_scenes (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    chapter_id UUID NOT NULL REFERENCES novel_chapters(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    title TEXT NOT NULL,
    brief TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'planned',
    word_count INTEGER NOT NULL DEFAULT 0,
    ordinal INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chapter_id, ordinal)
);

CREATE INDEX idx_novel_scenes_chapter ON novel_scenes (tenant_id, chapter_id, ordinal);

-- +migrate Down
DROP TABLE IF EXISTS novel_scenes;
DROP INDEX IF EXISTS idx_novel_chapters_volume;
ALTER TABLE novel_chapters DROP COLUMN IF EXISTS volume_id;
DROP TABLE IF EXISTS novel_volumes;
