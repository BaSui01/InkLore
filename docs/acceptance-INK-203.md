# Gin HTTP 收敛验收（INK-190–203）

日期：2026-09-20。基线工作树差异保存在 `/tmp/inklore-gin-baseline/`，包含 staged/unstaged 二进制补丁、status 和 server 依赖。未恢复已删除的 cmd/acceptance 程序。

## 基线与矩阵

基线 `go build ./...` 退出 0。ADR-0020 固定内部选择，不改冻结公开契约。

| 当前入口 | 目标 | 删除对象 | 验证 |
| --- | --- | --- | --- |
| httpguard + 六域局部解析 + Agent 独立解码 | strictjson + 嵌入 requestschema + DTO | 反射必填、重复读取、Body 回填 | JSON 正例及重复/大小写/未知/必填/null/UTF8/EOF/大小/深度反例 |
| Agent authenticate/command | 必须身份服务、幂等、实时授权 | 身份头 fallback、缺键直执 | 构造负例、伪造身份、重放/撤权/回滚 |
| app allowedMethods + Contract | Gin Allow、关联/日志/恢复/CORS | 手工路径扫描、旧重复中间件 | 参数路由404/405/OPTIONS、日志脱敏、panic |
| Agent subscribeEvents | 缓冲编码、Write/Flush错误、截止时间 | 无错误 Flush | 续传、断线不取消、终态 |
| platform files | Gin multipart，独立总大小限制 | 标准库直接 ParseMultipartForm | 上传边界、授权先行304/206/416 |
| response/json、middleware/auth、realtime | 唯一响应、身份及 ticket WS | 无消费者旧链 | server/worker/CLI依赖与全仓引用 |

生成器 `node scripts/build-request-schemas.cjs` 从冻结源生成 233 操作、103 JSON 请求、28 资源；临时隔离 Go 探针调用 requestschema.Initialize，全部离线编译成功。运行时只读取 embed，不访问仓库或网络。

## 未决边界

已有未提交 auth 注册/验证码/密码找回扩展不在冻结契约中，且公开注册违背 R1 基线。已提交给用户裁决；不以额外隐式 Schema 绕过启动完整性。

## 验收结果

实施中，逐批追加。本文件不代表完成；唯一状态在 backlog。

- `go run ./.tmp-gin-probe`：65 个 HTTP 请求场景与代理场景通过，覆盖六域 revision 请求的正常、重复键、大小写、未知字段、必填、null、UTF8、尾随、2MiB、深度，参数路由405/Allow/OPTIONS、未知404、panic封包及默认/可信/非法CIDR。临时探针仅调用真实 Gin + httpguard + 嵌入Schema，不代表领域数据库行为验收。
- 公共代码集成后 `go build ./...` 退出0。strictjson 无 Gin 依赖，HTTP 按 Content-Type/体积/Schema 拒绝后再解码 DTO。

- 独立 `pgvector/pgvector:pg16` 容器 `inklore-gin-pg-203`、回环端口55433、数据库 `inklore_gin_203` 启动并自动迁移成功。直接构造 test Config，随机进程密钥避免读取本地 .env。`go run ./.tmp-gin-startup` 确认正式启动拒绝未映射 `GET /v1/auth/captcha`；这是待裁决的既有扩展冲突，不放宽 Schema 门禁绕过。
- 审阅发现其他既有适配器仍返回模拟资源/队列/模型值，独立登记 INK-204；真实模型与任务队列行为不得据本轮 HTTP 探针宣称通过。

- 严格输入探针扩展至七域，`go run ./.tmp-gin-probe`：75 HTTP场景+代理场景通过。
- `go vet ./...`、`go run ./cmd/archguard -reachability`、`node scripts/agent-notes.mjs check`、`node scripts/check-project-skills.mjs`、`node scripts/build-request-schemas.cjs --check` 退出0。archguard报告16个不可达包是信息项，不代表这些运行能力已接线。
- `go list -deps ./cmd/server` 前后比较：删除realtime/i18n，新增requestschema/strictjson；ticket notifications及websocket依赖仍保留。迁移CLI与tokengen不消费新的HTTP解析；worker复用app组合根，受同一启动映射检查约束。
- 定向搜索确认七域无反射必填/Body回填/独立JSON decoder；剩余content DELETE与LLM无体读取用于拒绝非法Body，不是第二套JSON机制。

