# Agent Note: 数据库版本以实际服务核验

Status: implemented

Task: INK-210
Scope: Compose、dbx、数据库运行基线

## Problem

旧设计指定PG16，而用户系统服务已经为PG17.11；创建PG16验收容器会偏离用户所需环境。当前Docker default没有容器，不能把系统服务误称Docker数据库。

## Decision

按[ADR-0022](../../reference/decisions/0022-postgresql-17-baseline.md)采用PG17。dbx使用项目既有连接参数，实际查询服务版本与扩展。Compose使用pg17供新部署；既有PG16卷须备份迁移到新目录，不直接换镜像挂旧卷。

## Alternatives considered

另建PG16可复用历史验收配置，但用户明确要求系统版本。直接改镜像并重启旧卷容易造成跨主版本不兼容，未采用。

## Consequences

现有inklore_r1已在17，不需数据迁移；没有因此声称其他环境已升级。独立验收库创建被dbx高风险SQL设置阻止，不绕过该限制写入业务库。

## Verification

dbx当前连接inklore-pg-r1实查17.11、vector0.8.0、pg_trgm1.6与46张public表；建库返回SQL_BLOCKED。无数据迁移或业务写入。详[验收](../testing/2026-09-20-acceptance-ink-203.md)。
