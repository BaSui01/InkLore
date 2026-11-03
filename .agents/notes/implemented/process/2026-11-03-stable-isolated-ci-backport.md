# Agent Note: 稳定分支最小接入隔离数据库集成验收

Status: implemented

Task: INK-260
Scope: private 受保护分支、GitHub Actions isolated-integration 与 supabase-staging-preflight

## Problem

InkLore 已在 dev 的 PR #37 验证临时 PostgreSQL 17、Redis 7 和真实 Go API，但默认分支 private 未包含这些工作流。已有 PR #41 还包含约 70 个文件和单独的发布风险，不能为了让默认分支支持手动 Supabase 预检而提前混合发布业务源码。

## Decision

基于当下稳定分支创建独立 PR，只复制 dev 中两个工作流和 isolated-integration 实际引用的 scripts/sql/supabase-staging-permissions.sql；保留已应用的 17 组 R1 迁移及稳定分支的 Go/前端依赖。独立复核 dev/private 的全部迁移源文件哈希相同，不依赖先合并 PR #41。

PR 和受保护分支 push 均运行隔离 PG/Redis 作业，使用临时数据库和公开无价值凭据；远程 Supabase 工作流仅允许受保护环境中的人工 workflow_dispatch，且需要限权只读数据库角色与 SUPABASE_STAGING_DATABASE_URL Secret，不接受外部 PR 的真实数据库密钥。只复制工作流并不意味着配置好远程凭据或自动发布云端数据。

## Alternatives considered

- 合并包含业务/迁移/安全治理混合改动的 PR #41：范围远超本轮授权，拒绝。
- 将 Supabase 管理员 DSN 写进 GitHub 文件或 PR：违反密钥隔离和最小权限，拒绝。
- 只在 dev 配置在线测试：默认分支没有手动工作流注册入口，难以维护和发布，拒绝。

## Consequences

单独的敏感 workflow PR 必须依仓库保护进行人工审核。发布后普通 PR 以及 dev/private 的 push 可以复验 PG、Redis、迁移和基础 HTTP 行为；不提供 24 小时常驻 Go/Agent 服务。受限只读云端账号及 Secret 未自动建立，远程检查只有在私密凭据正确配置后才可执行。

## Verification

已通过 GitHub 连接对比 private 与 dev 的 17 个 R1 SQL SHA，逐项全部一致；已核验 dev 的 Isolated Integration、CI 正常运行历史及 private 最新安全依赖版本。提交后须以 PR 到 private 的最新 CI/Integration 结果和独立 review 为准。尚未宣称最新 PR 已合并、远程 Supabase 预检通过或完整 Agent/浏览器业务验收。
