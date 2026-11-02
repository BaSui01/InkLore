# Agent Note: PR 安全自动合并的判定、撤销与并发边界

Status: implemented

Task: INK-254
Scope: .github/workflows/safe-auto-merge.yml、PR 标签、分支门禁与仓库治理

## Problem

原自动合并器只排除少量敏感路径，安全文档 PR #24 曾自动合入 dev，说明目录黑名单不足以代表业务验收。它还可能因收到过期 synchronize 事件而撤销新 head 合法的 auto-merge；转回 Draft、变更目标分支以及重命名原受保护文件时，旧事件范围和检查不完整。按 AGENTS 工程记忆要求，应保留权限、并发、拒绝策略及验证边界。

## Decision

仅在 GitHub 默认分支的可信工作流中运行 pull_request_target，不 checkout 或执行 PR 提供的脚本。GitHub API 先读取最新 PR 状态，扫描全部变更文件（包括 rename 的 previous_filename），再次核对 head；旧 head 事件直接忽略，不撤销新 head 的决策。每个 PR 串行处理以减少竞态。

仅同仓库、目标 dev、非 Draft、未标记 manual-review / do-not-merge 的开放 PR 可进入自动合并判定；私有主分支 private 的 PR 坚持人工审核。安全、密钥、身份、持久化、迁移、核心架构与工作流路径永不自动启用；超过 20 个文件也人工处理。只有明确列出的普通文档可直接启用，其余非敏感源码须由维护者添加 auto-merge-approved 标签。标签和 PR 状态取 API 最新值，不只信任事件快照；转换 Draft、改 base 或加阻止标签时撤销现有请求。GitHub 负责最终三项 CI、合并冲突及 Review 对话门禁。

启用/撤销 auto-merge 需要 github-script 的 contents:write 与 pull-requests:write；此权限仅用于固定可信脚本内的 GitHub API，绝不运行源分支代码。对 PR 来自 fork 或目标非 dev 的情况 fail-closed。

## Alternatives considered

- 所有 CI 全绿就自动合并：无法区分业务验证与敏感路径，拒绝。
- 只维护敏感路径黑名单：目录名称和重命名会遗漏风险，改为低风险文档白名单加受控 opt-in。
- 过期事件一律撤销：可能抵消新的合法决定，改为陈旧 head 无操作并按 PR 串行。
- 给机器人绕过保护规则的权限：不采用；必须尊重 GitHub 分支保护与最新检查。

## Consequences

普通文档 PR 可省人工合并点击；源码 PR 默认不会自行合入，必须维护者明确加标签。敏感变更无论添加标签都需人工合并。GitHub 原生 delete_branch_on_merge 清理短期来源分支，不能用来自动删除尚未合并的旧分支。触发器运行在 private 默认分支，故仅更新 dev 中同名工作流不代表全局策略已部署。

## Verification

2026-11-03 对真实 GitHub #22 与默认分支的工作流差异、未解决的审查线程以及 #24 的 auto_merge 记录进行了独立核对；确认旧逻辑存在缺少状态撤销/旧 head 事件竞态和安全文档覆盖不足。新工作流静态覆盖 converted_to_draft / edited 事件、实时标签、head 二次核验、rename 原路径、文档白名单、权限与 PR 串行限制。

后续仍需真实事件验证：标签新增与撤销、Draft 切换、旧 head 后到、敏感文件改名、code opt-in 和 CI 自动合并；仅 workflow/Go 编译或 YAML 语法通过不能替代这些行为验收。不得用 admin 绕过分支保护完成本任务。
