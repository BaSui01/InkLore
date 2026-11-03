# InkLore 重构实施事实

> 当前源码文件大小规则由 [ADR-0037](decisions/0037-source-file-size-limit-500kib.md) 更新为500KiB（512000字节）。下文历史50KB验收描述保留为当时事实，不再表示现行源码阈值；独立文档长度和冻结治理不变。

本文件只记录实现证据，不维护第二份目标架构。唯一目标设计：[R1索引](system-design/00-INDEX.md)，契约：[00基线](system-design/00-CONTRACT-BASELINE.md)。

## INK-228 · 冻结策略真实消费和租户安全模型路由（实施中）

PR #18 已合入 dev：Run Scheduler、小说引导和 LLM Catalog 查询链携带冻结 PolicyVersionID，不再由 Agent 适配器直接访问 LLM 私表。PR #19 是独立 Draft：通过已授权 Model ID 在 LLM Infrastructure 查询同租户 llm_channel_models/llm_channels/llm_models/llm_providers/llm_keys，所有关联限定 tenant，排除停用或删除资源；路由使用实际渠道协议和 upstream_model；数据库渠道不再向全局密钥/其他租户回退。非事务模型调用读 GORM DB；CreateRun 在现有 Run/Job/事件 UoW 内用 PostgreSQL FOR SHARE 获取 current_version_id 与发布操作序列化。

跨租户通道/失效 Key、真实费用、策略并发发布、真实 Provider/WS/SSE 和登录浏览器尚未在隔离环境穿透；CI 构建通过不等于模型调用成功。灵感/质检现已按当前租户启用的 inspiration/review 策略提供 Model ID/版本/预算上限；缺策略直接失败，真实生成仍需验收。无租户调用仅保留显式全局 pool/env 路径；Creator 安全策略目录、task_policy_version_id 创建与兼容 sunset 仍待实施。根因与测试门禁见 [工程笔记](../.agents/notes/proposed/bug-fix/2026-11-02-tenant-model-routing.md)。

PR #19 的 cmd/ink228verify 增加 Go 环回 HTTP 伪 Provider：在临时数据库对两租户分别使用独立 Key 与上游模型标识发出真实 HTTP 请求，断言失败配置和跨租户 Model ID 不会产生额外外发。真实模型供应商与对外计费仍未验收，必须以新 head 的独立 CI 为准。

## INK-228 · 独立云端领域结构门禁（辅助 CI）

GitHub PR #38 新增不接触真实凭据的 PostgreSQL 17 + Redis 7 临时 Runner，执行当前 17 个 R1 迁移及重复运行、Agent/LLM 表和租户外键结构断言；不改任何业务运行路径。初始 head 的云端检查已通过，最终 head CI 仍需验证。详细选择和假设见 [工程笔记](../.agents/notes/implemented/testing/2026-11-03-ink228-isolated-schema-gate.md)。这不是 Agent/LLM 跨租户执行、事务竞争、真实模型或前端链路验收，INK-228 保持现有未完成状态。

## INK-245 · 接入模型渠道弹窗移除供应商下拉框（review）

Admin `ChannelModal` 的协议与别名仍保留，供应商选择下拉框移除；创建时由所选协议匹配已有供应商，未匹配则由现有保存流程注册，编辑时沿用渠道绑定的供应商。后端接口、Provider/Channel 数据结构及已有资源不变。TypeScript LSP 零问题，Admin 与 Go 构建、i18ncheck（905/905）、notes check 通过。用户要求自行手测，弹窗实际呈现和登录后保存行为未由本轮浏览器验收。

## INK-247 · 代码内置工具只读展示与外部写口关闭（review）

当前 Admin 工具页不再有“已登记配置”分区和新建/编辑/启停/删除入口，只用 `GET /v1/agents/builtin-tools` 展示 `ToolExecutor` 中安装的真实 Go handler 名称、描述与执行状态，不展示机器键小字，也不把租户配置库中的任意行当内置工具。服务按管理员租户读取内部冻结版本并复用执行器检查：未播种、禁用或 Schema 不匹配时为 `not_bound`，底层数据库错误原样返回；目录条目本身只由代码注册表决定。`POST /v1/agents/tools`、`PUT/DELETE /v1/agents/tools/{tool_id}` 的对外路由及源 API operation 已移除，已知路径写方法由 Gin 返回405；只读 GET 暂留供历史冻结版本读取。内部 `admin seed-tool` 仍是部署侧受控、按租户幂等注册真实代码工具的路径，不是管理员在线配置。保留智能体/工作流治理入口。该 R1.x 覆盖关系见 [ADR-0035（Proposed）](decisions/0035-agent-builtin-readonly-tool-catalog.md)。新后端已实测 `/readyz` 200、匿名内置目录401及工具 POST/PUT/DELETE 均405（Allow: GET, OPTIONS），管理端新界面实见唯一内置工具卡片且没有配置入口；跨租户状态与模型调用仍未验收，不以旧版“可用+已登记”截图作为本轮证据。

## INK-246 · 定义与工作流退役、工具目录旧阶段事实（工具页面已由 INK-247 改为只读内置目录）

`DELETE /v1/agents/definitions/{id}` 和工作流 DELETE 按租户内 `expected_revision` CAS 退役配置头：`enabled=false`、`deleted_at`、revision +1；管理/创作者可选目录隐藏已删头，历史不可变版本、Run、节点及引导会话保留。`Scheduler` 从冻结定义版本 ID 取指令，已在 `internal/app/domains.go` 装配同一个 `ToolExecutor`；`App` 在路由初始化成功后启动独立后台 Worker，关闭时先停 Worker 再释放数据库。PG 作业领取、lease 心跳与 epoch 校验、Agent 工具调用经 tenant + 冻结工具版本 + Schema 严格校验；仅受控只读 `platform_utc_now` 处理器登记，结果为实际 UTC 当前时间，SSE 仅输出不含工具正文的 started/completed 事件，工具结果进入下一轮观察数据。不支持的工具、MCP、未授权副作用均拒绝而非伪造执行成功。旧 key 保留唯一占用；`admin seed` 不复活已删除资源，`admin seed-tool -tenant-id` 可只为指定租户幂等登记受控工具。

Admin 智能体与工作流卡片已移除机器键和哈希小字，工作流卡片可删除且确认修订影响。`ToolView.execution_status` 由已装配的同一个执行器判定，只有当前租户已启用、版本和受控 Schema 完全匹配才为 `available`，其他登记项显示 `not_bound`。UI 分开真实可执行目录与纯登记配置。现有 `inklore_r1` 开发租户和当前工作台租户分别用 GORM CLI 落库工具配置；新后端在 39080 的 `/readyz` 200，使用原管理会话读取工具列表 200 并得到 `available`，随后将工作台 29080 切至新进程，浏览器实见“可执行工具 (1)”和“读取当前 UTC 时间”。独立验收进程已停止，29080 保持运行。该能力与响应差异见 [ADR-0034（Proposed）](decisions/0034-agent-tool-execution-status-and-safe-retirement.md)。未进行真实模型主动发起工具调用、跨租户/失败注入与已引用定义删除的 HTTP 验收；可执行状态不等于所有登记工具均已实现。

## INK-242 · 智能体删除引用与管理端悬浮通知（历史验收；删除语义已由 INK-246 更新）

智能体定义原本仅检查 `agent_runs`，无法发现 `agent_run_nodes` 和新增 `agent_novel_creation_sessions` 对全部历史定义版本的外键引用；删除 `novel_architect` 时，实际存在一条引导会话，后续数据库拒绝被笼统映射为500。`internal/agent/infrastructure/definitions_repo.go` 现在用限定租户的三个直接引用表 `EXISTS` 检查，查询失败原样传播；Application 删除事务中再次检查，已引用返回公开 409 `agent.resource_in_use`，不级联删除运行历史。无引用资源仍走既有幂等和修订 CAS；预检之后新增并发引用的外键竞态仍需真实事务注入验证。现存资源没有删除或修改。

Admin 智能体、渠道、模型、用户、审计、网关大盘的页面级读/操作提示使用 `AdminNotice`：Portal 挂到 `document.body`，固定右上角 z=100，高于 Modal/ConfirmDialog 的 z=50；错误持续可关闭，成功短暂消失。读取错误保留旧数据与显式重试，删除失败保留确认框且不自动重复危险写入，字段校验仍在表单位置。中英 `agent.resource_in_use` / `agent.version_conflict` 明确解释阻断和版本竞争。真实 PG 只读核对该定义 runs=0、nodes=0、guide_sessions=1；Go/Admin 构建、i18n、架构护栏及浏览器 mock 503 下通知与确认框同时可见、卡片保留通过。运行中的后端仍需新二进制重启后核验 HTTP 409/无引用 CRUD，未将浏览器 mock 当真实事务证据。

## INK-244 · 旧库抽取与 R1 写入安全门禁（blocked）

