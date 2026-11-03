# Agent Note: INK-166验收证据：章节确认与跨域事实事务

Status: implemented

Task: INK-166
Scope: 原 docs/acceptance-INK-166.md 的历史验收证据与迁移

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

### INK-166验收证据：章节确认与跨域事实事务

主代理于2026-09-18在实际组合根、隔离PG16和Redis7上独立复跑，限定构建及真实命令均退出0。

### 可复现命令

PowerShell：
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go build ./...
go run ./cmd/acceptance/ink166

环境由CLI先验证数据库名并执行R1迁移，凭据随机生成且不写日志。

### 通过分组

以下9组全部PASS，确认路由成功响应通过完整冻结Schema，RequireCoverage(/v1/content/chapters/{chapter_id}/confirm)==1：

- empty-confirmation：空提议仍使world revision、chapter revision和novel revision各按契约递增，不生成ledger event、payload、body version或writing event。
- confirmation-idempotency-state：原结果完整语义重放、异载荷同key冲突、已confirmed新key拒绝，且拒绝与重放无业务写。
- assert-provenance-projection：content锁定实际chapter/version/hash来源，事实事件写入后更新head、角色投影、审计、outbox与scope/history。
- chapter-world-fact-conflicts：旧chapter/world revision和整批事实冲突均返回冻结409，先前有效提议不产生部分提交。
- supersede-retract-historical-proof：assert、supersede、retract使用真实typed提议，历史event不可变，head revision与角色投影正确。
- concurrent-world-cas：真实并发确认严格一方成功，另一方world fact conflict；world与novel仅递增一次。
- world-event-fault-atomicity：World.OnEvent在真实ledger/payload/head/world/entity变更及Audit/Outbox写入后返回故障；16张表含幂等记录的规范化指纹全部回滚，原key经HTTP重试成功。
- confirmation-authorization：跨owner/tenant、角色撤销后的确认和缓存重放均拒绝，无状态写入。
- confirm-schema-coverage：确认操作单独取得完整成功Schema覆盖。

### 实现边界

content新增自己的WorldFactPort、ConfirmationSource、Confirm和typed fact value；internal/app/adapters/r1_world_fact.go显式映射到worldview的CommitFacts，不跨域读取私表。既有WorldInitializer保持兼容。

确认锁住content scope/novel/chapter后，将可信chapter/version/hash传入worldview同一UoW；worldview再次验证chapter历史proof、chapter_version revision=1、novel/hash和当前授权。确认成功不生成新的正文版本，章节状态改为confirmed并绑定当前版本，chapter/novel scope/history、audit/outbox和幂等结果原子提交。

content transport的nullable presence检查调整为：指针字段允许schema批准的null，非指针必填字段仍拒绝null；该修复覆盖target_event_id、reason和retract的value。新增content.chapter.confirmed显式audit action。

INK-163仍严格验收27个原有操作；确认操作由本任务独立覆盖，避免跨任务重复或遗漏。浏览器Chrome连接仍不可用，故本任务未宣称真实浏览器证据；HTTP/PG和架构门禁已通过。