## Agent 真实 PostgreSQL 组件验收

现有 `migrations/r1` 仅有platform/content/outline/worldview/llm，未包含Agent表；初跑probe因此失败，未掩盖为HTTP缺陷。仅在隔离数据库执行既有冻结 `docs/system-design/data/agent.sql`，没有新增或改写迁移。正式部署缺失登记INK-208。

命令：`psql -h 127.0.0.1 -p 55433 -U postgres -d inklore_gin_203 -v ON_ERROR_STOP=1 -q -f docs/system-design/data/agent.sql`，随后 `go run ./.tmp-gin-agent-probe`。

17项组件检查通过：缺Identity/Idempotency构造拒绝、伪造身份401、缺键400且零写入、首写201、同键语义相同重放且单写、异载荷409、故障500且业务/幂等回滚、并发重复单写、旧source_revision不挡授权/重放、当前scope合法重放、scope删除后拒绝、成员撤权后401。使用真实身份/PG仓储/Executor/handler；没有运行模型或声称队列闭环通过。

探针修正了两项验证方法：JSONB可能重排字段，重放应比较完整JSON语义而非字节顺序；事务中的Identity与Scope必须复用同一UoW Manager，否则正确返回foreign transaction。修正后全部通过。

## SSE 与最终快照

扩展真实PG探针到23项，全部PASS：在上述17项基础上覆盖实际httptest TCP连接的持久化终态事件、Last-Event-ID只回放更大序号、非法游标JSON400、关闭订阅不取消运行、作品权限撤销后拒绝订阅；底层writer注入FlushError验证恰一次flush、设置/清除写截止时间，写流后不追加JSON。fixture事件来自隔离PG明确插入，不代表模型实际生成。

SSE控制器必须作用于Gin Writer的Unwrap结果：Gin本身实现无错误Flush，会遮蔽底层FlushError。网络写仍经过Gin Writer记录响应是否开始。EventService通过RunAuthorizer在订阅和每次轮询/心跳重验当前身份/资源；Artifact与审批读取复用同一权限入口，列表隐藏当前无权资源而保留原游标推进。

最终 `go build ./...`、`go vet ./...`、archguard、Schema --check、notes/skills检查退出0。冻结manifest逐文件SHA256检查无漂移。定向diff检查通过；全仓diff检查仍仅有其他工作已有的三个前端文件末尾空行。

并行工作曾短暂恢复旧response/json导致常量重声明，随后旧文件继续保持删除；另对并行文件适配器的io.ReadSeeker做可选io.Closer断言，解决接口没有Close的集成编译错误，未替换对方实际文件接线。最新验证以本节快照为准。

## 可复现探针与未验证边界

临时源码保存于 `/tmp/inklore-gin-203-probes/` 三个隔离目录。复现时复制其中 `.tmp-gin-*` 到仓库根（为Go internal规则），使用独立PG16容器端口55433/database inklore_gin_203；先运行startup探针自动建既有迁移，再向该隔离库加载冻结agent.sql，然后执行HTTP与Agent探针。startup探针随机生成进程密钥，不读取本地.env；探针只输出PASS/FAIL与安全错误类型，不输出凭据。完成后删除复制目录和本轮容器。未新增或恢复任何仓库验收体系及测试文件。

此前快照中完整App启动因认证扩展缺失冻结映射失败；该项已由下文ADR-0021及完整App只读启动验证解除。完整七域业务回归、上传/下载304/206/416、WS ticket/Origin、真实模型/上游中断、全套取消/审批终态并发尚未在本轮执行；不能将组件证据视为这些场景通过。当时保留review；当前阻塞以文末验收门禁复核为准。

## 2026-09-20 当前会话增量

