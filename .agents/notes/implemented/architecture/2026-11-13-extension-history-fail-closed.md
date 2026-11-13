# Agent Note: 扩展流历史表 fail-closed 守卫

Status: implemented

Task: https://github.com/BaSui01/InkLore/pull/153
Scope: `internal/platform/database/migrate.go`（`GuardExtensionHistory`、`ErrLegacyExtensionHistory`、`ExtensionHistoryTable`、`UpExtensions`/`StatusExtensions`）、`cmd/migrate/main.go`、`docs/decisions/0060-retire-stage-naming.md`、`docs/maintenance/stage-naming-cutover.md`、`migrations/extensions/README.md`

## Problem

ADR-0060 把扩展流历史表从 `schema_migrations_post_r1` 改名为 `schema_migrations_extensions`，并要求其它环境由维护者执行一次幂等 `ALTER TABLE`。修订前的实现只在文档层面约束「先改名再跑 migrate」：目标历史表按 `CREATE TABLE IF NOT EXISTS` 建表，既有库若仍停留在旧表名，`AppliedRecordsIn` 读到的就是一张刚建出来的空表，7 条已冻结的扩展迁移会被当作未应用而重放；只有撞上非幂等 DDL（如 `000001_platform_mail_settings` 的 `CREATE TABLE`）才会失败。ADR-0060 原文还写明「代码侧不再读取旧表名」，使这条风险无法用代码兜住。

## Decision

- 用户裁定（2026-11-13）改为 fail-closed 守卫，并授权修订 ADR-0060 的该句口径：静默重放冻结迁移的风险高于多读一次旧表名。
- 新增 `GuardExtensionHistory`：旧表名经 `to_regclass` 按当前 `search_path` 解析（与迁移自身的目标 schema 一致），旧表存在且记录数大于 0 时返回 `ErrLegacyExtensionHistory`，消息内含旧表名、记录数与可直接执行的 `ALTER TABLE ... RENAME TO ...`。
- 扩展流入口收敛为 `UpExtensions` / `StatusExtensions`（`cmd/migrate` 是唯一调用方），守卫在 `EnsureVersionTableIn` 之前执行 → 被拒时数据库状态完全不变，不再出现「status 先建出空表」。
- `cmd/migrate` 的 `up` 分支用 `errors.Is` 判定该错误并原样回显（其余错误保持原有脱敏文案）；`status` 分支本就回显 `%v`。
- 自动改名仍不做：多实例并发启动会竞争，且改名属于破坏性 DDL。

## Alternatives considered

- **只保留文档约束**（不改代码）：风险是静默重放，仅靠非幂等 DDL 偶然失败兜底。未选。
- **守卫放进通用 `AppliedRecordsIn`**：主流也会被牵连扫描，且泛型函数无法自然区分扩展流。未选。
- **仅由 CLI 直接调 `UpIn(..., ExtensionHistoryTable)` 并在调用处检查**：未来新增调用方容易绕过。未选（改为单点入口）。
- **自动改名**：并发启动竞争 + 破坏性 DDL，且与 ADR-0060「由维护者在维护窗口执行」冲突。未选。
- **回显所有底层错误**：会带出连接主机/用户等信息，违反仓库的脱敏约定。未选（只对守卫错误放行）。

## Consequences

- 未完成改名的库在 `up`/`status` 上得到明确、可操作的拒绝，而不是重放冻结迁移；已正确改名的库与全新库行为不变（本地开发库、CI 一次性库实测均 exit 0）。
- 每次扩展流迁移多一次 `to_regclass`（必要时再加一次 `count(*)`）查询，成本可忽略。
- `search_path` 内若存在别的同名旧表且仍有记录，也会被拒绝——有意的 fail-closed 取舍。
- ADR-0060 的 2026-11-13 修订与本文共同构成新边界；生产 staging 的改名动作本身仍属维护者职责。
- 已迁移的常量名从 `cmd/migrate` 的 `extensionTable` 变为 `database.ExtensionHistoryTable`，相关文档已同步。

## Verification

- `go build ./...`、`go vet ./...`、`gofmt -l .`（零漂移）、`go run ./cmd/archguard`（G1/G2/G3/G5 全绿）；LSP 诊断 0 error。
- 探针库实测（`ai_novel_cutover_probe`，未触碰开发库 `ai_novel`）：全新库 `-cmd up` → exit 0（主流 17 条 + 扩展流 7 条新应用）；把扩展历史表改名回旧表名后，`-cmd status` 与 `-cmd up` 均 → exit 1 并打印改名指引，且 `to_regclass('schema_migrations_extensions') IS NULL` 为 `t`（被拒时未建表）；完成改名后 `-cmd status` → exit 0、`[extensions]` 7 条「已应用」；探针库已 DROP，无残留。
- 开发库 `go run ./cmd/migrate -cmd status` → exit 0（守卫不误报）。
- 文档门禁：`agent-notes check --base origin/dev`（115 篇）、`check-project-skills`、`check-design`（77 表）、`check-platform-contracts`（102 映射）通过。
- 未验证：生产 Supabase staging 的真实改名（无实例与凭据，仍属维护者动作）；多实例并发下的守卫行为未单独压测。
