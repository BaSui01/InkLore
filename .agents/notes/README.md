# InkLore 工程记忆

本目录保存代码无法表达的“为什么”：不显然的行为、兼容妥协、失败方案、隐含约束与故障根因。规则入口是 [AGENTS](../../AGENTS.md)，写作流程由 [决策技能](../skills/inklore-decisions/SKILL.md) 执行。笔记不能覆盖 AGENTS、已接受 ADR 或用户明确指令。

## 职责分工

| 资产 | 回答的问题 | 不承担的职责 |
| --- | --- | --- |
| [ADR](reference/decisions/README.md) | 正式架构裁决、全局契约与替代关系 | 不收集每个局部实现细节 |
| 本目录 notes | 为什么采用此实现、放弃什么、什么条件下可以重新考虑 | 不定义第二套架构权威、不管理任务进度 |
| [backlog](../../docs/backlog.md) | 谁在做、是否验收、具体阻塞 | 不承载全部长期因果 |
| [设计与现状入口](../../docs/development-guide.md) | 目标行为、实际接线与操作规范 | 不重复笔记中的讨论历史 |

架构变更先写 ADR；有值得保留的实现细节再写 note 并链接 ADR。纯格式、错别字、无行为样式修正和一眼能看懂的局部机械修复无需 note。行为、跨文件契约、持久化/配置格式、非显然兼容选择、维护工具或故障根因有长期价值时必须新增或更新相应 note。已有 note 能完整承接事实变化就更新，不为数量写笔记。

## 目录与身份

路径固定为 lifecycle/class/YYYY-MM-DD-kebab-title.md。日期为首次提出日（北京时间），后续移动不改；同一 class + 文件名跨生命周期只保留一份。

| lifecycle | 含义 | 转换要求 |
| --- | --- | --- |
| proposed | 方案尚未落地 | 写验收与风险；已有授权覆盖的工作直接推进，不额外设置确认门 |
| implemented | 决策已在对应范围落地并有证据 | 改为现在时 Decision，补实际 Verification；不能把设计文档交付说成业务实现 |
| rejected | 不采用但有防止重犯价值的提案 | Status 行写原因，保留 Proposal 与真实备选；不归档为 implemented |
| archived | 已被替代或失去活跃价值的 implemented 历史 | 先处理活跃引用，再通过归档工具移动和封印；正文不再改写 |

分类固定六个：feature（用户可见能力）、bug-fix（缺陷和复盘）、simplification（能力不增的收敛）、architecture（实现拓扑，正式裁决仍在 ADR）、process（工具/协作/发布）、testing（验证策略与证据，不允许项目禁止的测试文件）。只创建实际使用的目录，不维护全量 INDEX.md 笔记总表。README 仅说明规则，不手工列每篇笔记。

## 笔记格式

前三行固定标题、空行、状态，第 4 行空行，第 5 行关联任务，第 6 行作用范围：

~~~text
# Agent Note: 清楚描述这条工程约束

Status: implemented

Task: INK-143
Scope: scripts/agent-notes.mjs 与工程记忆工作流
~~~

rejected 的状态写为 Status: rejected — 具体原因。Task 必须对应 backlog 中真实任务行；Scope 写机制或路径，避免“所有东西”。所有必需二级标题只能出现一次，正文不能为空或只有占位符。

| 生命周期 | 必需二级标题 |
| --- | --- |
| proposed / rejected | Problem、Proposal、Alternatives considered、Acceptance criteria、Risks |
| implemented / archived | Problem、Decision、Alternatives considered、Consequences、Verification |

正文中文，固定标题英文以便机械校验。只记录真实考虑过的备选；验证写清命令、预期、实际结果和缺口，秘密与私有正文脱敏。implemented/archived 不保留 Proposal、Plan、Migration plan、Acceptance criteria、Risks 等提案标题。笔记不超过 50,000 bytes，不使用 Emoji。

模板：[提案](../skills/inklore-decisions/templates/note-proposed.md)、[已实施](../skills/inklore-decisions/templates/note-implemented.md)、[否决](../skills/inklore-decisions/templates/note-rejected.md)。模板不直接放到生命周期目录，复制后必须填实。

## 改前检索与改后同步

