-- 000016：连载大纲体系（INK-068）
--
-- 背景：卷（000014）给了长篇"分卷"的骨架，但"这一卷要推进什么、这一章写什么、
-- 埋了哪些伏笔计划在哪一章回收"仍然无处落库——AI 生成的大纲只能停留在聊天记录里，
-- 下一次生成又要重贴一遍。
--
-- 节点类型（node_type）：
--   mainline      主线（全书级，chapter_id 为空）
--   sideline      支线（全书级或挂到某一卷）
--   chapter_brief 章节细纲（挂到具体章节）
--   beat          场景节拍（挂到具体章节，是细纲的执行粒度）
--
-- 与 fact_ledger 的分工：这里存"**计划**"（打算写什么、打算哪一章回收伏笔），
-- fact_ledger 存"**已发生**"（章节确认后入账的事实）。二者不互相替代：
-- 计划可以改，事实只能追加或失效（ADR-0007）。
--
-- 层级用 parent_id 自引用表达（主线 → 支线 → 细纲），不用固定深度：
-- 网文的大纲层级因人而异，硬编码层数会逼作者改结构去适应数据库。

CREATE TABLE novel_outline_nodes (
    id UUID PRIMARY KEY,
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    parent_id UUID REFERENCES novel_outline_nodes(id) ON DELETE CASCADE,
    volume_id UUID REFERENCES novel_volumes(id) ON DELETE SET NULL,
    chapter_id UUID REFERENCES novel_chapters(id) ON DELETE CASCADE,
    node_type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'planned',
    -- 线索类节点的计划回收章节号（全书序号）；0 表示不适用或尚未规划。
    planned_resolve_chapter INTEGER NOT NULL DEFAULT 0,
    ordinal INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_novel_outline_nodes_novel
    ON novel_outline_nodes (tenant_id, novel_id, node_type, ordinal);
CREATE INDEX idx_novel_outline_nodes_chapter
    ON novel_outline_nodes (tenant_id, chapter_id, ordinal);
CREATE INDEX idx_novel_outline_nodes_parent
    ON novel_outline_nodes (tenant_id, parent_id, ordinal);

-- +migrate Down
DROP TABLE IF EXISTS novel_outline_nodes;
