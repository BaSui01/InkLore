# 项目架构与工程规则 (Architecture & Engineering Guidelines)

> R1完整重构唯一设计入口：[docs/system-design/00-INDEX.md](docs/system-design/00-INDEX.md)。旧设计已清理；以下目标不表示旧代码已经完成重构。依赖图的图形布局仅作分域示意，精确DAG以§1.2和ADR-0016为准。

本项目是模块化单体（Modular Monolith）小说内容生成平台，不拆微服务。HTTP API 与后台 Worker 运行于同一单体仓库中。

---

## 1. 架构分层：公共基座 vs 业务领域 (Platform vs Domains)

系统采用 **Domain-First 领域驱动分层** 与 **微内核技术基座** 结合的架构治理模式：

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                    微内核技术基座 (Cross-Domain Platform)                  │
│  internal/platform/: logger, cache (Memory+Redis), config, safego, crypto, │
│                      auth, timeutil, errors, response, middleware, sqlscope│
│              (不按特定业务域区分，作为全局共享的底层技术基础设施)          │
└─────────────────────────────────────┬──────────────────────────────────────┘
                                      │ 支撑
                                      ▼
┌────────────────────────────────────────────────────────────────────────────┐
│                   业务领域上下文 (7 Bounded Contexts)                       │
│                                                                            │
│  内容事实侧                             智能与算力侧                        │
│  ┌───────────────┐ ┌───────────────┐   ┌───────────────┐ ┌──────────────┐ │
│  │content        │ │outline        │   │agent          │ │llm           │ │
│  │作品与正文权威  │ │大纲与剧情蓝图 │   │TAO 工作流引擎 │ │模型网关与调度 │ │
│  └───────────────┘ └───────────────┘   └───────────────┘ └──────────────┘ │
│  ┌───────────────┐ ┌───────────────┐   ┌───────────────┐                  │
│  │worldview      │ │inspiration    │   │compliance     │                  │
│  │设定与事实权威  │ │拆书与灵感工坊 │   │合规与质检     │                  │
│  └───────────────┘ └───────────────┘   └───────────────┘                  │
│                                                                            │
│  每个域内部严格执行:                                                        │
│      Transport  ->  Application  ->  Domain  <-  Infrastructure            │
│                                                                            │
│  R1依赖DAG（ADR-0016）：
│  outline / worldview / llm 无业务依赖
│  content -> outline / worldview / compliance（只读导出审阅）
│  agent -> content / outline / worldview / inspiration / compliance / llm
│  inspiration / compliance -> llm；禁止content反向调用agent
```

### 1.1 公共技术基座（`internal/platform`）
- **职责范围**：横切各领域的通用技术支撑，包含日志与全链路追踪（`logger`）、双引擎缓存（`cache` 内存+Redis）、配置中心（`config`）、安全协程（`safego`）、密码学工具（`crypto`）、用户/租户上下文契约（`auth`）、标准时间工具（`timeutil`）、统一错误与响应（`errors` / `response`）。
- **定位**：作为底层技术基石，**不按业务域拆分**，为所有上层领域提供高可靠底层基础设施。

### 1.2 业务限界上下文（`internal/{content, outline, worldview, inspiration, compliance, agent, llm}`）
- 所有业务代码必须严格收敛在 **7 个业务域**中：`content`（作品与正文）、`outline`（大纲与剧情蓝图）、`worldview`（设定与事实图谱）、`inspiration`（拆书与灵感工坊）、`compliance`（合规与质检）、`agent`（智能体与工作流）、`llm`（模型调度网关）。禁止在根级设立通用的 `service`、`repository`、`shared` 等扁平包。
- 每个业务域必须具备 `domain/`、`application/`、`infrastructure/`、`transport/` 四层与一份 `doc.go`（声明该域职责与边界）；新增业务域须同步登记到本节与 `cmd/archguard` 的 `businessDomains`。
- 域内严格执行依赖单向流动红线：`Transport -> Application -> Domain <- Infrastructure`。
- **R1域间依赖只允许单向DAG**（ADR-0016）：`outline`、`worldview`、`llm` 不依赖业务域；`content` 经Port读取 `outline/worldview/compliance`（compliance仅导出审阅）；`agent` 经Port调用 `content/outline/worldview/inspiration/compliance/llm`；`inspiration/compliance` 仅依赖 `llm`。`content` 不反向调用 `agent`，**严禁出现环**。
- **跨域调用红线**：
  - 严禁跨域直接导入对方的 `infrastructure`，严禁跨域读写其他领域的私有数据库表；
  - 消费方必须在自己的 Application/Domain 层定义抽象端口（Port），并在 `internal/app/adapters` 负责适配装配；
  - **唯一受控例外**（ADR-0014 第 4 条）：允许跨域导入对方 `domain` 包中的**只读值类型**用于组装读模型（如 `content` 的 `ContextPack` 读取 `worldview` 与 `outline` 的值类型）。该例外不得用于写路径，新增此类导入需在评审中说明。

---

## 2. 领域职责划分与事实单一来源

1. **内容域 (`internal/content`)** —— 作品与正文事实权威：
   - 作品、卷、章节正文与章节版本快照的**唯一权威 Owner**；
   - 负责章节正文的版本保存、差异比对与多格式导出；
   - 「章节确认」（`ConfirmChapter`）是本域用例，它通过 Port 调用 `worldview` 落账，反向调用不存在。
2. **大纲域 (`internal/outline`)**：
   - 三级大纲（总纲 / 卷纲 / 章节细纲）、剧情节拍、伏笔线索状态机与叙事张弛度曲线的权威 Owner；
   - 不依赖任何其他业务域。
3. **设定与事实图谱域 (`internal/worldview`)** —— 设定与事实事实权威：
   - 角色档案、世界观词条、实体关系三元组与**跨章节事实账本**的唯一权威 Owner；
   - 负责事实一致性校验；事实账本只增不改不删；
   - 不依赖任何其他业务域，跨域引用一律以 `string` ID 表达，不得导入 `content` 的任何类型。
4. **拆书与灵感工坊域 (`internal/inspiration`)**：
   - 参考作品拆解、叙事套路提炼、作家文风指纹与灵感卡片的权威 Owner；
   - 产出均为**外部建议资产**，严禁直接写入 `content` 或 `worldview` 的事实表；
   - R1要求实现完整领域闭环，具体服务/接口/SQL见system-design对应分册；代码是否已实现只认验收证据。
5. **合规与质检域 (`internal/compliance`)**：
   - 敏感词与违禁内容审查、去「AI 味」检测与行文净化的权威 Owner；
   - 净化只输出改写建议，由作者确认后经 `content` 域落库，本域绝不写入章节正文；
   - R1要求实现完整领域闭环，具体服务/接口/SQL见system-design对应分册；代码是否已实现只认验收证据。
6. **智能体域 (`internal/agent`)**：
   - 包含完整的工业级 TAO（Think-Act-Observe）循环执行状态机（`runtime/application/loop`）；
   - 负责意图感知、上下文动态压缩（`promptctx`）、工具调用分发（`tool`）、安全审批门控（`approval`）与循环防死锁（`loop_guard`）；
   - **严格定界**：Agent 只能读取作品上下文并生成决策建议或草稿，绝非事实 Owner，无权私自绕过 `content` / `worldview` 篡改事实。
7. **模型基座域 (`internal/llm`)**：
   - 统一模型接入与流式调度中心，集成多供应商原生客户端矩阵（OpenAI, DeepSeek, Anthropic Claude, Google Gemini, Ollama 等）；
   - 负责高可用调度（`routing`：加权负载均衡、Key 轮换池、并发与 RPM 限流、故障自动熔断降级）；
   - 统一 Token 估算、工具调用累加与 SSE 流式协议输出；不依赖任何业务域。

---

### 1.3 统一 HTTP 与 Transport 技术栈规范 (Gin & Transport Infrastructure)

全代码库严格统一遵循以下 HTTP 接入层与传输技术分工，严禁引入第二套路由、校验或响应封装：

1. **Gin 的使用边界**：
   - Gin 仅且必须保留在 **Transport 层** 与 **技术组合根（internal/app 与 internal/platform/middleware）**。
   - **严禁** 在 Application 层、Domain 层或 Infrastructure 层导入或依赖 Gin 框架。
   - 统一使用原生 Gin 路由树生成的 Allow 机制并按契约补齐排序后的 OPTIONS；禁止手工全量扫描路由。
   - 全局中间件顺序严格锁定为：Contract（生成/校验 X-Request-ID、X-Trace-ID、Traceparent）-> Logger（仅记录受限元数据，绝不记录 Query/Body/Cookie/Auth 头）-> Recovery（关闭 Gin 默认堆栈转储，统一转换为契约 500 JSON）-> ContractCORS（按 Origin 精确白名单与 Credentials 控制）。
   - 可信代理通过 cfg.RateLimit.TrustedProxyCIDRs 统一设置；客户端 IP 统一通过 c.ClientIP() 读取。

2. **请求解析与严格校验体系 (httpguard + strictjson + requestschema)**：
   - HTTP 请求体验证与解码唯一由 internal/platform/httpguard 负责，内部串联 strictjson 与 requestschema。
   - **严格单次读取**：请求体限制在 2MiB 且只读一次，严禁再次读取或回填 Request Body。
   - **严格 JSON 规则**：校验 UTF-8 合法性、重复键检测（Duplicate Keys）、完整 EOF 闭合、最大嵌套深度 64，解码保留 json.Number 精度。
   - **嵌入请求 Schema**：基于全仓 238 个操作生成的 107 个机读 JSON Schema 随二进制嵌入并在启动期离线编译。启动时严格检查所有已挂载路由的映射有效性，未知字段、属性拼写/大小写错误或类型不匹配在接入层直接拦截返回 400，严禁散落手写 binding 标签或反射判空。
   - **无请求体检查**：DELETE 请求与显式无体端点必须通过 httpguard.RequireEmptyBody 校验，至多只读 1 字节并拒绝任何请求正文，严格拦截未声明的 Query 参数。

3. **流式通信 (SSE) 规范**：
   - 生成式逐字推流与 Agent 执行状态流唯一使用 HTTP SSE (text/event-stream)。
   - SSE 写入前必须完整编码整帧，通过 Gin Writer 记录写入状态，并针对 Unwrap 后的底层连接使用标准库 http.NewResponseController 设置 10 秒写截止时间与捕获 Flush 错误。
   - 响应开始后严禁追加 JSON 错误包；客户端主动断线仅释放连接资源，绝不取消持久化后台 Run 状态机；基于 Last-Event-ID 请求头执行可靠断点续传。

4. **实时低频通知 (WebSocket) 规范**：
   - 系统级终态通知唯一使用 WebSocket，路径严格限制为 /v1/platform/events。
   - 握手严格要求已鉴权主体预先申请的 60 秒一次性票据（WS Ticket）并通过 URL 携带，服务端基于数据库原子消费并校验 Origin 白名单；严禁使用 Wildcard 或明文 Token 跨域握手。
   - 废弃并彻底删除旧版广播适配器（如旧 realtime hub/broadcaster）。

5. **文件上传与分片下载规范**：
   - 上传统一经由 Gin 的 MultipartForm 入口，结合 MaxBytesReader 强制执行独立总请求体限制（20MiB 输入限制），严格由 platform_files 持久化并计算 SHA256。
   - 下载逻辑保留授权优先、SHA256 ETag 校验（匹配返回 304 空响应体）、单范围 Range 分片下载（返回 206）与越界 Range 拒绝（返回 416），严禁替换为无授权的静态文件映射。

---

## 3. 核心架构红线与工程铁律

1. **测试代码零容忍规范**：
   - 全代码库严格禁止编写、修改或提交任何 `*_test.go` 或 `*.test.ts` 文件，杜绝各类测试垃圾与维护包袱。
2. **密钥安全与合规规范**：
   - 所有 LLM 的 Provider API Key 严格只允许通过系统环境变量（如 `OPENAI_API_KEY`、`DEEPSEEK_API_KEY` 等）或 Secret Store 注入；
   - 严禁明文入库、严禁打印至任何日志输出、严禁返回至前端响应，对外仅暴露脱敏元数据。
3. **实时通信职责隔离规范 (SSE + WS 双持)**：
   - **SSE 独占请求级生成流**：AI 文本生成、打字机逐字推流、Agent 执行摘要（不公开隐藏推理）及工具调用回显必须使用 HTTP SSE (`text/event-stream`)；
   - **WebSocket 独占系统级低频广播**：仅用于长任务终态推送、全局状态通知与低频通知广播。
4. **全栈时间标准 (Beijing Time Contract)**：
   - 数据库存储一律使用带时区的 `TIMESTAMPTZ`（UTC 存储）；
   - 业务逻辑、自然日统计与排期必须通过 `timeutil` 显式转换为北京时间，严禁使用系统本地裸时间。
5. **单文件体积限制**：
   - 单个手写代码文件严格控制在 **500KiB（500 × 1024 = 512000 字节）以内**，职责过重的文件必须进行垂直或水平合理拆分。依据 [ADR-0037](.agents/notes/reference/decisions/0037-source-file-size-limit-500kib.md)，源码大小阈值适用于既有 archguard 扫描范围（含生成源码）；不改变独立文档长度与冻结治理限制。
6. **零硬编码铁律 (Zero Hardcoding Contract)**：
   - 严禁在前端组件中硬编码静态展示数组（如知识图谱节点/边、大纲树、预设章节正文），所有业务数据必须通过真实 REST API 读写后端数据库；
   - 严禁在多智能体流水线中使用 setTimeout 伪造执行进度，必须真实调度后端 Agent 状态机与 SSE 流式接口；
   - 严禁在代码中写死长篇提示词模板，必须由后端组装器根据前端传递的 Context Assembly 策略在 60,000 Token 上限内动态裁剪组装。
7. **纯净排版与零 Emoji 铁律 (No Emoji Policy)**：
   - 全代码库（Go 源码、TS/TSX 组件、CSS 样式、i18n 词条、接口响应与日志）严格禁止出现任何 Unicode Emoji 字符；
   - 统一使用规范的 Lucide SVG 矢量图标或文字徽章，保障出版级网文连载平台的专业质感。
8. **前端双端完全物理隔离与双端口独立启动铁律 (Dual-App Strict Isolation)**：
   - 前端必须彻底分离为两个独立的应用实例并由两个独立端口分别启动：
     - **创作者工作台 (Creator Studio)**：专属端口 3000，面向小说作者，只呈现创作核心功能（书架、开书、三栏编辑器、大纲、世界观图谱、流水线、拆书），绝无任何运维与模型调度菜单，绝无管理端切换入口；
     - **系统管理端 (Admin Console)**：专属端口 3001，面向系统运维与 AI 治理，只呈现模型大盘、渠道 Key 轮换池、任务策略、运行审计、智能体矩阵、工具台，绝无任何小说创作界面，绝无创作者端切换入口；
   - 严禁在同一个前端展示或通过前端按钮来回切换，必须保持物理隔离与清晰的心智边界；
   - 严格杜绝中英文混杂，全面接入纯净国际化体系（t("...") 驱动），严禁任何裸露中文硬编码。

---

## 4. 项目管理与文档治理 (Project Management & Docs)

本节与第 1–3 节的工程红线同等效力，规约本仓库的**项目管理工作方式**。完整说明见 `docs/project-management.md`。

### 4.1 文档即事实来源 (Documentation as Source of Truth)

| 文件 | 回答什么问题 | 何时必须更新 |
| --- | --- | --- |
| `AGENTS.md`（本文件） | 什么**绝对不能**做 | 新增工程红线时 |
| `docs/architecture.md` | 代码**现在**是什么样 | 增删包、改变接线、引入依赖时 |
| `docs/system-design/18-DELIVERY-AND-ACCEPTANCE-PLAN.md` | R1冻结实施顺序与验收门禁 | 仅正式R1.x变更时 |
| `docs/backlog.md` | **谁在做哪一件**、卡在哪 | 任务状态变化时 |
| `.agents/notes/reference/decisions/` | 当初**为什么**这么选 | 任何架构决策 |
| `.agents/notes/` | 实现**为什么**如此、失败路线与隐含约束 | 非平凡改动同批新增或同步；不替代 ADR / backlog |
| `CHANGELOG.md` | 每个版本**交付了什么** | 版本发布时 |

重构设计顺序：用户明确指令 > `AGENTS.md` > 有效ADR-0016/0017 > `docs/system-design/00-CONTRACT-BASELINE.md` > API/DDL机读契约 > 领域与UI分册。`architecture.md`记录实施事实，`backlog.md`记录任务，二者不覆盖R1目标。
**代码与 `architecture.md` 不一致时以代码为准并立即修正文档；代码与 `AGENTS.md` 不一致时视为缺陷。**

### 4.2 任务规约

- 所有任务必须在 `docs/backlog.md` 登记，使用 `INK-<三位序号>` 编号，**ID 永不复用**。
- 状态机：`todo → doing → review → done`；被外部条件卡住标记 `blocked` 并写明具体阻塞条件。
- 一个任务 = 一次可独立验收的交付；跨多个域的改动必须拆开。
- 开工前必须确认：验收标准可复现、依赖已 `done`、涉及架构红线时 ADR 已存在。

### 4.3 完成定义 (Definition of Done)

任务标记 `done` 前必须全部满足：

1. `go build ./...` 通过；
2. 验收标准逐条复现并留下证据（命令、请求或截图）；
3. 明确交代该改动对"运行路径"的影响（可用 `go list -deps ./cmd/server` 前后对比）；
4. `docs/architecture.md` 已同步；
5. 有架构决策则 ADR 已落盘；
6. `docs/backlog.md` 与 `CHANGELOG.md` 已更新；
7. 未新增 `*_test.go` / `*.test.ts`、未提交明文密钥、单文件 ≤ 500KiB（512000 字节）。

> 因本项目禁止编写测试文件（第 3 节第 1 条），**没有可复现证据的"已完成"一律视为未完成**。

### 4.4 AI 协作协议

- 开工前依次阅读 `AGENTS.md` → `docs/system-design/00-INDEX.md` 与契约基线 → `docs/architecture.md` 实施差距 → `docs/backlog.md` 目标任务。
- 长跨度任务用 **goal** 跟踪（目标必须写明验证方式），步骤用 **todo** 跟踪；同一时刻只保留一个进行中步骤。
- 遇到架构不确定项必须**停下来提问**，禁止猜测实现。
- 发现任务范围外的问题：登记新 ID，不在当前任务中顺手修改。
- 汇报格式：`任务 ID → 状态 → 改了什么 → 验收证据 → 剩余风险`。

### 4.5 架构变更 ADR-First

触碰分层结构、域边界、存储选型、通信协议归属、全局契约或新增对外依赖的改动，**必须先写 ADR 再写代码**（模板见 `.agents/notes/reference/decisions/0000-template.md`）。已接受的 ADR 不得改写，只能新增并标注替代关系。

### 4.6 项目级 Skills 入口

开发与维护任务按需读取仓库级技能：[开发交付](.agents/skills/inklore-development/SKILL.md)、[决策记忆](.agents/skills/inklore-decisions/SKILL.md)、[维护排障](.agents/skills/inklore-maintenance/SKILL.md)。使用说明见 [开发维护指南](docs/development-guide.md)。技能只编排既有规则与 docs，不覆盖本文件、已接受 ADR 或用户明确指令。改动技能及其文档链接后执行 `node scripts/check-project-skills.mjs`。

### 4.7 长期工程记忆

工程笔记规则见 [.agents/notes/README.md](.agents/notes/README.md)。改前检索相关活跃/否决笔记；非平凡改动同批保留背景、真实备选、代价和验证，纯机械改动免写。ADR 负责正式架构裁决，backlog 唯一管理任务状态，notes 不覆盖二者。已归档笔记及其 manifest 既有条目不可改写；使用 `node scripts/agent-notes.mjs check`，归档变动加 `--base HEAD`，CI 对照变更前 Git 基线。检查器不替代对理由与漏记的语义评审。

### 4.8 DSH并行开发子代理

本项目已配置 `inklore-parallel-terra` preset，使用规范与文件Owner见 [.dsh/SUBAGENTS.md](.dsh/SUBAGENTS.md)。主代理按依赖派发、明确独占文件，每批最多4个活跃子代理；这是调度约定，不能假称文件系统隔离或引擎硬并发限额。公共装配、迁移序号、依赖与看板由主代理统一整合。子代理固定GPT-5.6 Terra/high；新对话选择专用preset才生效。

### 4.9 当前状态入口

新一轮工作以R1设计为目标，读取 `docs/architecture.md` 与 `docs/backlog.md` 的当前证据。旧包可达数量、旧任务done与设计文档存在均不能证明R1完成；所有功能按system-design/18重新验收。