1. 读取目标机制的 ADR，沿代码入口或模块文档中的 Note 链接下钻。
2. 按分类检索 proposed、implemented、rejected；隐藏目录必须纳入搜索。终端可用 rg --hidden，代理也可用支持隐藏文件的搜索工具；默认不搜索 archived。
3. 将命中项判为无关、仍适用、部分重叠或将被替代。新笔记保留相关链接和检索结论；无需建第二份审计看板。
4. 决定不变且只修路径、参数等事实：更新活跃 note。理由或决定翻转：新写 note，明确替代范围与旧理由；不要把旧 Decision 改成反面。Accepted ADR 正文仍不可修改。
5. proposed 落地时移动到 implemented，日期保持，更新状态与必需节；同时更新入站链接。部分替代的旧笔记保持活跃并互链，完全替代才考虑归档。
6. 同一交付内同步代码、笔记、相关 docs 和验收证据。Git 提交/发布依用户授权。后续会话从这些仓库文件恢复记忆，不依赖聊天记录或后台定时任务。

检索例子（从仓库根运行；关键词用真实机制替换）：

~~~sh
rg --hidden -n --glob '*.md' '机制名' .agents/notes/proposed .agents/notes/implemented .agents/notes/rejected
~~~

只给实际存在的目录传参；尚未创建的生命周期目录无需预建。本项目代理可直接对 .agents/notes 搜索并排除 archived 结果。

## 校验与归档

~~~sh
node scripts/agent-notes.mjs check
node scripts/agent-notes.mjs check --base HEAD
node scripts/agent-notes.mjs archive .agents/notes/implemented/process/YYYY-MM-DD-topic.md
~~~

[工具](../../scripts/agent-notes.mjs) 无第三方依赖。check 验证目录/分类/日期、头块、任务存在、必需节、大小、Emoji、本地 Markdown 文件链接及归档 manifest。外部链接和 fragment 不校验，文本语义由评审检查。

归档仅接受 implemented 笔记，先检查整树，再确保目标未占用且没有指向源文件的治理文档链接；若有则列出并拒绝，不批量改他人的文档。先将活跃引用引到继任笔记，再归档；归档后继任笔记可链接到最终 archived 路径。若入站引用来自已封印历史，保留被引用的活跃笔记，不改旧封印来解除阻塞。检查源链接在目标位置仍有效，不能在封印后修链接。归档会将 Status 改为 archived，写目标、登记 manifest、移除源；失败应恢复，未成功时不要手动删除源。

归档工具使用 `.agents/notes-archive.lock` 排他锁，避免多个代理同时覆盖 manifest。锁已存在时拒绝执行，不自动抢占；进程被强制终止后，先核对进程是否仍在运行、工具打印的恢复副本和源/目标/manifest，再由维护者处理残留锁。工具会为写入保留恢复副本，跨文件更新不是数据库事务；不能在归档期间手动编辑源与 manifest。

archive-manifest.json 记录每篇归档相对 notes 根的路径及 SHA-256，哈希前仅将 CRLF/CR 规范化为 LF。普通 check 发现正文与 manifest 不匹配或孤儿条目；check --base 对照指定 Git 基线，拒绝已存在封印的删除或改写。基线存在但尚无 manifest 时允许首次引入并明确报告 bootstrap；基线无效则失败。CI 使用 PR 的 base SHA 或 push 前的 SHA，不把用户输入拼进 shell 命令。

归档内容与封印一并提交；不得用“重新算哈希”修复已封印内容。修正旧结论时新建活跃 note 并链接历史。哈希机制保护归档不被无声改写，不是数字签名；有权修改 CI/受信基线者仍需仓库评审保护。活跃笔记不做哈希冻结。

## 检查边界

CI 机械检查已有笔记的结构和归档历史，不会可靠地从代码 diff 推断“这一改动是否非平凡”。是否遗漏因果、备选是否真实、证据是否充分仍是 skills 和评审的责任。也不自动归档 ADR、回填旧任务、执行生产操作或启动网页看板。

## 正式 ADR 参考资料

[正式 ADR 索引](reference/decisions/README.md) 保留原有编号、状态与裁决效力。reference/ 是独立的历史裁决资料，不是 proposed/implemented/archived 笔记，当前冻结设计与机读契约继续留在 [docs/system-design](../../docs/system-design/00-INDEX.md)。
