# Agent Note: GitHub 协作视图与低权限周期性治理

Status: implemented

Task: INK-253
Scope: .github/dependabot.yml、.github/ISSUE_TEMPLATE、.github/workflows/repository-health.yml、docs/wiki

## Problem

仓库虽然已有 issues、CI 和启用的 Wiki，但没有统一的 Issue 入口、依赖版本更新调度、可审计的只读 GitHub 活动摘要以及实际 Wiki 页面来源。工程事实只允许由 backlog、design 与代码验收记录承担，单独的协作面板不能重新定义任务真相。

## Decision

沿用 dev 开发、private 稳定发布的仓库治理模型。Issues 表单采集可复现证据，Projects 仅提供工作视图而不成为第二份事实来源。Dependabot 仅建议 Go、pnpm、GitHub Actions 与 Docker 更新，普通版本 PR 指向 dev，依赖更新仍需现有 CI 与安全审阅。每周 GitHub 活动统计只读取 Issues/PR 并写 Job Summary，不授予仓库写权限、不下载或执行不可信 PR 代码。Wiki 使用仓库版本化的页面草稿作为源，单独的 Wiki Git 仓库需初始化后才发布。

GitHub 定时工作流与 Dependabot 配置从默认分支读取。因此这里只提交到 dev 并在审阅通过后按独立发布 PR 同步 private，不能宣称尚未部署的调度已经生效。

## Alternatives considered

- 自动开放所有安全和依赖更新 PR 直接合并：不符合敏感源码与依赖变更必须独立审阅的约束，不采用。
- 让 Wiki / Projects 取代现有 docs/backlog.md 和冻结设计：会导致任务与契约双份维护，不采用。
- 通过 pull_request_target 执行外部贡献代码生成统计：会引入权限与不可信代码风险，不采用。

## Consequences

新增 GitHub 使用入口和维护人员工作流文件，没有 Go/React/SQL 业务运行路径变更。Issues 模板在功能分支合入后可供使用；周期任务只有同步默认分支后才能执行。Wiki 的独立初始化与同步需要人工完成，不应从 PR 的绿色 CI 推断对外发布。历史凭据问题仍由 INK-252 单独阻断仓库公开。

## Verification

工作流已执行 actionlint .github/workflows/repository-health.yml 静态检查，返回 0；新文件经提交 diff / Skills / notes 检查。GitHub PR #30 的三项要求 CI 曾在最终修订前通过，合并最新 dev 后需要重新运行。无实际定时调度、Wiki 初始化或生产运行验证，本笔记不作这些完成声明。