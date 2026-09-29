# Agent Note: 数据访问统一经 GORM 的边界

Status: implemented

Task: INK-243
Scope: cmd/migrate/r1migration/executor.go 与 internal/platform/database、uow 的数据库访问边界

## Problem

用户确认全项目的应用层数据库访问应统一由 GORM 发起。盘点时业务仓储和日志读写均已使用 `*gorm.DB`，但旧数据迁移工具仍单独调用 `sql.Open("pgx", dsn)`，绕开已有连接配置。`database/sql` 还出现在底层连接池、事务隔离选项和仓储 `sql.Null*` 值类型中，这些不是第二套应用访问驱动。

## Decision

迁移工具连接统一通过 `internal/platform/database.Open` 创建 GORM PostgreSQL 会话并由 `database.Close` 释放。应用读写保留 GORM `Create`、`Where`、`Raw`、`Exec` 与 `Transaction`；不将 `sql.DB` 的连接池管理、`sql.TxOptions` 或 `sql.Null*` 误判为绕过 GORM。版本化 SQL 迁移仍由 GORM 执行，不改写已应用迁移的校验和。项目日志专属 GORM 会话继续使用静默 logger 避免循环故障。

## Alternatives considered

完全删除 `database/sql` 导入表面上统一，但 GORM 管理连接池和设置 PostgreSQL 事务隔离仍需要标准库类型，强行删除会损坏现有事务语义；重新引入 pgx 直连绕开连接池将形成第二套数据路径。因此只清理实际直接建连的遗留路径。

## Consequences

少了一条单独的 `sql.Open` 路径；迁移工具在创建连接时经统一封装执行数据库探活。后续 INK-244 已将提取样本和空写入占位替换为真实旧表只读抽取及受约束的目标写入；旧库不存在且身份/历史映射不足，LIVE 仍被安全门禁阻断，不得宣称完成真实数据迁移。详细权衡见 [迁移安全笔记](../bug-fix/2026-09-29-r1-migration-live-fail-closed.md)。

## Verification

`go build ./...`、相关 `go vet` 与 LSP 诊断核验本次文件；静态盘点 `internal/cmd/migrations` 无应用业务使用 `sql.Open` 或 `sql.DB.ExecContext/QueryContext`。未对真实遗留数据库执行数据迁移，故任务保持 review。不新建测试文件。