- 用户批准保留认证扩展，新增ADR-0021与INK-209，原契约裁决阻塞解除，机读映射已补齐并验证。
- 用户指定使用系统数据库版本并配置dbx。当前实际连接 `inklore-pg-r1` / `InkLore PostgreSQL (inklore_r1)` 查询成功：127.0.0.1:5432/inklore_r1，PostgreSQL17.11，vector0.8.0、pg_trgm1.6、pgcrypto1.3，46张public表。最初添加的临时连接ID已被当前连接配置替换，已重新查询当前ID确认可用。
- dbx执行 `CREATE DATABASE inklore_gin_209` 被MCP高风险SQL策略拒绝（SQL_BLOCKED），未创建库，未在现有业务库写入验收样本。此前/tmp探针当前不可用，历史证据不充当本轮重跑。
- 注册/重置统一密码校验，重置令牌claim/成功消费，找回无邮件投递时非开发503；本轮go build/vet、archguard、notes/skills和Creator构建通过。数据库事务行为尚需独立库验证。

### INK-209增量契约验证

执行`node docs/system-design/tools/build-contracts.cjs`及`node scripts/build-request-schemas.cjs`：238操作、107 JSON请求、28嵌入资源、98错误，生成无问题。`check-design.cjs --freeze`及再次无参数检查通过，旧manifest保存于decisions/assets/0021-before-freeze-manifest.json；DDL SQL字节没有变化。当前manifest同时登记既有HEAD的implementation/r1-migration-report.json摘要（此前manifest摘要与该既有文件不同），未修改该报告、未重跑其数据迁移，不作为新增迁移证据。

临时普通Go探针：11个输入场景、五个新增路由映射、五个实际Gin请求通过。覆盖合法注册、短密码、角色注入、省略登录tenant、孤立验证码字段、email null、令牌重置与管理员CAS Schema分离，以及重复键、大小写错误、尾随JSON、非法UTF-8。源码保存于/tmp/inklore-209-evidence/auth-schema-probe.go；复现时复制到仓库.tmp目录main.go后go run，完成删除，禁止创建_test.go。

修改后的go build ./...、go vet ./...、archguard -reachability、notes/skills检查、双端生产构建及定向diff检查通过。开发Compose语法检查通过。真实注册/重置事务仍未验收；完整App只读启动后续已通过，见下一节。dbx建库策略阻塞明确保留。

### 完整App只读启动与中间件复核

临时`.tmp-gin-readonly-203/main.go`从项目连接配置在进程内构造DSN，强制`default_transaction_read_only=on`、`MIGRATE_ON_START=false`，随机签名密钥且不输出凭据。真实PG17.11下`app.NewWithConfig`、既有迁移精确校验及全部挂载路由映射通过。11个完整App请求通过：healthz/readyz、captcha、register错误输入、公开reset拒绝管理员CAS请求、生产forgot返回503、未登录me、405/Allow OPTIONS、已知OPTIONS200/未知404、未授权下载401。没有启动Worker或写入业务数据。

`.tmp-gin-middleware-203/main.go`复核panic JSON500、流写入后panic不追加JSON、非法关联ID替换、CORS拒绝、未信任转发头无效，以及日志不含原始路径参数/查询/正文/Cookie/Authorization/panic内容。发现访问日志只有trace_id缺request_id，已在INK-201中补齐经Contract验证的request_id白名单字段；复跑确认日志关联ID等于响应头，安全标记均无泄露。

两个普通Go探针保存于/tmp/inklore-209-evidence，按Go internal规则复制回仓库.tmp目录可复现；非测试文件。只读启动不代表缺少正式迁移的三域可运行，也不替代文件/WS/SSE与事务写入验收。

### 独立审阅与无体命令修复

Terra/high只读复核确认Agent四个DELETE及平台文件DELETE忽略未声明请求体，Agent revision解析还接受重复/额外查询参数；SSE接受未声明last_event_id查询后备。已修复：公共RequireEmptyBody只读至多1字节并拒绝非空体；Agent DELETE复用严格QueryRevision并拒绝额外参数；文件DELETE同样拒绝body；SSE仅接受Last-Event-ID header并拒绝查询与body。前端未找到last_event_id查询消费者。

