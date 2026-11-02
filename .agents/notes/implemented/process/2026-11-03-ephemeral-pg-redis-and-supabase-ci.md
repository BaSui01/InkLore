# Agent Note: 隔离 PostgreSQL Redis CI 与 Supabase 只读预检

Status: implemented

Task: INK-255
Scope: .github/workflows/isolated-integration.yml、.github/workflows/supabase-staging-preflight.yml、docs/ci-supabase-staging.md

## Problem

仅凭 Go 编译和静态检查不能证明真实 PostgreSQL 17、pgvector、Redis 7 的连接或迁移路径正确。与此同时，已连接 Supabase 是独立在线服务，默认 public schema 和 Data API 角色边界不能等同于一次性 CI 容器。若直接让 Fork PR 使用真实数据库 Secret 或在共享 Supabase 的 public schema 运行完整迁移，可能损坏环境或泄漏其他租户数据。

## Decision

使用 GitHub Actions 每个 PR 的独立 PostgreSQL 17/pgvector 与 Redis 7 服务容器，只有固定的临时无价值 CI 密码，不依赖仓库或个人长期密钥。仓库现有迁移工具执行 up、表/扩展核对与重复执行，再运行 Go build、vet、archguard。容器由 Runner 生命周期销毁；这是隔离的真实数据库验收，不是线上功能测试。

Supabase 预检单独使用手动 workflow_dispatch 和受限 GitHub Environment，只有授权配置的环境密钥才能访问；执行前要求 TLS，SQL 只读检查专属 inklore_ci schema、anon/authenticated 访问权限和 vector 扩展，不运行迁移。尚未证明迁移能安全适配 Supabase：现有 000009_permissions.sql 中的 public 权限需要独立核查，不允许在共享 public 直接执行。

工作流对 PR 的默认权限只授予 contents:read，不运行 PR 作者传入的外部数据库连接串。密钥、日志和真实数据不进入版本库。

## Alternatives considered

- 仅运行编译与 archguard：成本低，但不检测迁移与数据库扩展问题，不选。
- 将正式 Supabase 作为每个 PR 的测试库：容易交叉污染，而且 Fork PR Secrets 与权限不可控，拒绝。
- 手动连接个人电脑上的 PostgreSQL/Redis：需要私人运行环境，无法作为统一 GitHub CI 验收，不采用。
- 直接对 Supabase public schema 应用所有现有迁移：无法保证租户隔离和权限安全，保留为单独评审事项。

## Consequences

普通 PR 可以持续获得真实临时 PostgreSQL/Redis 集成检查，减少依赖长期个人数据库。迁移和编译成功不代表业务 API、Admin/Creator 浏览器、外部模型、计费或 Agent 并发恢复通过。在线 Supabase 只读预检需要先配置独立、最小权限的 Environment Secret，不能自动转化为在线部署或完整数据库集成测试。

## Verification

2026-11-03 GitHub PR #37 最新初始 head 42548d9c54f33087a7212c8f104e0280d632e961 的 CI 报告：项目 Skills/notes、后端、前端以及“R1 PostgreSQL 17 + Redis 7 真实迁移验收”全部成功；具体记录以 GitHub Actions 对应 head 的工作流为准。新增笔记后必须对最终 head 再执行 CI，不沿用旧 head 结果作为最新验收。

已核实工作流包含 pgvector PG17 服务、Redis7、实际迁移 up、schema_migrations/业务表和扩展检查、重复迁移、构建静态门禁。Supabase staging 的预检流程尚未在带 Environment Secret 的 GitHub Runner 上完成，pg_trgm 和真实 API/多租户/模型调用验收仍未完成；不得标 INK-255 整体验收为 done。
