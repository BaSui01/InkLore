# InkLore 隔离在线集成验收（INK-255）

## 目标与范围

GitHub Actions 对每个 PR 运行**临时** PostgreSQL 17（含 pgvector）与 Redis 7，应用仓库原生 R1 迁移、重复执行检查、Go 构建/架构门禁及真实 API 启动和请求验收。作业运行完数据销毁，**不依赖个人电脑和任何真实数据库密钥**。

独立的 Supabase 数据库只用于受信任手动启动的只读预检，不让来自 Fork 的 PR 获得数据库连接串，**不**自动部署或迁移真实 Supabase 数据。

## 已连接的 Supabase 测试项目

- 项目：`pgsql`，东京区域 `ap-northeast-1`；项目 ID：`rjxtggttklpqbfjesrgp`。
- 使用专属私有 schema `inklore_ci`，并撤销 `PUBLIC`、`anon`、`authenticated`、`service_role` 对它的权限。
- 已成功执行 SQL 证明匿名与普通授权角色无 schema USAGE 权限。
- PostgreSQL 的 pgcrypto、vector、pg_trgm 已验证均安装在 extensions schema。
- 已向 inklore_ci 私有 schema 运行 17/17 组 R1 原始 SQL，登记完整源文件校验和。真实 SQL 复核确认 80 张私有业务表、public 中 0 张业务表；R1 000009 权限迁移硬编码 public 的问题用 scripts/sql/supabase-staging-permissions.sql 做测试环境隔离覆盖（ADR-0038），不篡改历史迁移。
- Supabase Free 项目可能因不活跃暂停，不保证 24 小时在线。

## Workflows

### `isolated-integration.yml`

PR 到 `dev` / `private`、手动触发：容器使用与生产不同的固定、仅供本次作业的无价值凭据；无外部 Secrets；验证 PostgreSQL/Redis、执行真实 R1 迁移及重复迁移，执行 Go build/vet/archguard。Creator/Admin 静态构建仍由既有 `ci.yml` 负责。

### `supabase-staging-preflight.yml`

仅 `workflow_dispatch`，并绑定 `supabase-staging` Environment。该 Environment 应设置仅允许可信分支，推荐需要维护者审批。把**单独、限权、仅能访问 `inklore_ci` 的数据库角色**的 TLS 连接字符串配置到该 Environment Secret：`SUPABASE_STAGING_DATABASE_URL`。

该作业只读检查 schema、角色权限、R1 迁移数与扩展状态。GitHub Environment supabase-staging 已设置仅允许受保护分支；只读数据库账号和 SUPABASE_STAGING_DATABASE_URL Secret 尚未配置，故远程预检工作流仍待运行验收。

注意：不要使用 Supabase 高权限 `postgres` 超级用户作为持续集成身份。不要把连接字符串提交到代码、Issue 或日志；连接串需要 `sslmode=require` 或更强 TLS 校验。

## 权限与安全验收（2026-11-03）

已实际执行 `scripts/sql/supabase-staging-permissions.sql` 与 `scripts/sql/supabase-staging-function-hardening.sql`，复核运行角色有授权的 SELECT/INSERT 而没有 DELETE、不可变账本 UPDATE 与 schema_migrations 修改权限；保留数据角色不可删除 ledger 事件。四个历史触发函数固定到 `pg_catalog, inklore_ci, extensions` 搜索路径，Supabase Security Advisor 的当前安全 lint 数量为 0。该结果不能等同于完整的跨租户与业务授权渗透测试。

## 后续端到端验收

迁移验收完成后，另用受控服务运行 Go API、Agent Worker 和 Redis。分别测试 `/healthz` 与 `/readyz`、真实 Postgres 读写回滚、跨租户拒绝、Agent 断线/恢复、API SSE、Creator/Admin 双端浏览器真实业务操作。

GitHub-hosted Runner 不适合当永久运行服务器。免费构建与免费常驻托管属于两种不同资源；不要把 CI 成功等同于端到端验收。
