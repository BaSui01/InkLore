-- 000015：章节版本历史（INK-069）
--
-- 背景：novel_chapters 的正文是原地覆盖的，没有 version/revision 列。一次 AI 改写
-- 覆盖掉人工精修之后无法找回——这是写作工具最不可接受的体验，也是 M8「改坏能回滚」
-- 的前置。
--
-- 模型选择：**存新状态**（append-new-state）而不是存旧状态。
--   * 每次内容发生实质变更后，把变更后的完整状态追加为一条版本；
--   * 因此 revision N 恒等于「当时的最新正文」，当前正文 = 最大 revision；
--   * 回滚 = 把目标版本的正文写回章节 + 追加一条 source='rollback' 的新版本，
--     历史永不覆盖（回滚本身也可再回滚）。
--
-- 只对 title/brief/content 的变更打版本；status/ordinal/volume_id 这类结构字段
-- 不产生版本，避免拖动章节就刷出一堆无意义的版本记录。

CREATE TABLE novel_chapter_versions (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    chapter_id UUID NOT NULL REFERENCES novel_chapters(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    revision INTEGER NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    outline TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    word_count INTEGER NOT NULL DEFAULT 0,
    -- human | ai | tool | rollback：改动来源，用于区分「谁写坏了」
    source TEXT NOT NULL DEFAULT 'human',
    model TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chapter_id, revision)
);

CREATE INDEX idx_novel_chapter_versions_chapter
    ON novel_chapter_versions (tenant_id, chapter_id, revision DESC);

-- +migrate Down
DROP TABLE IF EXISTS novel_chapter_versions;