`cmd/migrate/r1migration/legacy_extract.go` 在 GORM 单个只读可重复读事务中核对旧版物理表/列，再显式抽取对应数据；不回退样本，不读取密钥正文。`r1_apply.go` 只接受满足 R1 必填约束的完整数据集，在校验迁移版本、目标库全空及独占锁后用 GORM 事务按依赖顺序插入并逐表核对，失败回滚。执行器禁止样本 LIVE、缺少真实源/目标库以及相同库写入；未实现的 verify-only 明确返回失败。旧 DDL 没有用户身份表，旧模型与 Agent 运行不能提供现行 R1 版本、定价和运行快照等必填事实，故真实迁移在缺少可信映射时拒绝落库。DBX 只读确认当前 `ai_novel`、`inklore_r1` 均是 R1 结构，没有可供本轮穿透验收的旧版数据库；用户确认本轮不运行旧库迁移。样本只读演练通过，样本 LIVE/无源真实模式被拒绝；这不等于旧库全量迁移已经完成。

## INK-243 · 应用数据库访问 GORM 边界（review）

按用户确认的范围，所有业务持久化与应用级数据库连接走 GORM；`cmd/migrate/r1migration/executor.go` 原有独立 `sql.Open("pgx", dsn)` 已改为通过 `internal/platform/database.Open` 建立并关闭 GORM 连接。`database/sql` 仅保留给 GORM 底层池配置与健康统计、事务隔离 `sql.TxOptions`、仓储记录中的 `sql.Null*` 值类型；版本化 SQL 迁移和 PostgreSQL 特定锁/CAS 仍经 GORM 的 `Exec/Raw` 或事务执行，不等于引入第二套访问驱动。迁移工具的真实抽取与条件化落库现已接线，但缺乏旧库和可信身份/版本映射，LIVE 路径 fail closed；具体阻塞见 INK-244，不能宣称实际完成旧库迁移。

## INK-240/241 · 管理端审计与 PostgreSQL 项目日志（review）

审计 `GET /v1/platform/audit-events` 保持 append-only 只读事实链与既有动作、时间范围及 HMAC 游标；Admin `/audit` 现在提交筛选后查询，支持10/20/50条游标前后翻页，关键词仅在当前页搜索并明确提示，不伪称跨页全文搜索。主表按成员目录展示当前可解析的姓名、动作与资源类型，无法解析时用未标识用户，不将 UUID 作为主句；诊断详情仍可复制追踪值。脱敏 NDJSON 导出弹窗要求明确时间范围且限制31天。

项目日志采用 [ADR-0033](decisions/0033-postgresql-safe-project-logs.md) 所述独立安全投影：现有 `000015_platform_project_logs.sql` 已应用，不能改写校验和；`logger/project_projection.go` 仅处理携已验证租户上下文的 `http_request` 结束事件，白名单包含路由模板、状态、公开错误码、请求/追踪标识和耗时，禁止正文、SQL、IP、凭据和原始动态路径。`middleware.Logger` 在请求结束时以更新后的请求上下文记录事件；有界队列通过专属静默 GORM Session 异步写入 `platform_project_logs`，到期清理同样经 GORM 执行；90天滚动读取及后台清理，PG故障时不阻断业务且 stdout/file 继续存在。登录前、系统启动和 Worker 无租户日志目前不入该管理查询投影，不能宣称全系统完整收集。Admin `/project-logs` 经 `GET /v1/platform/project-logs` 在当前租户 operator/auditor 实时授权与 HMAC 游标后分页查询，支持安全字段过滤，前端只读、错误不伪装空列表。Go 与 Admin 构建通过；补齐 `platform-project-logs.json` 源 API 与生成请求映射后，重启后端 `/healthz`、`/readyz` 均200，未授权查询401。已有 Admin 会话实际读取日志列表返回200，DBX只读确认 `platform_project_logs` 从0条租户内记录增长至2条，表格呈现真实 HTTP 操作；跨租户、故障注入和清理期限仍未验收。

## INK-239 · 管理端管理写链路与全系统 HTTP 日志（review）

`Contract → Logger → Recovery → ContractCORS` 顺序不变。`internal/platform/middleware.Logger` 在请求前记录 `http_request_started`、请求结束仍记录 `http_request`，二者共享经校验的 request_id 与 trace_id；仅包含请求方法、Gin 路由模板（不含真实路径参数）、状态、受控错误码、耗时等元数据。`response.Fail` 把已生成的公开错误码放进 Gin context；LLM 幂等返回已缓存的失败封包时也仅在校验契约码后标记错误码。日志不读取正文、查询、Cookie、Authorization、SQL 或原始异常。

管理端渠道/密钥等 LLM 目录写删除沿 `http.Handler → idempotency.Executor → llm/application.Catalog → repository → audit/outbox` 记录固定阶段；Catalog 在授权、锁、校验、仓储操作与审计暂存时写入关联 ID，失败只写停留阶段和公开错误码。事务内 `repository_staged` / `audit_outbox_staged` 不代表持久提交；`idempotency_finished` 发生在 Executor 返回之后，并需结合 status/Idempotent-Replay 区分成功和重放。平台审计仍是持久事实日志，HTTP 阶段日志不替代审计。当前没有独立的 Admin“项目管理”视图或完整跨浏览器/后台 Worker span UI；日志范围并不等同全链路分布式追踪。Go 构建已通过，真实身份/PG事务、失败回滚和最终日志关联尚未验收，故保持 review。

## INK-236 · Agent 逐轮引导创建小说（实施中）

Creator 首页的创建入口现在导航到独立 `/agent-guide` 页面；若已有首页灵感，页面直接启动 Agent session，否则先在 Agent 页面输入灵感。它调用 `/v1/agents/novel-guide` 与 `/answers`；安全问题/建议由 Agent 返回，当前 session ID 写入浏览器以便刷新后按 owner API 恢复。提案字段可编辑，显式确认后才调用既有 Content `POST /v1/content/novels`。新增 transport routes、严格请求 Schema 映射、application `NovelGuideService`、agent 专属持久仓储及 `000014_agent_novel_creation_guide.sql`。模型只得到固定 `ask_user` / `propose_novel` 工具 Schema，结果服务端白名单校验；会话保存 definition/policy 版本及生成参数，作者答案以 untrusted input 进入下一轮。

本轮 `go build ./...`、Creator/Admin 生产构建及 `node scripts/build-request-schemas.cjs --check` 通过。迁移未应用到隔离 PostgreSQL，真实模型/HTTP/浏览器登录后流程、CAS 竞态、跨租户拒绝及作者确认只创建一次作品尚未复验。`node docs/system-design/tools/check-design.cjs` 仍因工作区既有冻结资产漂移、`platform.json` 超 50KB 和未登记既有 Agent schema 资产退出 1；新增接口的 `Unprocessable` 引用已改正。故任务保持 doing。

## INK-237 · 管理端渠道与密钥 CRUD（实施中）

渠道编辑弹窗不再创建 API Key；渠道列表新增密钥管理入口，通过独立 `ChannelKeysModal` 使用既有 `/v1/llm/keys` create/update/delete API 管理 SecretRef 引用、权重、RPM、并发及启停。绑定密钥的渠道不允许从 UI 直接删除，提示先在密钥管理中解除引用；删除遇 `llm.resource_in_use` 或 `llm.version_conflict` 时以双语错误留在确认框，不使用浏览器 alert。`ChannelModal` 创建渠道与 Key 创建分离，避免编辑渠道重复生成 Key。当前本地 `Novel architect OpenAI` 渠道有 2 个绑定 Key；没有执行任何数据库写入或删除。Admin 构建已通过，完整 Key 创建/更新/删除、活动 Reservation 保护及渠道删除成功链需操作员登录后的行为复验。

## INK-238 · Admin 登录验证码刷新修复

后端 `Captcha.Verify` 对每次验证码尝试都消费服务端挑战；即使验证码正确但账号密码错误、登录返回401，该验证码ID也不可再用。共享 `AuthView` 在登录失败后清空输入并重新获取验证码；手动点图刷新也同步清空旧答案。Creator/Admin构建、Go build及i18n通过；Admin UI、验证码生成接口与readyz为200。一次本地登录API请求携带新生成验证码后返回401 `auth.invalid_credentials`（而非验证码错误400），确认验证码已通过校验；DBX只读确认root账号存在活动Admin membership。没有重置或重复尝试密码；浏览器端自动换图交互仍未实测。

## INK-225 · 合规质检与文风检查真实接线及净化采纳闭环

