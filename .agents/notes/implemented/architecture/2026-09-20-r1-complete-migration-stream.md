# Agent Note: R1 完整版本化迁移链收敛

Status: implemented

Task: INK-208
Scope: migrations/r1, cmd/migrate, database.Verify

## Problem

此前 migrations/r1 仅包含 000001_platform 至 000005_llm（45 张业务表）。Agent、Inspiration、Compliance 域与 Permissions 安全角色的 DDL 虽已在 docs/system-design/data/ 冻结，但未形成版本化迁移文件，导致新环境启动或集成测试时需要手工加载裸 SQL，破坏了自动化部署与 database.Verify 的完整性校验。

## Decision

按照 [ADR-0024](../../reference/decisions/0024-r1-complete-migration-stream.md)，将冻结 DDL 规范化为版本化迁移链：
- 000006_agent.sql
- 000007_inspiration.sql
- 000008_compliance.sql
- 000009_permissions.sql

使得嵌入式迁移流与设计目录保持 1:1 对齐，完整包含全部 74 张业务表及角色权限控制。

## Alternatives considered

保持手工通过 psql 外部导入：容易导致各部署环境版本与 checksum 不一致，且无法利用 cmd/migrate 统一管理状态。

## Consequences

- 部署与初始化只需运行 cmd/migrate up，或通过应用配置自动执行，无需外部手工导入脚本。
- 完整应用后数据库包含 74 张业务表与 1 张 schema_migrations 表（共 75 张表）。
- 权限角色遵循最小特权原则，限制 runtime 角色的物理删除与不可变表的修改权限。

## Verification

1. 在隔离数据库 inklore_migrate_208 上执行 cmd/migrate -cmd up，新应用 9 条迁移，0 条失败；
2. 执行 cmd/migrate -cmd status 验证 000001~000009 全部处于已应用状态；
3. 执行二次 up 验证幂等跳过；
4. 探针验证 internal/platform/database.Verify 通过；
5. 主库 inklore_r1 增量升级 000006~000009 成功，表数量达到 75 张。
