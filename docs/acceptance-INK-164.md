# INK-164验收证据：outline三级大纲、场景、伏笔与节奏

主代理于2026-09-18在实际装配后独立复跑，下述 10 组退出0；29 个已挂载操作逐个取得成功响应并通过完整冻结Schema。子代理编译结果不替代此证据。

## 环境与复现

隔离 pgvector/PG16：127.0.0.1:55432，数据库 inklore_r1_acceptance_161；Redis7：127.0.0.1:56379。CLI先验证数据库名再迁移，随机生成身份和签名密钥，不打印凭据。

```powershell
$env:INKLORE_ACCEPTANCE_DSN="host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance_161 sslmode=disable"
go run ./cmd/acceptance/ink164
```

## 已复现证据

content_fixture、hierarchy_hash、ordering_rebind、beats_scenes_history、clue_state_events、pacing、authorization_cursor、rollback、schema_live_29、schema_coverage。

使用真实content作品/卷/章。独立固定键序JCS校验<>&、U+2028/U+2029、CRLF及非BMP的hash与完整码点长度；重排核对每个受影响资源和master修订。真实移卷后重绑定；beat/scene历史锚点、线索完整状态链与5条事件、张力均值/连续高低/null/超期/历史倒挂均复现。tenant限定outbox故障使master、clue、事件、历史投影、outbox和幂等全部回滚，同key恢复可重试。

## 集成与范围

Pacing/CaptureSnapshot使用明确REPEATABLE READ READ ONLY根事务；当前身份与scope在该上下文执行无锁读取，数据库拒绝写入。ChapterContext在internal/app/adapters读取content元数据。批量大作品性能和后台retention调度未宣称通过，留后续容量/调度集成。

公共R1迁移000002_content、000003_outline、000004_worldview原样复制冻结DDL，已在真实PG应用并验证checksum。go build ./...、archguard、97错误映射与四份DDL字节校验、冻结设计检查通过；INK-160的26组和INK-161的24组回归通过。go list -deps ./cmd/server确认对应四层包已进入运行路径；旧import仍可达，不宣称旧源码清理。

独立验收脚本位于 cmd/acceptance/ink164；公共harness按METHOD+template记录真实成功覆盖，不能仅凭路径或静态路由计数通过。当前契约仍为ADR-0018批准后的R1.1；ADR-0019的304仅为未批准提案。