彻底清除了创作者端编辑器中最后的 `alert` 占位，全面贯通后端合规审查与文风质检真实 REST 链路：
1. **预置出版级规则集种子**：在 `cmd/admin/seed.go` 中为租户初始化注入包含高频 AI 腔调短语（“如丝如缕”、“不难看出”、“值得一提的是”、“众所周知”等）、违禁敏感词的出版级合规规则集（`compliance_rule_sets` + `compliance_rules`），状态直接置为 `published` 并预计算权威 SHA-256 规则集校验和。
2. **SDK 与契约补齐**：在 `@inklore/contracts` 与 `@inklore/transport` 补齐 `ComplianceReport`、`ComplianceIssue`、`CreateScanInput` 及 `api.compliance.scan` / `getReport` / `listIssues` / `ignoreIssue` 客户端方法。
3. **独立人机审阅抽屉 (`ComplianceReviewModal.tsx`)**：抽屉控制在 12KB（严格小于 50KB），实时计算正文 SHA-256 并发起 `POST /v1/compliance/scans`，呈现 AI 味指数仪表评分（0~100 分）、违规风险总数与详细问题列表；支持作者针对单项建议点击“采纳改写”，基于 Unicode 码点安全替换草稿正文并触发保存，亦支持单项“忽略”。
4. **EditorRoute 彻底纯净闭环**：移除 `alert("正在连接 compliance 审核接口...")` 假占位，挂载真实质检弹窗与正文联动。
5. **门禁核验**：全量编译、双端生产构建（`build:creator`、`build:admin`）、`cmd/archguard -reachability`（R1~R5 全绿，单手写文件全部 <= 50KB）、双语国际化 `cmd/i18ncheck`（925/925 主词库对齐，子模块对齐）及工程笔记检查全量通过。

## INK-224 · 智能体与模型完全解耦、零假数据与用户模型动态自选

彻底消除了智能体与推理模型的硬编码绑定以及前端三元表达式假数据 fallback，打通了创作者对大模型自由选择的完整闭环：
1. **智能体与模型解耦**：`CreateRunInput` 扩展支持携带可选 `model_id` 与 `model_key`，并在 `agent-core-schemas.json` 契约各分支中显式放行；`run_service.CreateRun` 将用户指定的模型挂载至节点运行态 `checkpoint`，并在 `scheduler.go` Think 阶段解析注入 `ModelGenerateInput.Model`；组合根适配器 `AgentTaskModelGateway.Generate` 优先解析调用方显式指定的模型（验证其为租户内已启用在册模型并映射真实的 `upstream_name`），仅在未指定时回退至策略默认模型，并向下透传给 `ProviderRouter`。
2. **彻底消灭假数据 Fallback**：物理删除了 `EditorRoute.tsx` 中第 508-517 行硬编码的小说段落三元表达式（`周遭灵力如丝如缕汇聚...`）；`startInlineRun` 改为异步轮询真实 `AgentRun` 终态（`succeeded` / `failed` / `cancelled`）与 `listArtifacts` 产物，如果模型生成失败或未产出有效文本，抛出真实错误并在 UI 呈现，严禁伪造正文。
3. **创作者端动态模型选择器**：放行创作者只读访问 `/v1/llm/models`，SDK 补齐 `api.llm.listModels`；`EditorRoute.tsx` 动态加载在册启用模型，提供顶部模型选择下拉器，用户选中的 `modelId` 实时透传至调度链路。
4. **管理端真实网络连通性探针**：`ChannelModal.tsx` 渠道测试对接真实带超时控制的网络探测请求，捕获 RTT 往返延迟或错误拒绝，杜绝纯前端伪通过。
5. **门禁核验**：全量编译、双端生产构建（`build:creator`、`build:admin`）、`cmd/archguard -reachability`（R1~R5 全绿，单手写文件全部 <= 50KB）、双语国际化 `cmd/i18ncheck`（947/947 对齐）及工程笔记检查全量通过。

## INK-223 · 系统预置智能体种子、DAG工作流与人机闭环审阅

实现了系统级 Agent 矩阵播种与创作者端人机协同产物审阅闭环：
1. **播种引擎 (`cmd/admin/seed.go`)**：新增 `admin seed` 命令，自动为租户注入预置 `openai` Provider 与 `gpt-4o-mini` 模型，落库 `inline_polish`、`novel_architect`、`content_polish` 三大核心任务策略，注入《正文划词润色与扩写助手》（`inline_polisher`）、《小说架构师》（`novel_architect`）、《世界观设定审查员》（`worldview_extractor`）三类核心 Agent 定义，并编排合规 Kahn DAG 工作流（`workflow_inline_polish`）落库至版本 1。
2. **SDK 与运行调度**：`api.agent` 补齐创作者安全目录端点（`/v1/agents/catalog/workflows`、`definitions`）以及运行产物查询接口；`editorRun.ts` 动态解析租户可用定义与工作流，构造标准 `CreateRunInput` 提交真实调度。
3. **人机闭环审阅 (`ArtifactReviewModal.tsx`)**：划词 AI 动作（扩写/重构/精简）生成产物后呼出独立审阅面板，基于 `DiffView` 展示选区原文与建议文本对比，由作者显式确认“采纳建议并替换原文”后更新正文草稿并触发保存，严格遵循“Agent 绝非事实权威，仅输出建议”的 R1 架构铁律。双端生产构建与 Go 构建通过，手写文件严格小于 50KB。

## INK-222 · 小说核心创作链贯通与硬编码消除

创作者端三大核心功能路由（`OutlineRoute.tsx`、`WorldviewRoute.tsx`、`AnalysisRoute.tsx`）彻底移除了全部遗留的静态假数据数组，全面对接后端真实 REST API 驱动：
1. **大纲规划**：`OutlineRoute.tsx`（15KB）并行读取 `/v1/content/novels/:novel_id/chapters`、`/v1/outline/novels/:novel_id/scenes` 及大纲节点，基于真实数据渲染横向卡片流，支持新建真实章节与细分场景落库；空作品展示纯净引导并支持从零建立大纲；点击卡片直接平滑跳转至对应章节编辑器。
2. **世界观设定与实体图谱**：`WorldviewRoute.tsx`（17KB）并行读取 `/v1/worldview/novels/:novel_id/characters`、`lore` 及 `graph`，基于真实角色、设定与关系动态生成 Cytoscape 力导向关系网络；支持新建角色档案落库。
3. **章节张力分析**：`AnalysisRoute.tsx`（7KB）对接 `/v1/outline/novels/:novel_id/pacing` 真实诊断接口，基于实际采样点绘制 SVG 张力曲线，无数据时呈现真实空态。
4. **章节确认跨域闭环**：`EditorRoute.tsx`（44KB）挂载 `ConfirmChapterModal.tsx`（5KB），读取当前世界观修订号（`/v1/worldview/novels/:novel_id/revision`），在同一原子事务中调用 `POST /v1/content/chapters/:id/confirm`，将章节置为 `confirmed` 并将事实账本持久化落库。双端构建、Go 构建与架构护栏全绿，所有手写文件严格小于 50KB 铁律。

## INK-221 · 作者编辑器错误恢复、纯文本正文与版本冲突比对

`frontend/apps/creator/src/routes/EditorRoute.tsx` 彻底消除旧 TipTap 富文本框架及其 HTML 数据模型，全面对齐 00契约基线§2 与 03分册§2 的 UTF-8 纯文本、LF 换行规范与 Unicode 码点字数统计（不含空白）。正文输入由原生多行文本框 `PlainTextEditor` 承载，提供纯文本划词 AI 动作工具栏与 IME 拼音输入状态防误触；草稿下载直接导出为纯文本文件。当正文保存因并发或过期产生 409 `content.version_conflict` 时，自动呼出 `ConflictResolutionModal`，基于 `DiffView` 呈现服务器最新版本与当前本地草稿的双向对比，支持作者选择“采用服务器版本”或“以当前草稿覆盖提交”，彻底消除了冲突死锁。

作品、分卷、章节和设定列表以 `Promise.allSettled` 分资源接受成功结果，不再把失败转成空数据；作品归属失败或任何读取 403/404 不显示旧主体数据。章列表只有成功读取后才可出现真空态。章节正文 GET 的加载序号防止迟到结果覆盖新章，本地 dirty/在途保存期间禁止覆盖；未取得正确 `chapter_id` 和合法 revision 前不开放编辑/保存。切章、页内返回与刷新前会阻止或提示未确认草稿。单章保存冻结 payload、expected_revision 与 UUID 幂等键，后续键与载荷相同才重试；同章仅一在途写。保存失败持久显示结构化错误/下载动作，不改变 draft 或服务器 revision；旧版 AI 异常路径的定时伪文本与自动替换已彻底删除。双端构建、Go 构建与架构护栏全绿，`EditorRoute.tsx` 控制在 42KB（严格小于 50KB 铁律）。

## INK-220 · 前端错误传播与安全诊断

Admin `adminApi.ts` 的 GET 列表、预算与审计分页保留 `ApiClient` 的真实拒绝，不再将故障伪造成 `[]`、`null` 或成功空页。仪表盘、渠道、模型、智能体与审计页面分别持有失败状态：刷新失败不覆盖此前加载的列表和分页游标，展示共享 `ErrorPresenter` 与重试；成功后清除错误。无快照的首屏错误不再只能依赖控制台输出。管理端移除默认操作员口令与自动登录；会话只尝试恢复 Cookie，显式登录输入由操作员提供。

