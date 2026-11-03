# Agent Note: GitHub CI 状态只读摘要的运行边界

Status: proposed

Task: INK-253
Scope: .github/workflows/ci-status-overview.yml、docs/github-platform.md、GitHub Projects 原生自动收录

## Problem

GitHub 的 Actions 原生日志展示单次执行，却不适合快速核对 dev/private 分支最近的 CI 与安全扫描状态。Projects 的 Gate 字段是人工治理记录，不能冒充 GitHub 的实时检查结果。直接用广权限 Token 自动修改项目和 PR，会增加供应链和不可信贡献的攻击面。

## Proposal

增加默认分支可信的只读 CI 状态摘要工作流，允许手动、周期性以及 CI/Isolated Integration 完成后运行。只调用 GitHub 官方 REST API 读取仓库分支头、最多最近 100 个 Actions 运行和开放 PR 元数据，汇总至当前 Job Summary。所有来源只作文字或受限 URL 展示；当运行不属于当前分支 SHA、查询窗口无法覆盖、或 PR 使用合成 merge SHA 时，标记 stale / not observed，不声称通过分支保护。工作流没有 checkout、执行 PR 源码、远端 shell、Secret 访问、Projects 写权限或部署副作用。

Projects 工作项自动收录仅使用维护者通过 GitHub 原生 Auto-add 设置的功能，不往 GitHub Actions 中存放全局 Projects PAT。此配置进入 dev 后只是候选，GitHub 默认分支仍为 private；schedule 和 workflow_run 需审阅并经受保护发布进入默认分支才生效。

## Alternatives considered

- 使用 workflow_run 配合 contents:write 或 Projects PAT 自动改看板状态：无必要的高权限，不采用。
- 检出 PR head 再执行分析脚本：会把非受信任代码带入默认分支上下文，拒绝。
- 将找不到最新检查的 PR 视为通过：GitHub 可能使用合成 merge SHA，容易误报，拒绝。

## Acceptance criteria

- 工作流只有 actions/contents/pull-requests read，未检出来源代码，不含令牌和环境写操作。
- actionlint、脚本语法、项目 Skills/notes 通过；PR 最新 head 三项必需 CI 通过。
- 合入 dev 以后不宣布定时任务已激活；正式进入默认 private 后手动运行并审阅 Job Summary 的 stale/not observed 分类。
- GitHub Projects Auto-add 由项目维护者在 UI 启用并验证，不把未设置的自动录入写成已经生效。

## Risks

当 GitHub Actions 运行超过 100 条、PR head 与合成 merge commit 不一致、GitHub 的 workflow 名称不匹配时，摘要可能显示 Not observed；必须点击实际 PR Checks 继续确认。该摘要不替代安全扫描、业务验收或状态门禁；workflow_run 由外部贡献触发也不能扩大只读权限。默认分支发布仍需独立安全审阅。