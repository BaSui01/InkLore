# Agent Note: INK-167阶段验收：LLM目录与管理路由

Status: implemented

Task: INK-167
Scope: 原 docs/acceptance-INK-167.md 的历史验收证据与迁移

## Problem

该阶段验收记录位于 docs 读者文档目录，与长期有效的系统设计和用户指南混放。

## Decision

将源文档全文迁入工程记忆，保留历史运行路径、验收证据与未覆盖边界。本次只迁移记录，不重新声明旧版本测试在当前部署通过。

## Alternatives considered

继续存放 docs 可保留老路径，但会持续混淆文档职责；直接删除会丢失可追溯的证据。因此采用保留原始记录并同步引用的迁移方案。

## Consequences

相关入站链接需同步修改；旧记录中的验证命令和结果属于历史快照，应以当前代码及 CI 证据为准。

## Verification

内容来自 private 分支原文，仅将原有标题降级以符合笔记结构，并调整相对链接。以下是历史档案，不是新的验收结果。

### 原始验收档案

### INK-167阶段验收：LLM目录与管理路由

状态：doing，不能标记done。该阶段已完成R1迁移、Catalog/SecretRef/strict slice、HTTP管理transport和App真实装配；Budget基础API和Redis admission Lua 已有真实PG/Redis证据，但完整生产 Gateway、driver 与预算故障场景仍未完成。

### 复现命令

PowerShell：
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go build ./...
go run ./cmd/acceptance/ink167 inventory

### 当前证据

- 000005_llm在隔离PG已应用；12张LLM表存在，tenant_id为UUID。
- frozen_llm=32，mounted_llm=32。
- 真实Admin Operator GET：providers、models、channels、keys、task-policies、pricing、usage、budget均返回200并通过运行路由；fixture和生产admin bootstrap均显式Provision预算，不由GET懒建。
- 32个冻结管理操作已挂载；30个Catalog/pricing/usage操作调用R1 Catalog，Budget GET/PUT调用持久化Budget service。Budget reserve/dispatch/settle/recovery和Redis admission已有可编译Port/PG/Lua切片，但尚未由App worker和Gateway生产链完整接线。
- R1四层Catalog包含tenant隔离、CAS、policy不可变版本、pricing insert-only、SecretRef allowlist和provider-bound HMAC fingerprint；strict JSON/schema边界和精确Cost已编译。
- INK-160/161/163/164/165/166回归在LLM装配后继续通过；浏览器和真实供应商调用未宣称。

### 未完成缺口

- Budget真实Configure/CAS、北京时间日期和显式Provision已通过；admin bootstrap的Budget Provision与身份创建在同一UoW。跨日（cross-midnight settlement）、Reserve/Settle rollback以及reservation replay行为已通过真实PG事务与UoW验收验证。
- Redis RecoveryProbe、Lua Acquire/Renew/Report、100个唯一 invocation 并发 Acquire/Report、1:3权重SWRR分布（40次为10:30）和 fixture client 关闭后的 fail-closed Acquire 已通过；真实 PG RecoveryProbe/KeyHealth adapter、10秒联合续租和生产 Gateway 仍未验收。
- TLS/host binding的真实provider drivers、capability与driver能力联动、unknown outcome语义。
- Catalog mutation的真实PG/API正反例、Schema strict正反例、SecretRef安全fixture和并发预算证据。

go run ./cmd/acceptance/ink167 all退出0并通过原有路由盘点与4个真实的Budget Reserve/Settle/Rollback/Replay行为探针；不以编译、迁移、路由挂载或legacy实现代替上述行为证据。