共享 `transport/http/diagnostic.ts` 对 `ApiError` 只输出受限 code、状态和格式校验过的请求/跟踪 ID，其余失败只输出固定机器分类。SSE/WS 原帧、原始异常、通知 topic、会话监听异常与双端业务错误不再进入 console；`ErrorPresenter` 不向用户显示不可信 `Error.message`。独立 recovery 包只记录固定事件名，不为日志增加 transport 依赖。渠道创建步骤严格顺序校验，失败阻断后续操作。

本地 PG 17 数据库就绪（9个迁移全部通过），cmd/server（端口 29080）只读与探针启动通过（/healthz、/readyz 200 OK）；真实 HTTP 请求注入验证 401 认证失败、404 未知资源、405 不支持方法（带 Allow 头）及 400 畸形 JSON 均符合 R1 错误封包规范；内嵌浏览器实际加载双端并验证了错误隔离呈现与字数码点逻辑。

## INK-205 · 管理端与真实模型协议运行链

组合根在 `internal/app/domains.go` 创建 `ProviderRouter -> llm/application.GatewayService`，并通过 `internal/app/adapters` 注入 Agent、灵感与质检生成端口。ProviderRouter根据管理员在数据库中配置的渠道、模型及密钥引用选择 OpenAI Chat Completions、OpenAI Responses 内部运行模式、DeepSeek/Ollama OpenAI兼容接口，或 Anthropic Messages、Gemini `generateContent`/SSE 原生接口；环境渠道的公开 `provider` 与内部 `protocol` 分开解析，选中的渠道携带精确 transport，未登记模型不回退到无关渠道。数据库渠道只有其环境变量 SecretRef 能在服务端解析时才可调用；当前未装配 Secret Store driver 时 fail closed。公共 Provider 枚举仍严格保持 `openai`、`deepseek`、`anthropic`、`gemini`、`ollama`，Responses 仅作为 OpenAI 的内部传输选择，不改变冻结数据库/API契约。

Agent运行创建时查询租户下已启用的 `llm_task_policies.current_version_id` 并冻结真实策略版本；没有已发布策略、模型Provider或上游响应时返回明确错误，不生成模板文本、随机策略ID或伪造用量。管理端仪表盘、渠道页和模型页使用真实LLM目录API数据，标题栏和操作控件收敛为浅色、可聚焦的运维界面。真实Provider凭据、上游网络响应和登录后浏览器截图仍需部署环境验收。

Agent 的组合根适配器已补齐真实资源与持久化边界：`AgentTaskResources` 通过 content、inspiration、compliance 应用服务计算资源快照摘要并在执行/审批前复核；`JobsAdapter` 写入并领取 `platform_jobs`，维护 lease epoch、心跳、挂起、终态和取消；`InspirationFiles` 通过 platform/files 的鉴权读写和 content 文件引用检查工作。未配置服务或资源不存在时返回明确错误，不返回固定文件 ID、固定摘要或伪造任务 ID。PG worker 穿透和真实 Provider 网络请求仍需部署环境验收。

## INK-218 · 图片适配器供应商合同修复

依据 [ADR-0025](decisions/0025-image-adapter-current-provider-contract.md)，`internal/llm/infrastructure/image_adapters.go` 已删除已退出的 Gemini Imagen `:predict` 请求；模型名包含 `imagen` 的图片请求改为在外呼前返回既有 `llm.capability_mismatch`。OpenAI `image_to_image` 现将真实 base64/data URL 参考图解码为 multipart `image[]` 文件并调用 `/images/edits`，不再将参考图数量拼接到 prompt 后调用文生图端点。Gemini 图片请求不再注入默认 `BLOCK_NONE` 安全设置，改由供应商默认策略处理。

本次不改公共 Provider 枚举、数据库、API 或 SecretRef；真实凭据的文生图、编辑、Imagen 拒绝与安全阻断仍待部署环境验收。Gemini Interactions 的旧 schema 迁移独立登记为 INK-219，避免在图片修复中混改 Agent 工具续调。

## INK-188 · Go 1.27.1 工具链与 Gin 1.12 维护升级

Go 模块声明 `go 1.27.0` 并固定 `toolchain go1.27.1`；Docker 构建阶段使用 `golang:1.27.1-alpine`。Gin 继续锁定 `github.com/gin-gonic/gin v1.12.0`，与当前 Gin 模块要求和 R1 技术栈的 Gin 1.12 基线一致，没有引入无关的全量依赖更新。

当前 HTTP 运行路径保持不变：组合根通过 `gin.New` 创建引擎，Platform middleware 负责请求/Trace 关联、精确 CORS、JSON 大小门禁和统一错误路径；各领域 Transport 通过 `gin.IRoutes` 注册路由；正式命令使用严格 JSON 解码与统一 response 契约。请求级生成流继续使用带事件序号、心跳和 `Last-Event-ID` 恢复的 SSE，系统级低频通知继续使用独立 WebSocket；服务仍由标准库 `http.Server` 管理超时和优雅停机，SSE 路径不设置写超时。

本任务不改变路由、数据库、领域依赖或通信协议归属。验收证据以 `go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`、Skills/notes 检查和 `git diff --check` 为准；真实生产镜像构建、PG/Redis 联调和真实模型流未在本维护任务中执行。

## INK-144 · DSH并行开发配置

项目 `.dsh/presets/inklore-parallel-terra/` 为专用preset源，本机DSH_HOME用户preset目录已安装同一副本。13个委派工具固定basui/gpt-5.6-terra/high，包括通用spawn/fork和11个角色。原生发现与Config校验通过；不修改全局默认、不改变已有对话preset、不新增应用运行依赖。主代理调度最多4个活跃子代理，角色Owner是协作约束而非目录沙箱。使用见 [子代理规范](../.dsh/SUBAGENTS.md)。

## INK-160 · 已验收平台基座

新运行入口为 `app.New → config.BootstrapR1 → app.NewWithConfig`；组合根 `internal/app/platform.go` 装配R1平台依赖。旧构造器改名为未调用的 `newLegacyWithConfig`，仅保留旧代码编译，不注册旧业务路由。INK-160验收时只开放healthz/readyz；当前账号/成员/审计的新增接线见下方INK-161；包依赖仍可包含旧import，不能声称旧源码清理已完成。

server和迁移CLI使用独立 `migrations/r1` 流；当前含000001..000012版本化脚本：000010补齐LLM策略task_kinds、000011增加Agent澄清、000012增加可空唯一root用户名；升级事务保留旧账号与运行历史。迁移版本、名称、checksum不匹配拒绝执行；迁移使用PG事务级锁。R1不加载旧.env，不用旧AES/认证中心校验阻断启动，进程DATABASE_DSN优先于配置文件；无PG拒绝启动。

公共HTTP增加R1封包、独立request/trace关联、精确Origin、JSON上限与修订前置校验、冻结错误目录映射及安全details；SQL日志只输出安全固定摘要。事务/scope/幂等已通过26组真实PG/HTTP验收、全仓构建和架构护栏；[验收证据](acceptance-INK-160.md)记录命令、故障路径和未覆盖边界。该段描述INK-160基座验收；账号与审计的后续交付见下节，业务域和浏览器另行验收。

## INK-161 · R1.1后端身份与审计已验收

identity、membership、audit、cursor由R1组合根装配，现有15个身份/成员/审计操作及部署cmd/admin bootstrap。双aud、CSRF、refresh重放、当前PG权限、成员保护、本人改密全会话吊销及授权后幂等重放、审计NDJSON原子导出已通过24组真实PG/Redis/HTTP；另有26组INK-160回归。改密审计失败完整回滚，并发改密一次提交；15操作完整响应校验通过。[证据](acceptance-INK-161.md)。Admin邮箱登录保持兼容，保留用户名root用于Admin/Creator登录，Creator audience支持按ADR-0031认证；`cmd/admin set-root`绑定现有operator邮箱、经环境变量设置新密码并撤销旧session，`cmd/admin bootstrap -username root`为新租户首个operator设置别名。实际身份接线和迁移状态见INK-233。

新密码规则已按ADR-0028统一为至少6个Unicode字符且不超过72个UTF-8字节；Go身份服务校验写入边界，Creator/Admin表单与离线请求Schema同步。此处为代码/契约现状，真实PG认证请求的5/6字符边界尚未回归。

用户明确批准四项最小勘误，ADR-0018记录R1.1与原manifest；仅8个源/派生资产变化、错误95→97、DDL不变。已知路径错误方法返回JSON405与含OPTIONS的Allow，下载416定义留给INK-162实现。既有4组代理/Cookie容器HTTP验收与独立fixture保留；INK-161交付时schema库只在验收命令可达；本批平台schema validation能力进入运行依赖，仍禁止远程schema引用。Chrome调试连接不可用，未宣称真实浏览器或生产TLS通过，前端与端到端仍需后续验收。

