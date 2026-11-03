# Agent Note: INK-162阶段验收：平台文件、作业、Outbox、通知与观测

Status: implemented

Task: INK-162
Scope: 原 docs/acceptance-INK-162.md 的历史验收证据与迁移

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

### INK-162阶段验收：平台文件、作业、Outbox、通知与观测

状态：done。主代理已独立复跑全部平台组件与条件下载；后续Agent/Content业务handler与运维演练由171/172/176负责。

### 可复现命令

```powershell
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go run ./cmd/acceptance/ink162 all
```

2026-09-18主独立运行退出0，六组files_rejections/files/jobs_acceptance_adapter/outbox/schema_observability/websocket均通过。

- Files：真实multipart与四操作Schema、完整字节SHA/ETag/206/416、owner隔离、20MiB边界、绑定/业务引用阻删、UTF/MIME/DOCX与zip攻击拒绝。所有wire时间UTC。
- Jobs：使用真实scope授权fixture作品的验收adapter，证明租约fencing、跨到期checkpoint整事务回滚、Suspend/Wake、cancel、Recover不重复Execute。这不是生产content/agent作业链已接线的证据。
- Outbox：真实inbox去重、失败8次dead letter、真实审计。实际App.ReplayOutbox Creator403，缺cap直接Replay403，当前Operator重放成功且同tenant/event/actor/admin审计正确；dead-letter系统nil actor审计正确。
- WS：实际coder/websocket loopback连接，一次票据并发仅一个成功、Origin/aud、当前撤权断开1008；通知严格5字段，无正文。不是Chrome端到端证据。
- Schema/观测：本地Draft2020-12与恶意ref无网络；HTTP状态/SecretRef权限，非空metadata不泄露secret值，Redis不可用为degraded。

### 已解决与接线边界

`go run ./cmd/acceptance/ink162 conditional_download`退出0：已鉴权且ETag匹配返回304空body，非匹配返回完整200，授权失败不泄露ETag。ADR-0019已Accepted，源platform、派生contracts、freeze manifest和bodyless304 canary均通过。

实际server已接files、outbox、notifications、observability、schemavalidation；jobs通用状态机通过真实PG/scope验收adapter，领域handler注册与业务worker链留给171/172，指标采样/SecretRef部署目录/维护调度/备份演练留给176。平台任务不把消费Port adapter冒充领域业务实现。

公共层另有platformextensions六组真实PG验证，26组160及24组161回归通过。全仓build、archguard、四份DDL精确一致和97错误目录检查、冻结检查通过。
