# Agent Note: INK-160 平台事务与API基座历史验收

Status: implemented

Task: INK-160
Scope: docs/acceptance-INK-160.md 的历史验收记录与迁移

## Problem

阶段验收记录散落在 docs 读者文档目录，与长期有效的设计规范混放。

## Decision

将原始验收记录纳入工程记忆，保留当时的运行证据、未覆盖范围和实现路径；本次只迁移文档，不声称今日再次验收通过。

## Alternatives considered

继续放在 docs 可以沿用旧路径，但会继续混淆文档职责；直接删除将损失复现证据。因此选择保留全文并迁入 notes。

## Consequences

访问路径变更，需要同步入站引用；源记录中的命令和结果需结合当前版本重新验证。

## Verification

来源为 private 分支既有验收记录，以下保持历史事实并调整相对引用。本次验证限于文档格式、路径与 Git 提交。

### 历史原始记录

### INK-160 · 平台事务与API基座验收

验收日期：2026-09-18，北京时间。任务状态唯一来源为 [backlog](../../../../docs/backlog.md)。本轮只交付INK-160，不把平台表存在当作账号、队列、文件或业务域功能完成。

### 实现与运行路径

- [R1组合根](../../../../internal/app/platform.go)：`cmd/server → app.New → config.BootstrapR1 → app.NewWithConfig`，装配UoW/scope/idempotency。目前仅挂载healthz/readyz，旧业务路由不挂载；旧构造器保留为未调用代码，旧import尚未清理。
- [迁移](../../../../migrations/r1/000001_platform.sql)逐行等于冻结platform.sql；server与cmd/migrate使用同一独立流，不混跑旧18条迁移。Up持PG事务锁并校验所有版本、名称和checksum；Status/Verify拒绝混流或漂移。Verify验证迁移元数据，不声称检测任意手工表结构篡改。
- [UoW](../../../../internal/platform/uow/uow.go)通过不透明context共享事务；嵌套失败/取消和panic不能留下半账。[Recoverable](../../../../internal/platform/uow/recoverable.go)只隔离确定性命令失败，致命故障仍阻止根提交。
- [scope](../../../../internal/platform/scope/scope.go)验证当前用户、租户、成员、会话、audience及owner；父作品→子scope固定锁序；投影CAS、历史revision/hash与码点长度证明；当前权限与软删优先于历史证明。
- [幂等](../../../../internal/platform/idempotency/executor.go)前置授权→非阻塞事务advisory锁→持锁再授权→claim/replay→命令；业务和结果同提交，成功及确定性409/422重放原封包，5xx不缓存，过期processing不抢占。
- 公共HTTP使用冻结R1封包、真实request/trace关联、精确Origin、2MiB JSON上限、未知/重复/大小写别名字段拒绝与安全错误；不替代后续领域DTO/JSON Schema校验。

### 可复现命令

需要Go、Node、Docker，以及本机已可用的 `pgvector/pgvector:pg16` 镜像。验收程序只允许连接名称以 `inklore_r1_acceptance` 开头的专用数据库，并生成随机UUID隔离样本。

~~~powershell
docker run --detach --rm --name inklore-r1-acceptance-160 --publish 127.0.0.1:55432:5432 --env POSTGRES_HOST_AUTH_METHOD=trust --env POSTGRES_DB=inklore_r1_acceptance pgvector/pgvector:pg16
$env:INKLORE_ACCEPTANCE_DSN='host=127.0.0.1 port=55432 user=postgres dbname=inklore_r1_acceptance sslmode=disable'
go build ./...
go run ./cmd/acceptance/ink160
go run ./cmd/archguard
node scripts/check-r1-platform.mjs
node docs/system-design/tools/check-design.cjs
node scripts/check-project-skills.mjs
node scripts/agent-notes.mjs check
git diff --check
docker stop inklore-r1-acceptance-160
~~~

以上trust只用于本机回环端口的临时验收容器，不作为部署配置。命令无真实密码/API Key/用户正文。验收命令通过真实PG和loopback HTTP执行，不新增任何 `*_test.go` / `*.test.ts` 文件，不开放额外产品API。

### 实测证据