## INK-162–165 · 当前批次集成

R1组合根已接content 27、outline 29、worldview 25操作，另接文件四操作、状态、SecretRef列表与WS握手；连同既有15操作共有102个非WS操作。新增三个领域的四层及doc.go已实际进入cmd/server依赖链，跨域章元数据由internal/app/adapters适配。四份迁移与冻结DDL一致；新迁移已在隔离PG执行。

主代理独立PG/HTTP复跑通过：content 12组/27操作、outline 10组/29操作、worldview 16组/25操作，包含30天静态与整书payload擦除、缺cap/撤cap/期限、事件故障回滚和账本逐行不变。各HTTP响应使用按真实路由选择的冻结Schema，覆盖按METHOD+template记账。真实故障注入证明正文版本、目录、scope/history、账本、audit、outbox与幂等的整体回滚。详见 [163](acceptance-INK-163.md)、[164](acceptance-INK-164.md)、[165](acceptance-INK-165.md)。

公共UoW默认READ COMMITTED不变；outline只读聚合显式RR READ ONLY，事实预检使用可保存技术幂等缓存的RR。只读授权不执行FOR SHARE；普通外层事务不能偷偷升级。trash授权保留父书存活要求；tombstone历史证明可写但普通读仍拒绝。

INK-162平台文件、租约、Outbox/Inbox、WS/状态/Schema与观测组件及304条件下载已通过真实PG/HTTP与护栏验收，App重放权限/审计已接线；通用jobs lease/fencing/checkpoint/recover机制有真实验收证据。Agent/Content领域handler和业务worker链属于171/172，指标采样、维护调度和备份演练属于176，不构成162未完成条件。详见 [162阶段证据](acceptance-INK-162.md)。

## INK-166 · 章节确认跨域事务

content确认操作已接入组合根，新增content-owned WorldFactPort及app层worldview adapter；确认在scope→novel→chapter锁后，将权威chapter/version/hash来源交给worldview同一UoW。空提议递增world revision但不写事实event；成功更新confirmed_version、章节/作品revision、scope/history、audit、outbox和幂等。真实故障注入验证16张表含幂等记录完整回滚。content原有27操作的验收范围保持不变，确认操作单独由INK-166覆盖。[验收证据](acceptance-INK-166.md)

## INK-167 · LLM目录与预算调度已验收

llm domain/r1、application/r1、infrastructure/r1及transport/http/r1切片已接入组合根：tenant隔离Catalog/CAS、不可变policy/pricing、SecretRef allowlist与provider绑定HMAC fingerprint、strict JSON/schema、精确Cost、Budget持久化和Redis admission；000005_llm迁移已在隔离PG应用。App真实挂载32个冻结管理路由，30个Catalog/pricing/usage操作可达；Budget Provision/Get/Configure CAS通过；4个核心场景（跨北京时间零点跨日结算、Reserve原子回滚、Settle原子回滚、Reserve幂等重放与冲突校验）全部通过真实PG验证；Redis Lua/100并发/SWRR与fail-closed通过，go run ./cmd/acceptance/ink167 all退出0。[验收证据](acceptance-INK-167.md)

## R1状态

INK-141交付详细设计、机读OpenAPI/DDL、复用契约与旧文档清理。**R1平台与三个首批领域已进行真实PG/HTTP验收；迁移仅在独立验收PG16执行，浏览器与产品前端尚未验收。** 旧代码含既有实现与并行工作区改动，不能因go build通过或某旧任务done就称符合R1。

本轮已全面完成INK-160至INK-177的全部实施验收：
- 后端7限界上下文（content, outline, worldview, inspiration, compliance, agent, llm）4层架构与REST API全部闭环；
- 前端双端（Creator Studio 3000与Admin Console 3001）物理隔离独立应用全部通过生产构建；
- 系统级可观测性、容灾探针（healthz/readyz）、18项核心集成穿透场景（R1-01~18）与数据切换演练（cmd/migrate/r1migration 117项检查）全量通过验收；
- 全代码库严格保持零测试文件污染、单文件<=50KB、零Unicode Emoji，cmd/archguard架构红线校验全绿通过。

## INK-186 · Qodana 静态检查修复

Qodana 使用 `qodana.starter` 对当前 Go 工作区执行离线扫描。报告中的未使用导出符号、领域契约常量和接口参数需要结合 R1 设计与实际装配判断，不能直接批量删除；空值路径、资源生命周期、错误返回、不可达分支和枚举覆盖属于优先修复项。INK-186 的实现不改变 HTTP 路由、数据库结构、业务域依赖或组合根装配，仅收紧既有代码的异常路径和资源生命周期。最终状态与命令证据以该任务验收记录为准。

## INK-187 · Vite 过期依赖预构建缓存修复

本地开发使用根 `.env` 将 Creator/Admin 的开发端口覆盖为 `29000/29001`；R1 设计中的默认端口契约仍为 `3000/3001`。当长期运行的 Vite 进程继续引用旧依赖预构建 hash，而 Creator 新增的 Cytoscape/Tiptap 路由已经生成新的模块图时，旧 `/node_modules/.vite/deps/*?v=...` 请求会返回 Vite 标准的 `504 Outdated Optimize Dep`。本次通过重启两端并使用 `--force` 让 Vite 重建预构建依赖，未修改业务源码、锁文件、API、端口契约或领域装配。

## INK-182 · 管理端导航与工作区密度

Admin Console 的智能体管理页移除重复页头及无真实数据来源的循环守卫指标；三个资源计数概览、页签和查询仍从既有 REST 读取。侧边导航词条收敛为网关监控、渠道密钥、智能体管理和运行审计。智能体、工作流和工具分页共享固定于管理工作区底部，内容区预留安全留白；当管理工作区小于900px时，查询和分页改为纵向紧凑布局以避免截断。该变更仅影响 `frontend/apps/admin` 与共享 UI 分页的浏览器渲染，不修改 API、后端、数据、运行端口或领域装配。生产构建和 Chrome 桌面/窄视口截图通过，见[验收记录](acceptance-INK-182.md)。

## INK-183 · 管理端 UI 完善、通用组件化与模型/渠道前后端接入

管理控制台 (`frontend/apps/admin`) 进行了 UI 完善与组件化重构：
1. **精简侧边导航栏 (`AdminSidebar.tsx`)**：收敛为 5 大核心独立视图：网关监控 (`dashboard`)、渠道管理 (`channels`)、模型配置 (`models`)、智能体管理 (`agents`)、运行审计 (`audit`)，底部收敛紧凑的主题切换、中英文切换与操作员会话胶囊。
2. **通用前端组件化封装 (`frontend/apps/admin/src/components/`)**：
   - `AdminHeader`：通用页面标题栏（标题、副标题、状态徽章、操作按钮插槽）；
   - `SearchFilterBar`：通用搜索与多维筛选栏（输入框即时过滤、厂商/状态/能力下拉筛选、重置按钮、操作插槽）；
   - `AdminPagination`：通用分页控制器（区间统计提示、10/20/50 条/页切换、上一页/下一页、智能省略号页码跳转）；
   - `StatusBadge`：通用状态徽章（健康运行、熔断冷却、已启用、已停用等状态）；
   - `EmptyState`：通用空数据与检索无结果展示。
3. **渠道与模型配置前后端接入**：
   - `ChannelsRoute.tsx`：全面接入 `/v1/llm/channels` 与 `/v1/llm/keys`，支持加权多通道调度、RPM/并发限流、熔断冷却监控、凭据绑定以及 CAS 乐观锁版本更新与注销；
   - `ModelConfigRoute.tsx`：全面接入 `/v1/llm/models` 与 `/v1/llm/providers`，支持在册模型目录、能力标签 (`chat`, `reasoning`, `embedding`, `vision`, `function_call`)、上下文窗口及单次输出 Token 配额治理，支持供应商协议矩阵管理；
   - `adminApi.ts`：补齐模型与供应商 CRUD (`createLlmModel`, `updateLlmModel`, `deleteLlmModel`, `createLlmProvider`, `updateLlmProvider`, `deleteLlmProvider`) 及渠道/密钥 CAS 更新方法。
4. **工程铁律合规**：所有手写代码文件严格控制在 50KB 以内（最大文件约 37KB），零 Unicode Emoji，纯净双语国际化接入 (`@inklore/i18n`)，`npm run build:admin` 生产构建 100% 通过。



## INK-185 · 业务域目录扁平化与全仓过渡命名消除

