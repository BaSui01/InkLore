# Agent Note: 历史迁移包装不再提前提交执行器事务

Status: implemented

Task: #105
Scope: PostgreSQL 迁移 Up/Down、事务锁与隔离数据库失败恢复

## Problem

真实 PG17 验收发现，迁移器在 GORM 事务内执行带 BEGIN/COMMIT 的历史 SQL。内层 COMMIT 提前释放外层事务和 advisory lock，后续 SAVEPOINT 报错但迁移命令仍可退出成功，导致 DDL、历史登记和后续迁移失败无法保证共同回滚。

## Decision

只在执行时剥离历史文件独占行的最外层事务包装；保持原文件、函数体和校验和不变。Up 整条迁移流持有原事务锁，Down 使用相同锁并在一个事务中执行所选回滚；回滚前检查原始 name/checksum。失败返回不报告已经提交/撤销的版本。

## Alternatives considered

修改已应用 SQL 会破坏历史校验和。移除执行器事务会让历史登记脱离业务 DDL。只忽略 SAVEPOINT 报错无法保证并发串行和原子性。因此保留执行器权威事务，并兼容现有明确的外层包装。

## Consequences

没有新迁移，没有共享 Staging 或生产写入。失败会回滚本次流内所有尚未提交的迁移；Up 的 R1 与 post-R1 仍是原有两个独立调用，未声称跨两条流原子提交。Down 不再允许已修改的历史脚本。只接入已有隔离 CI，不新增部署链路。

## Verification

PostgreSQL 17.11 + Redis 7.4.2 独立实例完整应用 17 条 R1 与 3 条 post-R1，再执行幂等重放，原有 SAVEPOINT/无事务警告消失。实际 PG 故障注入验证第二条迁移 SQL 失败时先前 DDL/数据/版本记录全部回滚；历史表写入失败时 DDL 同样回滚；两个并发执行器只写入一次并安全重放；改动历史校验和被拒绝；多步 Down 中后续失败不留下部分撤销；合法 Down 可再次 Up。

go test -race ./internal/platform/database、Go build/vet、archguard、gofmt 与 diff 校验通过。测试必须显式提供 INKLORE_MIGRATION_TEST_DSN，且只接受 loopback 上的 inklore_ci/inklore_e2e 数据库。#105 的远程 Supabase、真实模型和完整 Run 恢复仍需各自证据，未宣称全部完成。
