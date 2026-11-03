---
name: inklore-decisions
description: 用于 InkLore 工程笔记维护、非平凡改动留因果、历史约束检索、ADR 编写与替代、事故根因沉淀。管理 .agents/notes 生命周期和归档校验，明确 ADR 裁决与 backlog 任务状态的独立职责。
---

# InkLore 决策与工程记忆

## 先判定记录落点

读 [AGENTS](../../../AGENTS.md)、[管理规约](../../../docs/project-management.md)、[ADR 索引](../../notes/reference/decisions/README.md) 与 [notes 规约](../../notes/README.md)。来源与适配见 [融合说明](references/source-adaptation.md)。用户已授权引入 notes；不沿用旧版“不建立 notes”的限制，不重复请求安装许可。

| 变更 | 记录方式 |
| --- | --- |
| 格式、错别字、无行为样式与机械修复 | 正常任务验收，无需为凑数量写 note 或 ADR |
| 分层、域边界、存储、协议、全局契约、外部依赖 | ADR-First；值得保留的实现细节另写 note 并链接 ADR，不复制裁决 |
| 非显然行为、跨文件契约、落盘/配置格式、兼容妥协、工具流程 | 新增或更新对应 note，保留为什么、真实备选、代价和验证 |
| 故障与失败路线 | 复盘根因沉淀 bug-fix；被否方案有防坑价值则保留 rejected |

notes 状态描述方案/笔记，不是任务状态；backlog 始终是唯一任务状态来源。笔记不能覆盖工程红线和正式架构裁决。

## 1. 改前检索

按机制名、域、符号、错误码搜索 decisions 与 notes 的 proposed / implemented / rejected。隐藏目录需包含在搜索内，默认排除 archived，历史追溯时才展开。沿代码入口注释和模块文档的 Note 链接阅读；没有必要不批量给源码加锚点。

每篇相关记录判为仍适用、部分重叠或将被替代，在新笔记写清检索结论和链接。没有搜索结果可以如实说明，不伪造“历史讨论”。事实先查清，架构冲突给出具体条款；用户已有授权覆盖的实现直接推进，只有缺少真实决策才询问。

## 2. 写 note

按 [notes 规约](../../notes/README.md) 选择生命周期与六分类，文件名保留首次提出日。只创建使用到的目录，不建立全量笔记 INDEX.md。Task 关联真实 INK，Scope 写真实机制或代码位置。

- 尚未落地：[proposed 模板](templates/note-proposed.md)，Problem、Proposal、Alternatives considered、Acceptance criteria、Risks。
- 已落地：[implemented 模板](templates/note-implemented.md)，Problem、Decision、Alternatives considered、Consequences、Verification。Decision 用现在时，明确落地范围；纯设计交付不能写成业务运行通过。
- 被否决：[rejected 模板](templates/note-rejected.md)，状态行写原因，保留原提案与判据，解释重新考虑需满足什么前提。

Problem 写具体触发与约束；Alternatives 只写真实权衡，先说对手方案优势再写未选原因；Consequences 同时写收益、成本和失效条件；Verification 写实际命令、结果、证据链接和未验证边界。正文中文、固定节名英文，使用相对 Markdown 链接，禁止凭据/私有正文/聊天流水账。

## 3. 同步与演进

1. 事实变动且决定理由不变：修订活跃 note 的路径/参数，不新增重复笔记。
2. 理由或决定翻转：新建 note 并明确替代范围，保留原有独特理由和真实备选，不能原地抹掉历史。
3. proposed 落地：移动至 implemented，日期不变，Status 改为 implemented，Proposal 改 Decision，验收与风险转化为 Verification/Consequences，删除计划节，更新入站链接。
4. 部分替代：双方保留活跃并互链。完全替代且无活跃价值：先检查入站引用，再用归档工具移动，不手动算新哈希覆盖旧封印。
5. 模板在技能目录，实际笔记只进入 notes 生命周期目录；不把空模板当已完成工程记忆。

归档执行 node scripts/agent-notes.mjs archive <implemented笔记路径>。命令会拒绝已有目标、无效树或残留入站链接；先把活跃引用改到继任笔记，成功后再从新笔记链接 archived 路径。archived 正文和 manifest 已有条目只读。修改历史判断需新建活跃笔记。

## 4. ADR 仍按正式流程

沿用 [ADR 模板](../../notes/reference/decisions/0000-template.md) 与现有编号/状态规则。新选择无真实裁决为 Proposed；不能替用户编造批准。Accepted 正文不可改写，后续变化新增 ADR 并明确部分或全部替代范围，按宿主规则更新替代说明。note 归档工具不操作 ADR。

## 5. 同批验收与交付

运行 node scripts/agent-notes.mjs check 和 node scripts/check-project-skills.mjs；涉及归档再用 check --base HEAD，CI使用真实变更前基线。普通 check 只校验工作树封印；基线校验才阻止正文与manifest一起重写历史。工具不自动判定代码是否非平凡、备选是否真实或是否漏写note，仍需语义评审。

实现、理由、docs 和验收证据同批交付，是否提交 Git 依用户授权。同步唯一 backlog，按 INK → 状态 → 变更 → 证据 → 风险汇报。
