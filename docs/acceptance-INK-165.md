# INK-165验收证据：worldview静态图谱与事实账本

主代理于2026-09-18在实际装配后独立复跑，下述 16 组退出0；25 个已挂载操作逐个取得成功响应并通过完整冻结Schema。子代理编译结果不替代此证据。

## 环境与复现

隔离 pgvector/PG16：127.0.0.1:55432，数据库 inklore_r1_acceptance_161；Redis7：127.0.0.1:56379。CLI先验证数据库名再迁移，随机生成身份和签名密钥，不打印凭据。

```powershell
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go run ./cmd/acceptance/ink165
```

## 已复现证据

content_source_world_initial、characters_lore_cas_static、relations_graph_lifecycle、facts_check_commit_batch、facts_supersede_retract、fact_world_cas_and_outer_rollback、digest_immutable_triggers、audit_failure_rollback、current_permission_replay、scope_and_deleted_replay、maintenance_current_authorization、frozen_25_operation_schema_coverage。

真实content版本DTO及历史proof构造来源，在同UoW调用CommitFacts；无伪造source投影。验证整批冲突无部分写、同world修订并发仅一个成功、空批次world+1而不追加event、外层错误全状态回滚。独立重算payload digest，真实PG UPDATE/DELETE ledger触发器拒绝。审计故障回滚，同key重试；旧预检key跳过旧版本重验但仍检查当前权限，撤权/删书拒绝重放。维护cap撤销与缺失payload拒绝，污染投影可从不可变账本重建。

## 30天擦除补充证据

主代理独立复跑新增四组均PASS：retention_entity_capability_and_deadline、retention_expired_entity_static_erasure、retention_novel_capability_and_deadline、retention_novel_payload_erasure_and_immutable_proof。

缺少/撤销当前维护授权拒绝；未满30天完全不写。仅本次tenant指定tombstone回溯31天的夹具到期后，单实体擦静态文字并保留同ID及账本/关系/head引用，重复清理无写。整书擦除通过当前维护callback在同UoW锁定真实novel投影并验证期限；事件故障全擦除回滚。成功后entities/relations/heads/payload及盐清空，ledger各行完全一致，事件返回payload_erased且敏感字段null，原账本UPDATE/DELETE触发器继续拒绝，擦后重建拒绝且不写。

## 集成与范围

CheckFacts采用最外层REPEATABLE READ，同时持久技术幂等缓存；预检本身不修改领域事实，后续CommitFacts重新检查。维护没有新增公开HTTP入口，App未配置维护授权时失败关闭。事实确认由166可信content适配；生产维护调度及最大容量性能仍待对应后续门禁。

公共R1迁移000002_content、000003_outline、000004_worldview原样复制冻结DDL，已在真实PG应用并验证checksum。go build ./...、archguard、97错误映射与四份DDL字节校验、冻结设计检查通过；INK-160的26组和INK-161的24组回归通过。go list -deps ./cmd/server确认对应四层包已进入运行路径；旧import仍可达，不宣称旧源码清理。

独立验收脚本位于 cmd/acceptance/ink165；公共harness按METHOD+template记录真实成功覆盖，不能仅凭路径或静态路由计数通过。当前契约仍为ADR-0018批准后的R1.1；ADR-0019的304仅为未批准提案。