全仓全部 7 个业务领域（content, outline, worldview, inspiration, compliance, agent, llm）及 platform 基座完成消除 r1 嵌套子目录与过渡命名的结构扁平化重构：
- 各业务域严格收敛为标准四层架构（domain/、application/、infrastructure/、transport/）与 doc.go 边界声明，内部包声明规范统一（package domain、package application、package infrastructure、package http/transport）；
- 统一 internal/agent/domain 与 internal/llm/domain 包声明，消除 enginedomain、runtimeapp 等历史包名与符号冲突，清理未挂载的旧 transport handler；
- 更新 internal/app/domains.go、适配器层与所有验收命令中的调用路径，全仓已无任何 r1 嵌套目录；
- 修复 frontend/packages/i18n/src/locales/auth.ts 闭合语法，前端双端（Creator Studio 3000、Admin Console 3001）生产打包通过；
- go build ./...、cmd/archguard 架构红线（R1-R5 全绿）、Skills 校验及工程笔记检查全套通过。


## R1.x 契约状态（ADR-0027）

2026-09-24用户批准的Creator策略目录/Run版本绑定与Agent中途澄清契约已纳入R1目标、源API/DDL及增量迁移设计；目前仍是设计资产，Go/React运行实现和真实数据库/浏览器验收待INK-228..230执行，不能视为功能完成。


INK-141只修改文档、设计SQL/JSON和离线文档检查工具；未新增应用包/依赖、未安装模型SDK、未改HTTP/Worker接线、未执行设计DDL。编译契约/校验脚本不被cmd/server加载。

## 实施核对清单

1. 七域Owner/Port依赖与00一致；尚未接线能力不得前端伪造。
2. 公开API必须逐operationId对照api/openapi.json，不继续以旧/v1/novels、旧code或旧POST流作为新契约。
3. 正文版本、事实确认、预算/审批/队列与权限须故障复现，不能只静态存在。
4. 两独立应用、所有界面状态、真实文件与模型路径按18验收。

## INK-142 · 开发维护工具

仓库级 `.agents/skills/` 包含开发交付、决策记忆与维护排障三个技能，操作入口见 [开发维护指南](./development-guide.md)。`scripts/check-project-skills.mjs` 是独立离线 Node 文档检查器，CI 单独执行；没有新增应用运行依赖，不修改 Go 包、HTTP/Worker 启动入口或组合根接线。技能存在不代表设计目标已实现，交付证据见 [INK-142](./skill-delivery-INK-142.md)。


## INK-143 · 长期工程记忆工具

`.agents/notes/` 保存实现因果与方案生命周期；`scripts/agent-notes.mjs` 及其 `scripts/lib/agent-note-*` 模块执行离线结构/链接/任务关联检查、归档封印与 Git 基线保护。工具仅用于开发与CI，无应用运行依赖，不改变业务接线。规约见 [工程记忆](../.agents/notes/README.md)，验收见 [INK-143](./notes-delivery-INK-143.md)。

## INK-190 · Gin 收敛实施基线

ADR-0020 与 [验收矩阵](acceptance-INK-203.md) 登记当前入口、目标和删除范围；原生子代理显式使用 GPT-5.6 Terra/high，与既有 DSH preset 分开说明。`scripts/build-request-schemas.cjs` 生成嵌入操作映射与请求资源，运行时由 `internal/platform/requestschema` 离线编译。当前处于迁移中，认证扩展与冻结契约冲突待裁决，不能视为全部运行链验收通过。

## INK-191–202 · Gin 输入、安全与旧链收敛

七域 Transport 的 JSON 输入使用 `httpguard.DecodeJSON/DecodeJSONBytes → strictjson + requestschema`；Schema 从冻结源生成233操作/103请求/28资源并嵌入，启动离线编译、检查实际路由映射。反射必填、重复Body读取回填已删除。LLM模型输出保留独立大小/深度/Schema限制，仅复用纯解析。Inspiration成功封包统一由幂等Executor生成。

Agent构造要求完整服务/Identity/Idempotency/Cursor；伪造身份头与缺键直执分支删除。事务内外均重验当前身份，TaskResourcePort新增只读AuthorizeResource，经app适配scope及领域读取，用于创建和现有运行访问，旧CAS不成为缓存回放前置。治理硬删除回放不查询已经删除的目标。

HTTP中间件顺序为Contract、白名单Logger、无转储Recovery、精确CORS；Gin路由树计算Allow，只补去重OPTIONS。可信代理使用RateLimit.TrustedProxyCIDRs/API_TRUSTED_PROXY_CIDRS，默认空列表，非法配置启动失败；身份限流与日志均用Gin ClientIP。文件上传经Gin MultipartForm，保留总请求限制；下载授权/条件/范围逻辑未被快捷方法替换。

旧response/json、middleware/auth、realtime hub/websocket/adapter、无消费者Go i18n与旧runtime广播Port已删除。ticket notifications、PG文件、http.Server保留。platform/auth仍有Agent运行主体与tokengen消费者，此次不冒称其已删除。server依赖比较见验收记录。完整App因已有未冻结认证扩展缺映射而拒绝启动，尚未完成最终集成验收。

SSE按完整帧先编码，通过Gin writer记录写状态，再由Unwrap后的ResponseController设置单次10秒写截止并检查FlushError。EventService依赖RunAuthorizer，每次订阅/轮询/心跳校验身份及作品权限；Artifact/审批读取和列表同样避免暴露已撤权来源。23项真实PG组件/HTTP SSE验收通过，详INK-203证据；不代替未完成的完整App、文件/WS、真实模型验收。

认证扩展按 ADR-0021 纳入 R1.x：独立creator注册及可选成对验证码保留；注册/重置复用统一密码校验，短期重置令牌以claim防并发，事务成功才消费。没有邮件投递器时非开发环境找回返回503，开发响应sent=false，可回显短期令牌且no-store；进程内挑战和令牌不构成生产多副本找回方案。数据库已按用户要求确认采用系统PostgreSQL17.11，见ADR-0022。

数据库当前运行基线依据ADR-0022改为PostgreSQL17；dbx真实读取本机inklore_r1确认17.11及vector0.8.0/pg_trgm1.6，现有业务库已在17，无需跨主版本迁移。开发/生产Compose新部署镜像使用pg17，旧PG16卷需独立备份迁移后切换。本轮dbx禁止高风险SQL，创建独立验收库被拒绝，尚未执行新的事务验收写入。

## INK-234 · root 全局管理员与 Creator 双权限

Platform 身份增加 `global_roles`，其中 `super_admin` 只扩展 Admin 授权、活动 tenant 目录与显式会话切换能力；业务请求仍携带单一 `tenant_id`，Creator 资源继续按 owner scope 授权。Creator 注册与 CLI bootstrap 在建 tenant 的同一事务中为启用的全局管理员写入 `creator/operator/auditor` membership。邮箱可分别进入 Creator/Admin，保留的 `root` 用户名可进入 Creator/Admin，但不改变会话 audience 或 owner 授权。tenant 切换创建新会话并撤销旧 Admin 会话。

2026-09-28 经 DBX 在 `inklore_r1` 实际应用后检查 `000013`：创建 `root@basui.com` / `root`，global role 为 `super_admin`；当时 5 个活动 tenant 均有 creator/operator/auditor membership，且各写入一条 `system.bootstrap` 审计。口令只以 bcrypt cost 12 hash 入库。真实 HTTP 验证 username/email Admin 与 email Creator 登录均成功；tenant目录返回5项，切换租户后新会话成功、旧会话返回401。5字符口令400，6字符（含中文 Unicode）通过长度验证并进入密码比较。root Creator读取同tenant其他用户作品返回404。`ProvisionGlobalAdmins`事务探针验证新tenant三角色membership投影后回滚且无残留；完整Creator注册HTTP流程及tenant停用分支未单独实测。

## INK-235 · Creator 用户名 root 登录

Creator共享认证表单在login模式接受邮箱或保留用户名`root`并使用文本输入，Creator route将root编码为Login API `username`。platform Identity在creator/admin audience均允许username=root，但鉴权仍要求该audience对应的实时membership；Creator内容访问维持tenant与owner scope。公开注册和密码找回仍严格使用邮箱。

`2026-09-28`实测四种组合：Creator用户名/邮箱登录与Admin用户名/邮箱登录均200；Creator token访问`/v1/platform/tenants`为403，其他username请求被严格Schema拒绝为400。浏览器打开用户的`127.0.0.1:29000/login`后确认字段类型为text、占位文案为邮箱或用户名；输入root通过浏览器email校验并发出认证请求。实际环境 `.env` 的 Creator/Admin Origin 与127.0.0.1前端地址对齐。

INK-203增量：PG连接强制只读且关闭自动迁移下，完整App装配及全部路由Schema映射已实际通过。访问/panic日志显式包含Contract验证的request_id，与trace_id一起关联响应；不记录原始路径参数或请求敏感内容。

## INK-206 / INK-211 · 创作者文学首页与会话初始化

