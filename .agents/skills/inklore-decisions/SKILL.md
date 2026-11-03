---
name: inklore-decisions
description: 用于 InkLore 重要技术取舍、工程笔记维护、历史约束检索、ADR 替代和故障根因沉淀。按需记录理由与验证，保留历史证据，通过 GitHub 任务关联追溯。
---

# InkLore 决策与工程记忆

## 1. 先判断是否需要记录

读取 [AGENTS](../../../AGENTS.md) 与 [笔记规则](../../notes/README.md)，按当前任务选择记录位置：

| 内容 | 记录位置 |
| --- | --- |
| 用户目标、任务状态、依赖、验收与实施讨论 | GitHub Issue/Milestone/PR |
| 当前架构、部署与使用方式 | 受影响的长期 docs |
| 非显然根因、兼容妥协、重要取舍与失败路线 | 对应工程笔记 |
| 历史设计、旧 ADR、原始验收材料 | notes/reference；仅作追溯 |

纯格式、错别字、一眼能理解的机械修复无需额外 note 或 ADR。重要长期选择要有理由和影响说明；按影响决定是否形成正式决策记录，不把每个局部修改变成审批流程。

## 2. 核对历史与当前证据

按机制、符号、错误码检索相关源码、活跃笔记和 [历史 ADR](../../notes/reference/decisions/README.md)。包含隐藏目录，只有需要追溯时才展开 archived 或大批历史设计。

逐项判断历史假设是否仍成立，说明仍适用、部分被替代或已过时的范围。Accepted 状态证明曾作出该选择，不证明它永远正确或具有当前授权。

当前用户要求、实际风险与真实实现决定本次处理。已有授权覆盖时继续工作；缺少真实决策或涉及未授权的高风险操作时，提出有证据的具体问题。

## 3. 写最少但有用的笔记

按 [笔记规则](../../notes/README.md) 选择现有生命周期和分类；Task 关联真实 GitHub Issue/PR URL 或 `#编号`，历史 INK 仅作为兼容引用。Scope 写具体机制或路径。

- 尚未落地：使用 [proposed 模板](templates/note-proposed.md)。
- 已在明确范围落地：使用 [implemented 模板](templates/note-implemented.md)。
- 已否决且有防止重犯价值：使用 [rejected 模板](templates/note-rejected.md)。

写实际问题、采用方案、真正比较过的备选、代价和可复核验证。未执行的行为注明未验证；纯文档或静态工具交付不能写成业务运行通过。不要编造备选或批准，不复制临时日志、秘密和作者正文。

## 4. 更新与替代

1. 理由不变、事实有更新：修订对应活跃 note，避免重复记录。
2. 决定被替代：新记录写明原因和替代范围，保留旧记录的独特历史价值；不以旧 ADR 冻结当前实现。
3. proposed 已落地：按实际范围改为 implemented，保留首次日期，整理验证和后果，并修复入站链接。
4. 完全失去活跃价值的 implemented 笔记：检查引用后使用归档工具；部分替代的记录保持相关链接。

归档执行 `node scripts/agent-notes.mjs archive <implemented笔记路径>`。已封印 archived 正文与 manifest 既有条目不得重新计算覆盖；修正历史结论用新记录表达。

## 5. 验证与交付

修改笔记后运行 `node scripts/agent-notes.mjs check`；涉及归档时增加 `--base <变更前真实提交>`。修改技能或指南时运行 `node scripts/check-project-skills.mjs`。

机械检查验证格式、链接和历史封印，不证明任务真实存在、理由正确或业务完成。GitHub 保存任务状态，笔记生命周期只描述这条记录。

只同步受影响内容，按用户授权交付。需要了解既有资料来源与采用范围时阅读 [来源说明](references/source-adaptation.md)，不把上游流程整体引入项目。