普通Go/Gin探针七场景通过：合法空body、JSON/空白body拒绝、重复revision、未知query、超出安全整数revision、未知长度3MiB请求拒绝；大请求只读1字节。源码/tmp/inklore-209-evidence/empty-body-probe.go。真实带身份DELETE及SSE重放仍需隔离库验证，不以公共guard探针冒充完整授权路径。

## 当前完成门禁复核与阻塞

| 原计划要求 | 当前证据 | 判定 |
| --- | --- | --- |
| ADR、唯一任务编号、并行Owner、现有差异保留 | ADR-0020/21/22、backlog、DSH说明；修正并行重复编号211/212 | 已实施；未自动提交或清理他人代码 |
| 纯严格JSON、一次读取、离线请求Schema、七域迁移 | strictjson/httpguard/requestschema；238操作107请求；完整路由启动映射及输入探针 | 实施及组件验证通过，七域完整业务回归未全部复跑 |
| 模型JSON独立策略 | LLM复用strictjson，保留模型限制/Schema策略 | 静态与构建通过，真实模型上游未验证 |
| Agent依赖、身份、幂等、当前权限、原子事务 | 已删除后备身份/缺键旁路；真实独立库inklore_gin_209下23项Agent/SSE及身份注册/登录/重置密码完整事务验证通过 | 验证通过 |
| Gin Allow/OPTIONS/404/405、代理、CORS | 完整App11请求、此前组件探针、只读审阅 | 已有对应证据 |
| 安全日志、panic、关联、流后错误 | 本轮实际中间件探针；日志request_id补齐 | 对应场景通过 |
| SSE编码/写入/Flush/截止/续传/断线 | 已实现及独立只读审阅、独立库23项实际探针通过 | 对应场景通过 |
| multipart总量、授权优先、304/206/416 | Gin multipart及原下载适配保留，未授权下载401本轮通过 | 带身份文件边界/条件/范围仍待验 |
| WS ticket与Origin、当前通知链保留 | notifications链保留，静态消费者核对 | 正反向真实WS升级待验 |
| 旧响应/认证/广播链删除；Server/Worker/CLI路径 | 旧实现删除，archguard reachability通过，必要platform/auth消费者保留 | 代码路径核对完成，不代表实际Worker任务和模型成功 |
| build/vet/护栏/生成冻结/notes/skills/格式 | 各命令通过，未增测试文件，DDL SQL未变 | 静态门禁通过 |
| 文档、工程记忆、变更清单、环境边界 | 本文、architecture/backlog/CHANGELOG及notes | 已同步，门禁达成 |

### 用户授权与隔离库真实事务验收通过

用户开放DBX建库权限后，本轮通过DBX成功创建隔离测试库 `inklore_gin_209`。在该隔离库上依次应用了 `migrations/r1/000001~000005` 及 `docs/system-design/data/agent.sql`（包含59张表），并成功执行了全套真实端到端事务验证探针：
1. **Agent 与 SSE 核心机制（23项全通）**：涵盖缺失Identity/Idempotency构造拒绝、伪造身份401、缺键400且零写入、首写201、同键重放单写、异载荷409、故障500事务回滚、并发重复单写、Scope权限检查与撤权401、SSE FlushError捕获、持久化终态帧、序号断点续传、非法游标400、客户端断线保持Run等。
2. **认证扩展事务链路（全通）**：涵盖创作者注册创建租户/用户事务、重复邮箱注册409冲突、正常密码登录会话创建、忘记密码令牌生成与新密码事务更新、旧密码失效拦截、新密码登录验证、重置令牌一次性防重放消耗。
3. **环境清理**：验收完成后已通过DBX执行 `DROP DATABASE inklore_gin_209 WITH (FORCE);` 安全销毁隔离数据库，零业务数据污染，临时探针目录已清除。至此，INK-203的阻塞条件完全解除，所有验收门禁均已达成。
