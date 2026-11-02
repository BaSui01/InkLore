# Agent Note: INK-228 的独立 PostgreSQL 领域契约冒烟检查

Status: implemented

Task: INK-228
Scope: .github/workflows/ink228-integration.yml、scripts/ci/ink228_schema.sql

## Problem

仅依赖 Go 编译不能验证 Agent 与 LLM 所需 PostgreSQL schema 的完整性。租户路由依赖 llm_channel_models、llm_task_policy_versions 和 agent_runs 以及 tenant_id 外键，若迁移丢失或关联被修改，编译仍可能通过。临时 GitHub Runner 能提供可复现的结构证据，但不能替代真实业务请求或双租户隔离验收。

## Decision

对 Agent/LLM、装配、持久化和迁移相关 PR 触发仅有 contents:read 权限的工作流，用一次性 PostgreSQL 17 和 Redis 7 服务执行仓库原有迁移两次，要求本期登记的 17 个迁移、三张领域表和 tenant_id 相关外键存在。使用固定且仅限一次性服务的公开 CI 测试密码，不向 PR 暴露 Supabase、Provider 或生产环境 Secret。手动 workflow_dispatch 可重复诊断。

把迁移数 17 固定为当前基线约束，后续新增正式迁移时必须同步审阅此检查，而不能默默跳过结构门禁。这是静态 schema smoke gate，不模拟真实 Agent Worker 的双租户数据访问或 Provider 调用。

## Alternatives considered

- 只依赖 Go build、vet 和 archguard：无法发现 SQL 结构缺失，不采用。
- 使用连接的共享 Supabase 作为所有 PR 的迁移目标：存在授权泄漏和不可逆环境污染风险，拒绝。
- 从自动门禁去掉迁移版本核对：可能把遗漏迁移当作成功，不采用。

## Consequences

增量仅为无密钥的一次性 CI 服务和只读 schema 验证 SQL，没有任何 Go/TS/数据库运行态代码变更。工作流会增加部分 GitHub Runner 分钟数，并对未来正式新增迁移提出更新 schema gate 的维护要求。和 INK-255 的通用 PG/Redis/健康流程可能共享底座，但此任务仍验证 INK-228 专用外键，后续应评估是否合并重复启动开销。

## Verification

2026-11-03 PR #38 对应 head 95441e522d54748dc9ead3535374e5ad799730d3 的 GitHub Actions 运行 41486362825 成功，名为 INK-228 PostgreSQL 17 + Redis 7 (isolated)。三项常规 CI 亦成功；本轮合并最新 dev 后仍应使用最终 head 重跑。未验证真实租户路由、模型 Key、Agent Worker、CAS 竞态、SSE 与权限拒绝，不将该结构检查视为 INK-228 功能验收。