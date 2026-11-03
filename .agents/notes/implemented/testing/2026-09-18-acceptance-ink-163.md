# Agent Note: INK-163验收证据：content作品、卷、章、版本与排序

Status: implemented

Task: INK-163
Scope: 原 docs/acceptance-INK-163.md 的历史验收证据与迁移

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

### INK-163验收证据：content作品、卷、章、版本与排序

主代理于2026-09-18在实际装配后独立复跑，下述 12 组退出0；27 个原有 content 操作逐个取得成功响应并通过完整冻结Schema，确认操作由 INK-166 单独覆盖。子代理编译结果不替代此证据。

### 环境与复现

隔离 pgvector/PG16：127.0.0.1:55432，数据库 inklore_r1_acceptance_161；Redis7：127.0.0.1:56379。CLI先验证数据库名再迁移，随机生成身份和签名密钥，不打印凭据。

```powershell
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go run ./cmd/acceptance/ink163
```

### 已复现证据

first-novel、resource-crud、unicode-idempotency、concurrent-cas、diff-reconstruction-statistics、snapshot-failure-rollback、organization-complete-set、deleted-volume-parent、delete-restore-position-count、archive-deleted-parent、cross-owner-revocation、successful-schema-coverage。

真实两个请求CAS仅一个提交；同key返回原结果，异载荷409。InsertVersion wrapper在同事务观察正文已更新后故障，9表全部列指纹、版本、作品计数、正文、投影和幂等记录恢复原值；相同key之后成功。验证中文、CRLF、非BMP码点/hash；diff可独立重构A/B，等长替换仍产生added/removed。排序漏项/重复/跨书等拒绝且无写，软删恢复压紧位置/计数，归档与已删除父拒写。角色撤销403，session吊销401。

### 集成与范围

版本revision=1的chapter_version投影及章每revision历史证明同事务发布。版本UUID与章节归属由content权威读取后交给INK-166 adapter；确认操作不计入本任务的27操作覆盖。复制、导入导出/检索统计分别留后续任务。

公共R1迁移000002_content、000003_outline、000004_worldview原样复制冻结DDL，已在真实PG应用并验证checksum。go build ./...、archguard、97错误映射与四份DDL字节校验、冻结设计检查通过；INK-160的26组和INK-161的24组回归通过。go list -deps ./cmd/server确认对应四层包已进入运行路径；旧import仍可达，不宣称旧源码清理。

独立验收脚本位于 cmd/acceptance/ink163；公共harness按METHOD+template记录真实成功覆盖，不能仅凭路径或静态路由计数通过。当前契约仍为ADR-0018批准后的R1.1；ADR-0019的304仅为未批准提案。