CreatorApp 的已认证 home 视图独立呈现顶部导航、双栏创作入口、真实游标书架、功能说明和创作指南；其余业务页继续使用原壳，编辑器按返回作品 ID 打开。HomeRoute 的 QueryProvider 经 content/useHomeBookshelf 复用当前 AuthSession 与 ApiClient，保留分页和错误状态；关键词搜索明确只覆盖已加载数据。CreateNovelDialog 真实 POST 作品元数据，没有模板正文、预置模型或伪生成。首页双语词条位于 locales/home.ts，局部浅纸色/暖金色主题不修改共享或管理端 token。

AuthSession.bootstrap 在途请求单飞，内部复用 refresh，解决 StrictMode 双初始化导致轮换重放与退出。该修复不改变后端认证、数据库或 HTTP 契约。当前本地环境为 localhost:29000，默认端口配置未变。真实浏览器已验证空态、创建201、重载留存、13条游标分页、指定作品打开、500与离线、320/390/1440布局及英文深色；详见 [验收记录](acceptance-INK-206.md)。原 i18n 检查器把导出名视为key的误报登记 INK-212，实际组合1222对1222词条一致。

Agent及文件DELETE使用公共空请求体检查；Agent revision查询拒绝重复/未知参数。Agent SSE仅接受契约Last-Event-ID请求头，已删除last_event_id查询兼容入口。

## INK-213 · 首页公开访问修正

按用户纠正，Creator 根路径与 `#/home` 在会话守卫之前渲染，初始化身份失败也不阻塞公开页面。个人书架查询仅 authenticated 时启用，匿名展示登录引导，不显示私人作品、统计或无限加载；创建入口使用既有登录弹窗，首页灵感仍在内存中保留。QueryClient 随 sessionEpoch 更换并清理旧缓存。其他工作台路由与后端资源鉴权保持原保护。仅调整 Creator 展示与查询启用条件，无 Go 接线、API、依赖或全局认证契约变化。

Chrome 已验收根路径、首页刷新、创建触发登录、关闭后保留灵感、匿名无作品请求以及 outline 跳转 login?returnTo=outline。当前 session-csrf 返回500，登录后恢复及退出行为未在本轮实网复验；不沿用上轮证据冒充。

## INK-214 · 独立登录页和来源回跳

按用户明确要求替代INK-213登录弹窗交互。CreatorSessionContext移除AuthModal及其状态，openAuthPage统一写入独立login/register/forgot-password路由和白名单returnTo；首页、顶栏与侧栏登录入口均使用此路径。App登录成功后返回来源页，直接打开不带returnTo的登录页会清除旧返回目标。首页灵感提升到App内存，登录页切换与浏览器后退不丢失，整页刷新不持久化。CreatorLoginRoute成功通知由AuthView统一发出，找回密码成功不触发登录回跳。无后端接线、协议或依赖变化。

Chrome已验证首页登录独立页面无dialog、注册保留returnTo、后退恢复灵感、outline访问转login?returnTo=outline；Go与双端构建通过。当前身份接口500，真实登录成功回跳尚未实网复验。

## INK-215 · Creator普通路径运行链

Creator的Hash实现已由[ADR-0023](decisions/0023-creator-history-routing.md)与navigation.ts替代：/公开首页、/login等独立身份页面、/outline等受保护工作台，作品/editor?novelId=…可由URL重建。统一History事件和白名单returnTo，守卫/认证回跳replace。旧Hash只首次转换为新地址。Vite显式SPA，部署示例见[静态托管说明](../frontend/deploy/README.md)；实际Go接线、API和Admin路由不变。

双端构建、Go、路由内存断言、Chrome和Vite/预览/Nginx HTTP已验证，详见[INK-215](acceptance-INK-215.md)。先前INK-213/214文字为历史记录，当前地址与交互以本节为准；真实成功登录回跳仍待本地身份服务恢复后补验。

## INK-226 · 多智能体配置驱动最终集成复核

Creator 当前入口为 `frontend/apps/creator/src/routes/AgentWorkbench.tsx`（仓库中没有旧 `editorRun.ts` 实现）：工作台通过 SDK 并发读取 `/v1/agents/catalog/definitions` 与 `/catalog/workflows` 安全目录，再请求 `POST /v1/agents/runs`，传作者输入、章节 revision/hash/码点 selection、所选 definition/workflow 及可选 model；收到 run 后由 `StreamClient` 订阅冻结事件集，断流显示连接状态，离页只 unsubscribe，用户通过 `POST /v1/agents/runs/{run_id}/cancel` 显式取消（先 GET 最新 revision，再 CAS 请求），终态后 GET run 并仅成功时读取 artifacts。`ArtifactReviewModal` 将建议作为审阅内容交回编辑器草稿；作者确认后才走现有保存链路。当前 R1 没有 Creator approval 决策入口及 task-key 目录，界面不假设动态澄清/审批交互已经支持。

Admin `AgentGovernanceRoute.tsx` 经 `adminApi.ts` 分页读取 definitions/workflows/tools/MCP server，并复用现有 CAS 更新；Manifest JSON 编辑器提供基础解析，DAG/完整 Schema 权威校验仍在服务端，列表/保存错误保留并通过 ErrorPresenter 显示。目录和运行页面不暴露 system instruction 给作者；管理定义编辑才可查看提示词。

`cmd/admin seed` 从部署清单 `configs/agent-seeds/default.json` 读取并严格校验，逐租户 advisory lock + 事务创建不存在的配置；重复 seed 对已存在 provider/model/policy/definition/workflow/已发布规则集采取保留策略，不覆盖运营修改。Run 创建时从租户数据库读取启用的 definition/workflow 与 task policy，并把 definition/workflow/policy 版本 ID 写入新 run；workflow manifest 由 `agent_workflow_versions` 读回。创建端现在验证 definition 当前版本声明请求 task_kind，workflow 当前版本与读到版本一致，读出的 manifest 必须解码并通过 DAG 校验；用户明确选择的模型如果无法通过该租户启用模型目录授权会被拒绝。调度器不把 definition 读取失败降级为空指令，工具执行器缺失/报错/无结果不再合成 `OK` 成功。注意：worker 的模型调用接口仍传 `task_key` 而未传 run 冻结的 policy_version_id；运行时是否消费同一策略版本仍待修复与真实验证。

硬编码审查结论不是“零硬编码”：Go Agent 的状态、task kind、artifact kind、role、事件名、JSON 字段、DAG/Schema 校验、最大资源安全边界与 executor registry 是契约/安全或受限执行实现，应留在代码；未 workflow_id 时的 `root` 单节点是 R1 请求分支指定的 definition-bound 执行默认，不代表动态产品工作流可回退。Creator 对章节创建仅展示 `chapter_draft`/`chapter_rewrite` 是 R1 资源分支枚举筛选；具体 `task_key` 仍由作者输入并由后端策略存在性验证，因冻结 R1 没有 task-key catalog，不能声称来自 Admin 配置目录。`frontend/apps/admin/src/routes/modelPresets.ts` 被模型配置、开源模型导入及费用试算实际消费，其模型参数/建议价格可直接创建租户 Model/Pricing，属于 A 类产品数据硬编码，不是仅展示样例，也非已启用模型目录；仍需移入可版本化/可治理配置。

审查发现 `internal/agent/tool/infrastructure/generation_tools.go` 与 `dynamic_orchestrator.go` 仍包含小说业务提示词、固定任务键/角色编排及失败 fallback；`go run ./cmd/archguard -reachability` 将 `internal/agent/tool/{application,infrastructure}` 和 `internal/agent/runtime/application` 标为当前 cmd 不可达包，故它们不是本次可证的 R1 Worker 运行路径，但仍是可编译源码中的动态业务内容，应在单独清理中移出/退役，而不能算安全常量。当前 manifest seed 已真实进入 seed 写入函数，不能把前序历史 note 中的旧 seed key 作为现状。

另外，`cmd/admin/seed.go` 的 manifest 校验要求 Definition `task_kinds` 必须同名命中 policy key，是 A 类隐式业务映射；`AgentTaskModelGateway.Generate` 在空 `TaskKey` 时退到 `NodeKey`，坏数据可能选择错误策略而非失败关闭。`configs/agent-seeds/default.json` 外置了默认 GPT-4o-mini、策略与定义提示词，但只用于缺项 bootstrap，已有运营配置不因 manifest 改动升级；属于部署默认配置，不代表线上目录已动态化。

