# ADR-0060：移除 R1 阶段命名并统一为单一迁移流

- 状态：Accepted
- 日期：2026-11-12（承接 2026-11-04 的初版裁决，按当前仓库结构重做）
- 关联任务：INK-301
- 决策者：BaSui01（用户指令「完全移除 R1 的影响」，2026-11-12）
- 替代范围：不替代任何已接受 ADR 的技术裁决；仅退役阶段命名、阶段专用工具与阶段专用历史表名。领域边界、依赖 DAG、迁移校验和保护机制全部原样保留。

## 背景

R1 是 2026-09 重构阶段的名称，阶段工作早已结束，但命名渗入活代码与数据库标识，产生三处实质问题：

1. **迁移目录三套并存**：根目录 `migrations/*.sql`（18 个 pre-R1 旧血统文件，BIGINT 方言，**零 Go 导入者**，其 `000007` 假定 `agent_runs.id` 为 BIGINT 并重建 `agent_run_nodes`，与现役 UUID 方言冲突，对新库不可应用）；`migrations/r1/*.sql`（17 条 UUID 方言，服务器启动、`cmd/migrate`、`cmd/admin` 实际使用的流）；`migrations/post_r1/*.sql`（7 条扩展迁移 + 校验脚本，使用独立历史表 `schema_migrations_post_r1`）。
2. **一次性阶段工具**：`cmd/migrate/r1migration/`（INK-177 旧库→R1 数据搬迁引擎，11 个文件）无任何调用方。
3. **阶段措辞渗入**：约 40 处注释/错误文案（`frozen R1 contract`、`R1 PostgreSQL unavailable`、`冻结 R1 流`）、archguard 规则编号 `R1–R5`、契约文件名 `agent-r1x-schemas.json`、内部 context key `r1_request_id/r1_error_code`、Redis 键前缀 `inklore:r1:*`、错误目录版本 `R1`。

## 决策

1. **主流统一**：`migrations/r1/*.sql` 17 条**字节不变**移动到 `migrations/` 根（`git mv`，R100 相似度）；删除 18 个旧血统死文件与 `migrations/r1/migrations.go`。包收敛为唯一的 `InkLore/migrations`。因为文件名与字节不变，`LoadMigrations` 解析出的 version/name/checksum 与已应用库 `schema_migrations` 记录**逐条相同**，既有库（含本地与 staging）零影响。
2. **扩展流去阶段化并保留独立性**：`migrations/post_r1/` → `migrations/extensions/`（包名 `extensions`），历史表 `schema_migrations_post_r1` → `schema_migrations_extensions`（常量 `extensionTable`）。选择保留独立历史表而非合并进主流，是因为主流 17 条受校验和保护，合并需重编号并改写存量库记录，收益为零、风险为实。存量库只需一次幂等 `ALTER TABLE ... RENAME TO schema_migrations_extensions`（本地已执行；其它环境由维护者执行）。
3. **删除 `cmd/migrate/r1migration/`**。旧库数据迁移如未来需要，以中性命名、按当前 UUID 方言重新立项。
4. **去阶段化措辞**：错误文案与注释按机制描述；archguard 规则编号 `R1–R5` → `G1–G5`（保留既有编号序列与语义，本仓历史无 R4）；契约 `agent-r1x-schemas.json` → `agent-policy-schemas.json`（`agent-schemas.json` 的 4 处 `$ref` 同步，按目录枚举生成，重生成通过）；内部 context key → `request_id`/`error_code`；错误目录 `version` → `1.0`；`docs/system-design/data/*.sql` 设计资料头注释中性化。**Redis 键前缀不改**（见保留项：它们是跨版本协调状态）。
5. **保留（不可改）**：
   - **已应用迁移 SQL 的字节内容**（含 `-- INK-141 / R1 frozen target` 头注释）：受校验和保护，任何改动都会让既有库 `Up/Verify` fail-closed；
   - **准入与限流的 Redis 键前缀 `inklore:r1:llm:` / `inklore:r1:login:`**：这些键是跨版本**协调状态**（RPM 计数、并发租约、尝试标记、熔断状态），滚动升级期间新旧进程必须共享同一命名空间，否则会各自计数而突破供应商限流；改名需配套 drain-and-cutover 机制，故保留（AGENTS.md 要求持久化值与缓存键在有真实消费者时保持兼容）；
   - `FormulaVersion = "r1"` 与 `compliance-schemas.json` 的 `"const": "r1"`：冻结 DDL CHECK 约束下的数据版本值，属协议非阶段命名；
   - `DeepSeek-R1`：外部模型名；
   - `CHANGELOG.md`、`.agents/notes/`（含 archived manifest）、已接受 ADR：按 AGENTS.md 记录「当初为什么」，改写即伪造历史。

