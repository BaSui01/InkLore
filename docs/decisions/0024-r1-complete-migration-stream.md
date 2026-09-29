# ADR-0024：R1 完整版本化迁移链收敛

- **状态**：Accepted
- **日期**：2026-09-20
- **关联任务**：INK-208、INK-203
- **替代关系**：扩展原 migrations/r1 仅收录 000001~000005 的中间态，彻底补齐 Agent、Inspiration、Compliance 正式版本化迁移与最小权限角色脚本，使得迁移流与 docs/system-design/data 冻结目标完全 1:1 对齐。

## 背景与决策

此前 `migrations/r1` 仅包含 000001 至 000005（涵盖 platform, content, outline, worldview, llm 对应 45 张业务表）。Agent、Inspiration、Compliance 三域的 DDL 虽然已冻结在 `docs/system-design/data/`，但未形成版本化迁移序列，导致新环境启动或集成测试时需要手工加载 `agent.sql` 等脚本。

决定：
1. **补齐迁移脚本**：
   - `000006_agent.sql`：对应 `docs/system-design/data/agent.sql`（14 张表，状态机、审批、工作流与产物）；
   - `000007_inspiration.sql`：对应 `docs/system-design/data/inspiration.sql`（8 张表，素材、拆解、文风与灵感卡片）；
   - `000008_compliance.sql`：对应 `docs/system-design/data/compliance.sql`（7 张表，规则集、质检报告、处置与行文净化）；
   - `000009_permissions.sql`：对应 `docs/system-design/data/permissions.sql`（定义 inklore_runtime 与 inklore_retention 最小权限安全角色）。
2. **完整 74 表权威对齐**：
   - 000001 至 000008 迁移流包含完整 74 张业务表，与系统设计唯一来源和 `check-design.cjs` 验证保持完全一致。
3. **支持标准启动与工具链**：
   - `cmd/migrate` 与 `internal/app` 的 `database.Verify` / `database.Up` 原生识别 000001~000009 全序列；
   - 彻底废除部署时手工加载裸 SQL 脚本的例外路径。

## 代价与验证

- 新环境初始化时，仅需运行 `go run ./cmd/migrate -cmd up` 即可完成包括权限在内的全量建表与索引。
- 现有仅应用到 000005 的环境，执行 `migrate up` 时将按版本顺序无损增量应用 000006~000009。
- 验收必须在独立 PostgreSQL 17 数据库上完成 000001~000009 全程 Up 校验，并通过 `database.Verify` 检验无误。