| 检查 | 预期与实际 |
| --- | --- |
| 全仓构建 | `go build ./...` 退出0 |
| 综合PG/HTTP | `go run ./cmd/acceptance/ink160` 输出26组PASS，退出0；每组内含多项断言 |
| 迁移 | 空库成功创建14张平台表及pgcrypto/pg_trgm/vector；重复skip；checksum篡改在独立事务内被Up/Status/Verify拒绝并回滚 |
| 事务 | 同一backend PID/transaction ID；业务+快照回滚；吞掉嵌套错误仍回滚；foreign/closed context拒绝；取消与panic无残留 |
| 可恢复边界 | 确定性嵌套失败仅回滚局部、外层可提交；取消和被捕获panic仍阻止根提交 |
| 授权/CAS | owner通过、同租户其他人/跨租户拒绝、空主体401；双连接CAS恰一成功/一409；历史hash+长度证明；父软删拒绝子快照、同ID新revision恢复 |
| DDL反例 | 复合FK错租户、current_revision=0、会话跨租户成员、文件hash不符；核对PgError的约束名和SQLSTATE23503/23514，不以任意SQL错误冒充约束通过 |
| 幂等 | JSON键序/空白/等价数字重放一次；改载荷409；并发claim在2秒内409；异常无claim/业务残留；409/422普通回滚和显式提交409；嵌套UoW错误可缓存 |
| TTL/权限 | 过期completed原行重认领；DELETE expected_revision参与hash；过期processing不抢占；撤销当前成员后completed重放先401，不返回旧body |
| HTTP | 实际app.New启动、旧路由404、关联头接续与重建、精确CORS、非法JSON/revision400、超限413、panic500无敏感信息、SSE已发头后不追加JSON |
| 启动故障 | 先用有效配置启动成功，再独立移除IDEMPOTENCY_HASH_KEY，错误确指向该键；无PG时即使旧内存开关开启仍拒绝；关池ready503而health200 |
| 日志 | 故意构造非法DSN，CLI拒绝且不回显标记；GORM不调用SQL字符串回调、不打印参数或原始PG异常 |
| 护栏 | archguard全部通过：跨域infra/platform反向依赖/50KB/禁测试文件/Go解析；冻结设计7593引用、420链接、74表，issues/warnings均0；95错误映射逐项一致 |
| 运行依赖 | `go list -deps ./cmd/server`确认新增uow、scope、idempotency、migrations/r1可达；旧包因未删除import仍可达，不夸称清理已完成 |

SQL反例产生固定 `database operation failed` 日志属于预期拒绝，不输出SQL、正文、密钥或连接串。

### 发现及修复

初次集成构建暴露验收命令的重复短变量声明，修正后全仓编译通过。旧Bootstrap会读取.env并覆盖显式DSN；迁移流检查拒绝该旧库，未提交DDL，随后改用BootstrapR1并在独立PG完成迁移。只读审阅指出缺key用例可能因空DSN假阳性，已改为有效DSN正例后单独去key并检查错误来源。

### 启动与使用约束

R1从进程环境/挂载配置加载，**不读取旧.env**。需要真实DATABASE_DSN，四个独立且至少32字节的JWT_SIGNING_KEY、CSRF_KEY、IDEMPOTENCY_HASH_KEY、CURSOR_SIGNING_KEY，以及精确且不同的CREATOR_ORIGIN/ADMIN_ORIGIN。APP_ENV只接受development/production/test；生产必须挂载配置并先执行R1迁移，不能靠拼错环境名进入自动迁移。

UoW context同步使用、不逃逸、不并发。Authorize必须绑定实际请求tenant/actor及目标资源，不能在重放前重新做CAS；读写进入同一事务后再次验证并持锁。码点长度由内容Owner从正文计算，不信任客户端声明。人为残留processing会持续409，需诊断，不能因过期自动重复未知工作。

### 未覆盖与后续

未做真实模型调用、账号登录/refresh/CSRF/审计业务、文件和作业运行、七域业务、双端浏览器、生产部署、旧库导入或容量恢复验收；这些属于后续任务。R1源契约未改，改密幂等头与缺失错误响应差异记录于 [INK-178](../architecture/2026-09-18-r1-contract-differences-ink-178.md)，进入受影响任务前需裁决。

工程理由见 [事务与幂等笔记](../architecture/2026-09-18-r1-platform-atomic-command-boundary.md)。

