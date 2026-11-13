# Agent Note: 移除 R1 阶段命名并统一迁移流

Status: implemented

Task: INK-301
Scope: `migrations/`（17 条主流 `git mv` 到根、`post_r1` → `extensions`、删除 18 个旧血统死文件）、`cmd/migrate/`（双流 runner 改名 + 删除 `r1migration/`）、`cmd/admin/`、`cmd/archguard/`（规则编号 R1–R5 → G1–G5）、`internal/app/`、`internal/platform/{database,errors,idempotency,logger,response,identity}/`、`internal/{compliance,inspiration,llm}/`、`scripts/{sql,ci}`、`.github/workflows/`、`docs/`（architecture、ci-*、system-design、decisions/0060）、`deploy/staging/`、契约 `agent-r1x-schemas.json` → `agent-policy-schemas.json`、错误目录版本 `R1` → `1.0`

Related: [ADR-0060](../../../../docs/decisions/0060-retire-stage-naming.md)

## Problem

用户指令：项目完全不能有「R1 阶段」。R1 是 2026-09 重构阶段的命名，阶段早已结束，但命名残留在活代码与数据库标识里，形成三处实质问题：

1. **三套迁移目录并存**：根 `migrations/*.sql` 18 个旧血统文件（BIGINT 方言，**零 Go 导入者**，`000007` 假定 `agent_runs.id` 是 BIGINT，与现役 UUID 方言冲突，对新库不可应用）；`migrations/r1/*.sql` 17 条（UUID 方言，服务器启动/`cmd/migrate`/`cmd/admin` 实际使用）；`migrations/post_r1/*.sql` 7 条（独立历史表 `schema_migrations_post_r1`）。
2. **一次性阶段工具**：`cmd/migrate/r1migration/`（11 文件，零调用方）。
3. **措辞渗入**：约 40 处注释/错误文案、archguard 规则编号、契约文件名、内部 context key（`r1_request_id`）、Redis 键前缀（`inklore:r1:*`）、错误目录 `version: "R1"`。

## Decision

- **主流统一且零影响**：17 条 `migrations/r1/*.sql` **字节不变** `git mv` 到 `migrations/` 根（git 记录为 R100 重命名），删除 18 个旧血统死文件与 `migrations/r1/migrations.go`；包收敛为唯一的 `InkLore/migrations`。因文件名与字节不变，`LoadMigrations` 的 version/name/checksum 与既有库 `schema_migrations` 记录逐条相同——**已应用库零影响**（本地实测 17 条全部跳过）。
- **扩展流去阶段化但保留独立表**：`post_r1/` → `extensions/`（`package extensions`），历史表 `schema_migrations_post_r1` → `schema_migrations_extensions`（常量 `database.ExtensionHistoryTable`）。不合并进主流：合并需重编号并改写存量库记录，收益为零、风险为实。存量库只需一次幂等 `ALTER TABLE ... RENAME TO schema_migrations_extensions`（本地已执行，生产由维护者执行）；未完成改名的库在扩展流迁移时会被 fail-closed 拒绝（2026-11-13 追加守卫），不会静默重放已冻结的扩展迁移。
- **删除 `cmd/migrate/r1migration/`**，未来旧数据迁移以中性命名重新立项。
- **措辞与命名**：错误文案/注释按机制描述；archguard `R1–R5` → `G1–G5`（保留原编号序列，本仓历史无 R4）；`agent-r1x-schemas.json` → `agent-policy-schemas.json`（`agent-schemas.json` 4 处 `$ref` 同步）；内部 context key → `request_id`/`error_code`；错误目录 `version` → `1.0`；`docs/system-design/data/*.sql` 设计资料头注释中性化；`deploy/staging/README.md` 措辞中性化。
- **保留（不可改）**：**准入与限流的 Redis 键前缀 `inklore:r1:llm:` / `inklore:r1:login:`**（跨版本协调状态：RPM 计数、并发租约、熔断状态；滚动升级期间新旧进程必须共享同一命名空间，改名需配套 drain-and-cutover，且 AGENTS.md 要求缓存键在有真实消费者时保持兼容）；已应用迁移 SQL 的字节内容（含 `-- INK-141 / R1 frozen target` 头注释，受校验和保护，改则既有库 fail-closed）；`FormulaVersion = "r1"` 与 `compliance-schemas.json` 的 `"const": "r1"`（冻结 DDL CHECK 约束下的数据版本值，属协议）；`DeepSeek-R1`（外部模型名）；`CHANGELOG.md`、`.agents/notes/`、已接受 ADR（封印历史）。

## Alternatives considered

- **仅改名不合并**（三套目录只改中性名）：根目录 18 个死文件仍是「对新库不可应用」的陷阱，「完全移除」不成立。未选。
- **扩展流合并进主流**：需重编号 7 条 + 改写存量库迁移记录 + 生产停机映射，收益为零。未选。
- **连已应用迁移 SQL 与历史档案一起清洗**：会让所有既有库无法启动，违反校验和保护与 AGENTS.md。未选。
- **关闭分支保护强推重写远程历史**：会重写公开历史、协作者分叉、CI/部署引用失效；用户选择走 PR（远程最新树不再含 R1，历史保留）。未选。

## Verification

- `go build ./...` = 0、`go vet ./...` = 0、`gofmt -l .` 空输出（0 漂移）。
- `go run ./cmd/archguard`：G1/G2/G3/G5 全部 PASS。
- `go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/{zh-CN,en-US}.ts`：920/920 一致。
- `go run ./cmd/migrate -cmd status`：`[main]` 17 条、`[extensions]` 7 条均已应用；`-cmd up` 零新应用、全部跳过（字节不变重命名的实证）。
- `node docs/system-design/tools/build-contracts.cjs`：246 operations、issues 空（错误目录 version 已为 1.0）。
- `node docs/system-design/tools/check-design.cjs`：77 tables，issues/warnings 空；`node scripts/check-platform-contracts.mjs`：102 条错误映射一致。
- 复扫：`cmd/`、`internal/`、`scripts/`、`.github/`、`frontend/packages/` 内 R1 阶段字样 0 命中；剩余命中仅为已应用迁移 SQL 的封印注释、`FormulaVersion='r1'`、`DeepSeek-R1`。

## Consequences

- 生产 Supabase staging 库的 `schema_migrations_post_r1` 改名需维护者在维护窗口执行一次幂等 `ALTER TABLE`；代码侧不再读取旧表名。
- `scripts/codegen.mjs` 第 192 行用 `spawnSync('npx', ...)` 调 `openapi-typescript`，在 Windows 上因不解析 `.cmd` 而 `ENOENT`（既有环境限制，与本次改动无关）；本次前端生成物经核查无需变更（已含相同 schema，且无 R1 字样）。建议后续改为 `spendSync('npx.cmd')` 或经 `pnpm exec` 调用。
- 前端生成物 `.gen.ts` 与 `.codegen/specs/*.json` 未因契约改名产生类型变化；`.codegen/specs/{agent,llm}.json` 的更新反映打包来源文件的真实变化，已随本次提交。