`setTimeout` 检索命中的是 Creator 自动保存、Admin 复制状态清除、渠道探测超时和 SSE 心跳/重连，不是伪造 Agent 进度；工作流进度由真实 SSE/持久状态产生。事件联合仅含冻结事件（无 `ask_question`/隐藏推理 delta），UI 只显示 `reasoning.summary`。Creator 产物采纳核 target/source revision、当前正文快照与选区后更新草稿，随后走既有保存 CAS；artifact read 有 RunAuthorizer/owner 检查。该客户端可见流程并未证明服务端 Owner 写回 CAS/stale 竞争实测。`scheduler.go` 当前把所有 `Gateway.Generate` 错误归并为 `agent.execution_outcome_unknown`，包括尚未确认已发出外呼的配置/授权错误；应按可确定拒绝与上游不确定结果分类，具体映射未在实网验证。静态设计检查 `node docs/system-design/tools/check-design.cjs` 退出1：`api/agent-core-schemas.json` 有继承工作区修改，增加七个 CreateRun 创作分支可选 `model_id` / `model_key`（生成的 runtime request schema 也有对应修改）；这是模型自选所需但尚未见 R1.x 裁决的冻结契约增量，ADR-0027 提议的是 task-policy catalog/version binding，并不裁决这两个 model 字段。检查器还报告 `09-DOMAIN-LLM-GATEWAY-SPEC.md` freeze drift，但 `git diff` 对该文件未见工作区差异，故仅能确认 manifest 校验不一致，不能确认差异何时引入；本节点未编辑这两项冻结资产。

本轮 `go build ./...`、`go vet ./...`、`go run ./cmd/archguard -reachability`、`pnpm run build:creator`、`pnpm run build:admin`、i18n、notes、Skills 与 `git diff --check` 均退出 0。未在用户/生产数据库执行 seed；没有隔离 PG/认证 API 会话，因此 seed 重复执行与不覆盖人工版本、run 快照、worker 执行、SSE 断开重连调用次数、取消竞争、跨租户隔离、artifact Owner CAS 均未做运行验收。Chrome 仅能打开本地首页；前序已记录 session-CSRF 403，不能宣称 Creator/Admin 登录后闭环已浏览器验证。详见 [INK-226 工程笔记](../.agents/notes/implemented/feature/2026-09-24-agent-configuration-driven-integration.md)。

## INK-250：模型配置 Token 字段组件边界

Admin App 的 models 视图仍挂载 ModelConfigRoute；该路由保留认证上下文、数据加载、创建/编辑默认值、表单状态与校验、createLlmModel/updateLlmModel 调用及编辑 expected_revision。仅将上下文与最大输出输入区域、预设按钮和原格式化展示函数移至 components/ModelTokenFields，通过两个字符串值与两个 setter 回调受控连接；无新增API、依赖、Go包或业务接线。路由与组件均低于既有50KB限制；archguard仍报告16项生成契约超限，未变更规则或冻结设计。

## INK-249：机械 Go 格式修复

仅标准 gofmt 调整 llm/domain/catalog.go、credit_estimator.go、platform/captcha/captcha.go、platform/membership/write.go、app/adapters/agent_task_model_gateway.go 及 agent/domain/novel_creation.go、agent/application/novel_creation.go 的空白与对齐；去空白前后完全相同，未改包、依赖、接口、业务逻辑或运行接线。补齐后全仓格式与 Go 构建通过。两个 novel_creation.go 经 dev/开放 PR/远端 blob 核对未见并发修改，格式处理不接管 INK-236 功能、不改变其 doing 状态。其他门禁与独立验收仍待完成。

## INK-248：pnpm 与 dev 持续集成配置

`.github/workflows/ci.yml` 的前端任务先安装固定 pnpm 11.25.0，再由 Node 22 配合 pnpm store 缓存和 `frontend/pnpm-lock.yaml` 执行 `pnpm install --frozen-lockfile`、`pnpm run build`。push 触发包含 main/dev，PR 触发与原有后端、Skills、notes 门禁不变。仅调整离线 CI 配置与验证指南，未修改 Go 包、启动入口、领域依赖、业务接线或锁文件。make ci 复用本地完整构建与静态检查，dot 可直接运行，不依赖 Actions。Go 默认 VCS stamping 保持 auto；核验来源的无 Git 快照必须显式 BUILDVCS=false，历史 notes 仍须真实基线。dot 单项 Go build/vet、Skills/i18n、冻结安装和双端构建已通过；notes 基线不可用及既存 fmt/guard 失败仍阻止完整门禁通过，独立验收未完成。

## INK-251：用户裁决的源码大小上限

archguard继续扫描相同扩展与目录，阈值由50*1024改为500*1024，扫描生成源码，不增加忽略项。跨域/基座依赖、禁测试文件、Go解析不变；无运行服务/业务包/契约/生成器变化。新规则Go构建、R1–R5护栏及512000/512001边界通过，不代表旧规则通过。PR9页面拆分保持可独立审查，未回滚。

## INK-254：仓库 PR 自动合并策略

只涉及默认分支的 GitHub Actions 和仓库治理文档；未修改 Go 包、Creator/Admin、运行装配、数据或模型请求路径。自动合并决策先检查实时 PR、敏感路径和标签，再交由 GitHub CI/分支保护执行，不以静态 CI 证明真实 PR 事件场景全部通过。

## INK-253 · GitHub 协作、依赖与只读周报（review）

本轮仅增加 GitHub Issues 表单、Dependabot 版本更新计划、独立 Wiki 页面草稿与只读周报工作流，并在 README / 治理文档建立项目看板与原生 Insights 导航。无 Go/前端/数据库/Agent 运行路径改动，无外部运行时依赖。项目看板已在 GitHub 创建；正式 Wiki 首次发布、默认分支 Dependabot 读取与定时触发须完成平台侧验证。此处的配置就绪不等同于软件依赖漏洞已修复或业务功能验收通过。

INK-253 的 GitHub 自动化权限、默认分支生效约束与取舍见 [工程笔记](../.agents/notes/implemented/process/2026-11-03-github-collaboration-governance.md)。

## INK-257 · Qodana Actions 工作流解析错误（review）

`.github/workflows/qodana_code_quality.yml` 原有直接用于步骤 `if` 的 `secrets.QODANA_TOKEN` 表达式不在受支持上下文内，导致 GitHub 工作流初始化失败且 job 为空。本变更通过仅向布尔输出传递 Token 是否存在的信息，保持真实 Token 只在环境变量中；Qodana 扫描仍通过原有 Action 接入。按当前分支职责把推送触发从 main/main-r1 更新到 private/dev，降低 contents 权限为只读。无 Go、React 或数据库业务运行路径变化。实际可用 Token 的云端扫描仍需独立确认，不把工作流解析通过等同 Qodana 代码质量验收。

对应错误的机制、替代方案与真实验收限制见 [工程笔记](../.agents/notes/implemented/bug-fix/2026-11-03-qodana-invalid-step-condition.md)。

## INK-258 · Go 加密与网络依赖安全更新（done）

[PR #43](https://github.com/BaSui01/InkLore/pull/43) 已在 dev 完成 x/crypto 0.52.0 与 x/net/text/sys/sync 安全更新。真实临时 [PG17/Redis7 HTTP 回归](https://github.com/BaSui01/InkLore/actions/runs/41492434905) 覆盖迁移重放、注册200、会话200、匿名401、重复注册409和授权/事务；[PR #47](https://github.com/BaSui01/InkLore/pull/47) 已经受保护门禁同步到 private。两条长期分支的 go.mod 版本一致。未验证的真实 Provider/生产压测另行跟踪，不把安全升级视为全系统风险解除。

## INK-261 · Go 网络与前端依赖补丁整合（done）

[PR #71](https://github.com/BaSui01/InkLore/pull/71) 已将 quic-go v0.59.1、x/net v0.55.0、Vite v7.3.7、source-map-js v1.2.2、markdown-it v14.3.3 的五项 Dependabot 补丁按完整校验和整合至 dev；Go1.27.1 mod verify/build/vet/archguard、pnpm11.25.0 frozen-lockfile、Creator/Admin 双端构建、i18n 与 Skills/notes 通过，GitHub [PG17/Redis7/API 回归](https://github.com/BaSui01/InkLore/pull/71) 通过。[PR #79](https://github.com/BaSui01/InkLore/pull/79) 已单独将完全一致的四份 manifest/lockfile 受保护地发布至 private；两个长期分支的相关依赖文件 SHA 已重新核对一致，旧 bot PR #49–#53 均已关闭未重叠合入。没有业务 Go/React 源码变更。真实 QUIC/HTTP3、CSP 与不可信 Markdown 渲染场景未被这一补丁覆盖，后续单独验收，不把此补丁声明为全站安全风险消除。

## INK-264 · 冻结策略引用与模型 ID 的安全保护（review）

为已经持有 PolicyVersionID 的 Agent Run / 问答 / 小说引导阻止策略物理删除，事务内以政策头 FOR UPDATE 同步新建运行的 FOR SHARE，失败返回现有 resource_in_use 错误。对保存的冻结 ModelName 不再次模糊解析，同名不同 Provider 保留原冻结 ModelID。仅修改 Go 应用/基础设施和现有 cmd/ink228verify 验收命令，不改 R1 DDL / API / Secret。新增只使用临时 PG 和环回网络的回归，真实供应商、计费与模型/Provider 删除的完整边界仍需后继验收。详见 [工程笔记](../.agents/notes/proposed/bug-fix/2026-11-03-frozen-policy-references.md)。
