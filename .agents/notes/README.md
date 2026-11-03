# InkLore 工程记忆

本目录保存值得长期保留的根因、取舍、兼容理由和失败路线。当前工程要求见 [AGENTS](../../AGENTS.md)，写作流程见 [决策技能](../skills/inklore-decisions/SKILL.md)。历史设计与旧 ADR 不自动成为当前开发约束。

## 职责

| 位置 | 职责 |
| --- | --- |
| [GitHub Issues](https://github.com/BaSui01/InkLore/issues)、[Milestones](https://github.com/BaSui01/InkLore/milestones)、PR | 当前任务、依赖、实现与验收状态 |
| [docs](../../docs/README.md) | 面向开发者、维护者或用户的当前说明 |
| 生命周期笔记 | 有长期价值的工程因果与验证 |
| reference/ | 历史设计、ADR、旧规划、原始验收与截图 |

纯格式、错别字、明显的机械修复无需额外笔记。已有活跃 note 能承接事实就更新；重要决定被替代时说明原因与范围，保留旧记录的独特证据。Accepted ADR 的历史状态不禁止根据当前需求与证据作出新选择。

## 生命周期与路径

沿用 `lifecycle/class/YYYY-MM-DD-kebab-title.md`。日期为首次提出日（北京时间）；同一 class 与文件名跨生命周期只保留一份。分类为 feature、bug-fix、simplification、architecture、process、testing，按实际内容选择，不预建空目录或全量笔记索引。

| lifecycle | 含义 |
| --- | --- |
| proposed | 尚未落地的方案与判据；不新增固定审批步骤 |
| implemented | 决定已在明确范围落地，具有相应证据 |
| rejected | 已否决且有保留价值的方案，写清原因与重新考虑的前提 |
| archived | 已退役的 implemented 历史，经归档工具移动和封印 |

笔记生命周期只描述记录，不能据此把 GitHub 任务标记为完成。

## 笔记格式

前六行使用以下结构：

~~~text
# Agent Note: 具体主题

Status: implemented

Task: #15
Scope: 具体机制或代码路径
~~~

Task 可写真实的 `#编号`、`https://github.com/BaSui01/InkLore/issues/编号`、`https://github.com/BaSui01/InkLore/pull/编号`，或用于追溯的 `INK-三位及以上数字`。无需在旧 backlog 创建同名行；离线检查器只验证格式，作者须核对真实关联。

rejected 状态行为 `Status: rejected — 具体原因`。必需二级标题如下，正文写实际内容而非占位符：

| 生命周期 | 必需二级标题 |
| --- | --- |
| proposed / rejected | Problem、Proposal、Alternatives considered、Acceptance criteria、Risks |
| implemented / archived | Problem、Decision、Alternatives considered、Consequences、Verification |

正文中文，固定节名用于现有检查器。验证包含真实命令、结果与未验证项；implemented/archived 不保留 Proposal、Plan、Migration plan、Acceptance criteria、Risks 等未落地计划节。生命周期笔记不超过 50,000 bytes，不写 Emoji、秘密或作者私有正文。

模板：[proposed](../skills/inklore-decisions/templates/note-proposed.md)、[implemented](../skills/inklore-decisions/templates/note-implemented.md)、[rejected](../skills/inklore-decisions/templates/note-rejected.md)。复制后替换任务、范围和内容，不能把模板当交付证据。

## 历史参考资料

reference/ 接受 Markdown、JSON 与 PNG，保留原有证据格式，不套用生命周期模板和 50KB 限制。仍只允许普通文件与目录，拒绝符号链接及不支持的文件类型；截图不按 UTF-8 文本解码。

[历史设计](reference/system-design/)、[旧 ADR](reference/decisions/README.md)、[旧 backlog](reference/planning/backlog.md)、[旧架构记录](reference/implementation/architecture-history.md)仅供追溯。当前机器契约仍在 [docs/system-design](../../docs/system-design/00-INDEX.md)，当前状态需核对源码和实际运行。

## 检索、校验与归档

按机制、符号、错误码检索相关活跃/否决笔记，包含隐藏目录；需要追溯时再读取 archived/reference。只更新本次受影响的内容和入站链接。

~~~sh
node scripts/agent-notes.mjs check
node scripts/agent-notes.mjs check --base <变更前真实提交>
node scripts/agent-notes.mjs archive .agents/notes/implemented/process/YYYY-MM-DD-topic.md
~~~

[归档工具](../../scripts/agent-notes.mjs)只处理 implemented 笔记，拒绝已有目标和残留入站引用。先把活跃引用转向继任记录；若引用来自已封印历史，不修改旧封印来解除阻塞。归档成功后再引用最终 archived 路径。

archive-manifest.json 使用 LF 规范化 SHA-256。普通 check 验证工作树内容和封印；`--base` 对照真实 Git 基线，拒绝历史条目删除或重算覆盖。CI 使用 PR base 或 push 前提交。活跃设计不使用这一历史封印机制。

归档期间不手动编辑源文件或 manifest。已有 `.agents/notes-archive.lock` 时先核对活跃进程和恢复副本，不自动抢占；异常恢复要保留证据，未成功前不删除源记录。

机械检查不能判断理由是否正确、备选是否真实、任务是否完成或本次是否需要笔记。它不会在线核验外部 URL/fragment，也不授权部署、合并、修改生产数据或变更用户设置。
