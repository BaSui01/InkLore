# 隔离集成验收

## 自动验收范围

[隔离集成工作流](../.github/workflows/isolated-integration.yml)在临时 PostgreSQL 18（含 pgvector）与 Redis 7 上应用仓库迁移，检查重复执行、回滚、角色权限及 API 的健康、就绪和身份请求。PR、dev/private push 与手动触发使用各自提交的代码；临时数据库随作业销毁，不使用共享数据库密钥。

[并发策略验收](../.github/workflows/ink228-integration.yml)核对迁移记录、租户组合外键及任务策略并发行为。迁移历史位于 [migrations/](../migrations/)，保留版本、文件名与校验和。当前文件数量不是永久架构约束。

[常规 CI](../.github/workflows/ci.yml)执行 Go build/vet/gofmt、架构检查、i18n、工程记录与双端构建。具体覆盖以工作流和脚本为准；构建成功不证明 Agent、模型、编辑器或完整作品导出已经验收。

## 私有 schema 权限覆盖

初始权限迁移包含 public schema 的历史设定。[测试环境权限覆盖](../scripts/sql/supabase-staging-permissions.sql)用于隔离的 inklore_ci schema，收回 public 访问并限制运行/保留角色；不改写历史迁移。

覆盖脚本当前只审查过前 17 个迁移，故保留明确版本数保护，防止新增表自动取得未经审查的权限。追加迁移时应重新审查授权集合并更新相应保护，不能为通过 CI 直接放宽它。普通迁移完整性另以仓库真实版本、名称、校验和核对；权限覆盖适用范围与普通迁移数量是两件事。

## 共享测试环境只读预检

[Supabase 预检工作流](../.github/workflows/supabase-staging-preflight.yml)仅接受手动触发，并使用 supabase-staging Environment。维护者应限制可信分支和批准范围，配置仅能访问目标 schema 的受限角色及 TLS 连接串到 SUPABASE_STAGING_DATABASE_URL；不使用超级用户作为持续集成身份，不在日志输出凭据。

工作流本身只读检查。共享环境是否在线、Secrets 是否已配置、该提交能否手动触发，都应在操作时重新确认。本轮治理未连接或更改 Supabase。此前部署信息与原始验收叙述见 [历史记录](../.agents/notes/reference/implementation/ci-staging-history.md)，不能替代当前环境证明。

## 业务验收边界

长期创作链需要额外观察 UI、API、数据库、Worker 与实际模型结果，覆盖跨租户拒绝、断线恢复、事实确认、作者采纳与可下载导出。每条运行结论记录源码 SHA、环境、操作和结果；CI 绿灯仅适用于实际覆盖的场景。
