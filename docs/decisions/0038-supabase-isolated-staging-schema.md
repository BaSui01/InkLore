# ADR-0038: Supabase 测试数据库使用隔离 schema 与独立权限覆盖

- 状态：Proposed
- 范围：仅 InkLore 在线验收环境（INK-255），不改变生产数据库拓扑
- 日期：2026-11-03

## 背景

InkLore 原生 R1 DDL 大部分对象不带 schema 名，`000009_permissions.sql` 对 `public` 进行明确授权。Supabase 默认暴露 `public` Data API；直接将全量领域业务表迁入其中，不符合 InkLore 既有自管认证、租户隔离与后端单一数据入口约束。

东京 Supabase 项目 `pgsql` 当前为 PostgreSQL 17，项目可恢复并正常查询，但 Free 计划并不保证 24 小时在线。

## 决策

1. 在 Supabase 原有 `postgres` 数据库中使用专有 `inklore_ci` schema 作为**测试数据隔离域**。该 schema 不属于 Data API exposed schemas；禁授 `PUBLIC`、`anon`、`authenticated`、`service_role` 的 USAGE。
2. `pgcrypto`、`vector`、`pg_trgm` 统一由 Supabase `extensions` schema 提供。迁移会话仅针对该环境将 `search_path` 设置为 `inklore_ci,extensions,public`。不修改 R1 既有迁移内容及历史哈希。
3. 为避免覆盖 R1 `000009_permissions.sql` 的历史校验和，原样执行其权限声明，再以单独且可复现的 `scripts/sql/supabase-staging-permissions.sql` 将 NOLOGIN 的 `inklore_runtime`、`inklore_retention` 访问范围补到 `inklore_ci`；两角色仅依既有授权边界访问业务表。原迁移对 `public` 的授予不会用于 InkLore 业务，且该环境不在 `public` 建立 InkLore 表。
4. 后续真正登录连接的应用/迁移身份必须独立、最小权限、仅用于测试；数据库密码与 DSN 不入库。可信 GitHub Actions `supabase-staging` Environment 限制为受保护分支，外部 PR 永不能接收其 Secrets。默认 PR 集成验证使用临时 PostgreSQL/Redis，不触碰云端数据库。
5. 迁移及其重复校验属于数据库级验证；不能据此宣称 Creator/Admin、Agent 的真实业务行为已经验收。测试资源中不得导入正式用户正文及付费模型密钥。

## 未选择的方案

- 直接在 Supabase `public` 建立 InkLore 表并对其事后补 RLS：增加一个平行的授权系统，攻击面和治理复杂度增大。
- 直接更改冻结的 `000009_permissions.sql`：会破坏既有数据库的迁移哈希一致性。
- 用 `service_role` 或 Supabase API Secret 代替 InkLore 后端鉴权：越过现有业务授权与租户边界。

## 验证与限制

- `inklore_ci` 的 `anon`/`authenticated`/`service_role` USAGE 皆为 false。
- R1 17 组原始 SQL 和校验和以实际 `inklore_ci.schema_migrations` 为准，业务表只位于私有 schema，`public` 不新增业务表。
- `scripts/sql/supabase-staging-permissions.sql` 需要在此专属 schema 部署后执行，后续角色/表更新仍必须审查授权。
- Go API 连接到该 schema 时必须在服务端 DSN / 会话中明确配置 search_path，绝不能依赖 Supabase 默认公共路径。
- 该 ADR 为在线测试专用提案，正式生产存储、权限、迁移应独立审查。
