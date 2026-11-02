# Agent Note: Supabase 私有 schema 与 GitHub 隔离集成门禁

Status: implemented

Task: INK-255
Scope: Supabase pgsql/inklore_ci、scripts/sql/supabase-staging-*、GitHub Actions 集成和预检工作流

## Problem

InkLore R1 的 SQL 对象默认使用不带 schema 前缀的名称，权限迁移 000009 又将范围显式写为 public。Supabase 的 public 默认供 Data API 暴露，直接迁移会把自管租户应用引入不必要的第二条数据访问面。每次前端 PR 都连接真实托管库也可能造成数据和密钥暴露，而且 Supabase Free 不保证持续在线。

## Decision

对日常 PR 采用 GitHub-hosted Runner 临时 PostgreSQL 17（pgvector）与 Redis 7，不提供真实环境 Secrets；原始迁移、重复迁移、Go build/vet/archguard、实际 API 就绪探针及 Creator 身份注册/重复写入检查由同一隔离作业执行，作业结束后数据销毁。Creator/Admin 静态构建继续由原 CI 负责。

Supabase 东京 pgsql 独立使用 inklore_ci 私有 schema，所有 R1 数据表、约束与迁移登记只在该 schema。原始 17 个 SQL 及其文件 SHA256 不改写；连接会话指定私有 schema、extensions、public 的顺序。由于 R1 000009_permissions 对 public 授权，应用专用 scripts/sql/supabase-staging-permissions.sql 在完成迁移后将 NOLOGIN 角色的访问权限补到私有 schema，并保持不可变事实与迁移登记表的更新/删除限制。触发器四个原始函数的 search_path 由单独可复现的硬化脚本固定。

GitHub supabase-staging Environment 限制只接受受保护分支，提供只读预检工作流。专用登录身份及 SUPABASE_STAGING_DATABASE_URL 尚未配置；不能用高权限 postgres 用户或 PR 输入直接代替。设计依据见 [ADR-0038](../../../../docs/decisions/0038-supabase-isolated-staging-schema.md)。

## Alternatives considered

- 直接在 Supabase public 创建 80 张业务表后补 RLS：增加 API 暴露面，与 InkLore 既有自管身份边界重叠，拒绝。
- 改写已应用的 000009_permissions.sql：会破坏迁移源文件哈希，拒绝。
- 每个外部 PR 连接同一个 Supabase 实库：存在跨 PR 数据竞争与 Secrets 暴露风险，拒绝。
- 全部本地人工测试：不能提供云端可复现门禁，只作为开发调试补充。

## Consequences

临时 CI 能证明真实迁移、基础设施就绪和部分 HTTP 行为；Supabase 隔离库能承接进一步的只读 SQL、权限安全审计，但不是全天在线 Web 服务。后续仍需要受限连接凭据、安全远程预检、Agent Worker、前后端浏览器以及故障注入验收。每次新增 R1 表、角色或函数都必须复查私有 schema 授权和 search_path，不依赖默认 public 的自动权限。

## Verification

2026-11-03 Supabase MCP 实际执行 PostgreSQL 17 的 17 组 R1 原始迁移，inklore_ci.schema_migrations 共 17 行（64 字符哈希），私有 schema 共 80 张表，public 新增业务表数为 0。pgcrypto、vector、pg_trgm 均位于 extensions；anon/authenticated/service_role 对 inklore_ci 的 USAGE 均为 false。NOLOGIN 的 inklore_runtime 允许业务表 SELECT/INSERT，但不允许 DELETE、修改不可变事实账本或修改 schema_migrations；inklore_retention 不允许删除 worldview_fact_ledger_events。Supabase Security Advisor 曾指出四个未固定 search_path 的触发函数，使用 staging 硬化脚本修复后安全 lint 为 0。GitHub Actions 已实测临时 PG/Redis 迁移和 API health/ready（见 PR #37）。新加入的 Creator 注册/会话验证仍以 PR 最新 CI 结论为准。

PR #37 中发现 public 表和序列已有对象授权残留。最新权限覆盖已添加显式撤权，并在 GitHub Actions #41490710935 内使用包含无关 public 表/序列的隔离数据库实证：原始 R1 权限迁移后有多余授权，更新覆盖后 has_table_privilege / has_sequence_privilege 均为 false，私有 schema 的正常访问不受影响。当前修正版尚未在共享 Supabase 重新执行，审查线程保持开放。

没有调用付费模型，没有导入真实创作数据、没有运行浏览器与跨租户完整业务验收；远程只读数据库登录账号和 GitHub Environment Secret 创建未通过安全执行门禁，故远程工作流的自动验收尚未宣称完成。
