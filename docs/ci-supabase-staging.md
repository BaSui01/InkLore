# InkLore 隔离在线集成验收（INK-255）

## 目标与范围

GitHub Actions 对每个 PR 运行**临时** PostgreSQL 17（含 pgvector）与 Redis 7，应用仓库原生 R1 迁移、重复执行检查以及 Go 构建/架构门禁。作业运行完数据销毁，**不依赖个人电脑和任何真实数据库密钥**。

独立的 Supabase 数据库只用于受信任手动启动的只读预检，不让来自 Fork 的 PR 获得数据库连接串，**不**自动部署或迁移真实 Supabase 数据。

## 已连接的 Supabase 测试项目

- 项目：`pgsql`，东京区域 `ap-northeast-1`；项目 ID：`rjxtggttklpqbfjesrgp`。
- 使用专属私有 schema `inklore_ci`，并撤销 `PUBLIC`、`anon`、`authenticated`、`service_role` 对它的权限。
- 已成功执行 SQL 证明匿名与普通授权角色无 schema USAGE 权限。
- `pgcrypto` 原有，`pgvector` 已启用；`pg_trgm` 的安装需要另行复核。
- **尚未把 R1 业务表迁移到 Supabase**。因为 R1 `000009_permissions.sql` 硬编码 `public`，直接修改 `search_path` 并不完全足够。必须先审阅迁移权限契约和受控专属数据库角色，不能将业务表直接释放给 Supabase 默认 Data API。
- Supabase Free 项目可能因不活跃暂停，不保证 24 小时在线。

## Workflows

### `isolated-integration.yml`

PR 到 `dev` / `private`、手动触发：容器使用与生产不同的固定、仅供本次作业的无价值凭据；无外部 Secrets；验证 PostgreSQL/Redis、执行真实 R1 迁移及重复迁移，执行 Go build/vet/archguard。Creator/Admin 静态构建仍由既有 `ci.yml` 负责。

### `supabase-staging-preflight.yml`

仅 `workflow_dispatch`，并绑定 `supabase-staging` Environment。该 Environment 应设置仅允许可信分支，推荐需要维护者审批。把**单独、限权、仅能访问 `inklore_ci` 的数据库角色**的 TLS 连接字符串配置到该 Environment Secret：`SUPABASE_STAGING_DATABASE_URL`。

该作业只读检查 schema 存在、Data API 角色无权限、向量扩展启用；它不会写数据、执行迁移或者调用付费模型。**尚未配置该 Secret**，因此只能在配置完成后验收。

注意：不要使用 Supabase 高权限 `postgres` 超级用户作为持续集成身份。不要把连接字符串提交到代码、Issue 或日志；连接串需要 `sslmode=require` 或更强 TLS 校验。

## 后续端到端验收

迁移验收完成后，另用受控服务运行 Go API、Agent Worker 和 Redis。分别测试 `/healthz` 与 `/readyz`、真实 Postgres 读写回滚、跨租户拒绝、Agent 断线/恢复、API SSE、Creator/Admin 双端浏览器真实业务操作。

GitHub-hosted Runner 不适合当永久运行服务器。免费构建与免费常驻托管属于两种不同资源；不要把 CI 成功等同于端到端验收。
