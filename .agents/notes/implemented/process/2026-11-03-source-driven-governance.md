# Agent Note: 用源码与风险驱动替代历史阶段冻结

Status: implemented

Task: #15
Scope: AGENTS / docs / 项目技能 / DSH 提示词 / 治理检查器 / PR #110

## Problem

旧规范、技能、角色提示词和检查器分别要求 R1 唯一入口、固定七域和实施顺序、每项任务登记 INK/backlog、禁止测试以及设计哈希冻结。这些机制会让局部源码修复服从历史规划；与此同时，真实的 API/SDK/组合根接线缺口无法由设计完成度证明。此前 PR #110 已整理部分文档，重复另开搬迁分支会制造冲突。

## Decision

沿用 #110 的独立文档分支与原提交，追加明确范围的治理改造。当前事实由源码、组合根与可复现结果确定；重要安全、事务、兼容与历史追溯约束保留。当前任务由 GitHub 承载，历史设计/规划进入 reference。三个项目技能和 DSH 提示词同步新规则，避免 AGENTS 改了而实际代理仍按旧顺序执行。

archguard 允许有价值的测试，继续拒绝跨基础设施、平台反向依赖、超大源码和语法错误；语义checker退役设计hash与固定FR门禁，保留真实契约关系验证。普通CI按仓库真实迁移记录核对，权限覆盖的17版本保护仍保留，新增授权需安全复核。Go业务改动仅注释及导入别名，界面只改阶段用语。完整证据见[源码基线](../../reference/audits/2026-11-03-source-baseline.md)。

## Alternatives considered

只改 README/AGENTS 会留下 skills、notes校验和DSH里的实际限制，未采用。全仓字符串替换会破坏迁移、缓存键、模型名和兼容入口，未采用。删除整套checker会丢权限、CAS和引用检查，未采用。为本轮同时修Agent/编辑器/预算业务会超出低风险授权，并使治理PR无法独立审查，未采用。

## Consequences

规则不再保护历史阶段；已有设计仍可追溯，机器资产保持路径。历史ADR Accepted只证明当时选择，不永久冻结架构。当前隔离检查会如实暴露语义缺陷；契约checker仍有幂等声明与5个错误码同步失败，没有重算hash或关闭检查掩盖。

本记录的implemented仅指工作分支内治理改造，不代表PR已合并、业务闭环已完成或GitHub任务应关闭。后续竞品、里程碑与Issue写入遵循用户的分别确认要求。

## Verification

Go build、vet、gofmt、archguard、i18n和Creator/Admin构建通过。28项隔离正反例证明有效边界仍拒绝违规，历史封印仍防正文篡改和重算覆盖。生成器隔离比较确认API语义不变、仅元数据和旧FR报告有预期差异；18个Go业务文件token等价。Markdown相对路径检查修复1处SDK链接。

最终Skills/notes、链接、迁移与机器资产差异、CI结果以及无法运行的PowerShell/DSH/真实模型/浏览器范围集中记录在总报告及PR，不以本笔记代替运行证据。