## 备选

- **仅改名不合并**：保留三套迁移目录，只把 `r1` 改中性名。被否：根目录 18 个死文件仍是陷阱（对新库不可应用），「完全移除」不成立。
- **把扩展流合并进主流**：需重编号 7 条并改写存量库迁移记录；生产库需停机映射，收益为零。被否。
- **连历史档案与已应用迁移 SQL 一起清洗**：违反 AGENTS.md 与校验和保护机制，会让所有既有库无法启动。被否。
- **关闭分支保护后 force push 重写远程历史**：会重写公开仓库历史、令协作者本地分叉、使 CI/部署引用失效。用户选择走 PR（远程最新树不再含 R1，历史保留）。

## 影响

- **收益**：迁移流唯一且自洽（主流 17 条 + 扩展流 7 条各有独立历史表）；阶段心智负担清零；守卫脚本、archguard 输出、契约文件名不再携带过期阶段语义。
- **成本**：一次性约 60 文件的改名/改写 diff；外部协作者对 `migrations/r1/...` 的旧链接失效（活文档已同步）。
- **兼容性**：已应用迁移的 version/name/checksum 不变，`database.Up/Verify` 行为不变；本地实测 `migrate -cmd status/up` 主流 17 条与扩展流 7 条全部「已应用/跳过」，零新应用。
- **安全边界**：无协议、鉴权、密钥或网络配置变化；archguard G1–G5 语义与原 R1–R5 完全一致，仅编号更名。
- **失败恢复**：若某环境库曾被旧根流单独打过补丁（代码侧无法证实），`database.Up` 会以 `migration stream mismatch` fail-closed 拒绝，不会静默错配；该库需按本 ADR 裁决重建或映射。

## 验证与实施边界

本轮实测判据（本地 PG 18.6 + `ai_novel` 库）：

- `go build ./...`、`go vet ./...`、`gofmt -l .` 全部退出 0（零漂移）；
- `go run ./cmd/archguard` G1–G5 全部 PASS；
- `go run ./cmd/i18ncheck ...` 中英词条一致（920/920）；
- `go run ./cmd/migrate -cmd status` 输出 `[main]` 17 条与 `[extensions]` 7 条均已应用；`-cmd up` 零新应用、全跳过；
- `node docs/system-design/tools/build-contracts.cjs`（246 operations，issues 空）、`check-design.cjs`（77 tables，无 issue/warning）、`scripts/check-platform-contracts.mjs`（102 条错误映射一致）均退出 0；
- `node scripts/generate-request-schemas.mjs` 与 `node scripts/codegen.mjs` 重生成通过。

实施边界：**生产 Supabase staging 库**的 `schema_migrations_post_r1` 改名需由维护者在维护窗口执行（一次幂等 ALTER TABLE RENAME），代码侧不再读取旧表名；本任务未访问任何远程数据库。

## 后续与关联

- 任务与证据：[INK-301](../../docs/backlog.md)、[工程笔记](../../.agents/notes/implemented/architecture/2026-11-12-retire-stage-naming.md)。
- 代码导读：[docs/architecture.md](../architecture.md)、[migrations/extensions/README.md](../../migrations/extensions/README.md)。
