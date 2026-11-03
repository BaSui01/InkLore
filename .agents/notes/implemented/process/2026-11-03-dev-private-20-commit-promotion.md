# Agent Note: dev 每 20 个提交受保护地同步 private

Status: implemented

Task: INK-256
Scope: .github/workflows/dev-promotion.yml、GitHub 分支保护与 dev/private 发布流程

## Problem

仓库采用 private 为稳定主分支、dev 为持续开发分支。靠人手定期复制或强推容易丢失双方历史；直接用 dev 作为 PR 源并启用仓库合并后自动删分支，又容易让长期开发分支误被删除。原安全自动合并工作流只针对目标 dev 的日常 PR，不负责开发分支到稳定分支的批量升级。

2026-11-03 实际比较 private...dev 为 dev 超前 42 个提交，同时 private 还有 6 个 dev 不包含的提交。合并存在分叉和安全敏感源码，不能把“达到 20 个提交”误读为已通过发布验收。

## Decision

在 GitHub 默认分支 private 上配置可信 Action；dev 的 CI push 成功完成后触发评估，每小时定时补偿，另允许维护者手动运行。通过 GitHub Compare API 的 ahead_by 判断是否达到 20 个提交，不用时间、文件数或两个分支的简单 SHA 比较代替。低于阈值不变更任何引用。

达到阈值时创建 release/dev-<dev commit prefix> **临时快照分支**，从该分支向 private 创建 PR，而不将 dev 直接作为源。每次仅允许一个未完结发布 PR；定时重复触发不可产生重复发布。保持 dev 原有引用与开发活动，GitHub 合并后删除 release 临时来源分支。

发布 PR 必须使用 **merge commit**，不可 squash/rebase：保留 dev 提交祖先以使下一轮 private...dev 的 ahead_by 重新计数。只有 stable 未包含在 dev 之外的额外提交为零、差异不包含 Secret/权限/迁移/架构治理/Agent 与 LLM 底层等敏感路径、最新 dev commit 三项必要 CI 均通过且 Compare 未截断时，才尝试启用原生 auto-merge；GitHub 必须继续验证冲突、最新 CI 与 Review 对话。任何条件不满足，只创建 PR 并打 manual-review 标签等待人工处理，不绕过任何保护规则。

工作流只用受信任的默认分支 github-script 和 GitHub API，不 checkout 或运行不可信 PR 分支代码，不读取数据库/生产密钥。GITHUB_TOKEN 创建的 PR 不保证触发 pull_request workflow，因此不把 PR 创建当作 CI 已验证；若 GitHub 要求 PR 独立检查而尚未具备，就保持阻塞。

## Alternatives considered

- 直接向 private push/force-push：会绕过保护与代码评审，还可能覆盖分叉，拒绝。
- 直接用 dev 做发布 PR 源并靠自动删除：有长期分支删除风险，不采用。
- 每到 20 个提交 squash 合并：虽然文件会同步，但 dev 祖先没有进入 private，ahead_by 不会归零，拒绝。
- 无论敏感文件和分叉都自动合并：会提前发布未验证的 Agent/Secret/迁移变更，采用条件审查代替。
- 用外部 PAT/长期令牌生成 PR：权限与 Secret 管理负担更大，优先使用 GITHUB_TOKEN 并保持分支保护，受限事件行为明确留证。

## Consequences

正常开发继续在 dev；每累积 20 个尚未进入 private 的提交自动准备发布 PR，可安全合并时由 GitHub 自动合并，不能安全合并时保留可审计的人工 Review 阻塞。长期 dev 不会因自动清理被删除。存在分叉时需先用受保护 PR 将 private 新提交同步到 dev 或解决冲突；这属于额外验收，不在后台强制处理。该流程不打发布 tag、不部署服务、不修改数据库。

## Verification

2026-11-03 已核实真实 GitHub 比较 private...dev：ahead_by=42、behind_by=6；默认分支为 private，dev 已受保护，必需 CI 三项和禁止强推/删除策略存在。新工作流使用 Node 语法检查和 actionlint 静态检查，均退出 0；检查触发器为 dev 成功 CI、定时、人工触发，Merge Commit 和 release 快照策略已在源码核对。

在最终工作流合并到 private 前不得声称自动发布已上线。合入后还须人工触发并核对：仅当 ahead_by>=20 才建 PR；重复运行不重复创建；当前 42/6 分叉加敏感变更时给发布 PR 标记 manual-review 且不合并；确实安全时自动启用、合并及临时分支删除，并回读 dev 仍存在。以上运行行为未完成前 INK-256 保持 review。
