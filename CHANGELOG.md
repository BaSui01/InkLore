# 更新日志（Changelog）

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 的结构与 [语义化版本](https://semver.org/lang/zh-CN/) 规则。

> 说明：仓库在 2026-09-16 之前未纳入版本控制，因此没有更早的历史条目。首次基线提交见 INK-001。

## [Unreleased]

### 双端 UI/UX 设计实施包（INK-259，review）

- 新增 [docs/ui-ux](docs/ui-ux/README.md)：管理端/创作者端现有路由盘点、核心页面布局与交互方案、组件复用和响应式/无障碍/权限验收矩阵。
- 明确与冻结 15/16/17/20 分册、实际业务代码和 API 的边界；高保真设计图及前端功能改造均为后续单独交付，本次没有运行路径变化。


### Agent 模型渠道租户安全与策略冻结（INK-228，待隔离验收）

- Agent 的请求由冻结的 PolicyVersionID 与所属租户的精确 Model ID 选择数据库渠道、协议和上游模型名称；停用、缺失和凭据失败不回退到其他租户或全局密钥；不再于组合根无 tenant 读取 LLM 业务表。
- CreateRun 将策略当前版本读取放入原有 Run/Job/事件 PostgreSQL 事务，用 FOR SHARE 防止发布竞争时产生无授权版本漂移。
- 灵感分析和合规质检改为使用本租户现行启用的任务策略查询 Model ID/冻结版本；缺少策略明确失败，避免复活原跨租户查询。
- PR #19 在临时 PostgreSQL 中已验证双租户渠道与冻结 Run/Job、发布锁竞争；新增环回 HTTP 伪 Provider 验证租户 Key / upstream_model 的实际网络路由及无授权外发，必须在最终 head 通过。付费真实 Provider、计费审计和 Creator 策略目录、兼容 sunset 仍未完成，整体任务维持 doing。


### Go 加密依赖安全升级（INK-258，待独立验收）

- 以 Dependabot PR #28 为版本来源，在 dev 提交 x/crypto 0.48.0→0.52.0，联动 x/net、x/text、x/sys、x/sync 的必要版本与校验和更新；未直接更改业务源代码。
- 加密、认证与网络依赖图变化需要最新 CI 和隔离服务的实际认证/租户回归证据。稳定 private 分支暂不直接合入，仍须受保护发布 PR。

### 在线隔离 CI / Supabase 预检（INK-255，待验收）

- 增加 GitHub Actions 临时 PG17+pgvector/Redis7 集成作业，验证 R1 迁移、幂等重放及后端构建规则；不读取真实 Supabase 凭据。
- 增加可信人工触发的 Supabase 私有 schema 只读验证工作流，需另行配置最小权限数据库连接与 GitHub Environment。
- 东京 pgsql 的 inklore_ci 私有 schema 已完成17组 R1 迁移（80张业务表，public 零业务表），pgcrypto/vector/pg_trgm 统一位于 extensions；已执行独立的私有角色权限覆盖并验证 Data API 角色无 schema 访问权。
- Supabase 四个不可变触发器 search_path 已固定并通过安全 Advisor 零告警验证；独立脚本保留原 R1 迁移 SQL 与 SHA256 不变。临时 CI 新增实际 Creator 注册/会话持久化 HTTP 验证，按最新 PR head 检查证据。
- 临时 PostgreSQL 模拟 Supabase 已存在的 public 表/序列，修复并验证旧权限迁移的对象授权残留，最新隔离 Actions #41490710935 全绿。修正版暂未在共享 Supabase 再次执行。

### INK-228 隔离 PG17/Redis7 数据库结构冒烟门禁（review）

- 新增面向 Agent/LLM 变更的临时 PostgreSQL 迁移与租户外键检查，固定当前 17 个迁移基线、重复迁移和无持久化服务凭据验证；不执行真实模型或 Supabase 写入。
- 静态 Schema 门禁不代表 INK-228 的真实多租户路由、冻结策略和 Worker 行为已完成验收。

### Qodana 工作流有效性修复（INK-257，review）

- 将无效的 step `secrets` 条件换成 Token 可用性布尔输出，保留凭据只在受限环境变量中；更新 push 到 private/dev，令工作流可正常解析。
- 保留缺少 QODANA_TOKEN 时的安全跳过行为，扫描仍须真实 Token/平台执行验收；不将跳过视为安全通过。


### GitHub 协作与依赖维护（INK-253，review）

- 新增 Issues 表单、Dependabot 四类依赖周更配置、只读仓库健康周报工作流，准备 Wiki 首页与开发/架构/治理导航。
- 建立 Projects 看板与 GitHub 页面入口；不修改业务运行路径。配置进入默认分支前周报定时任务和依赖版本更新不声称已启动，Wiki 需独立首次发布与复核。


### INK-254：安全自动合并门禁加固（review）

- 自动合并收紧为普通文档白名单和非敏感源码显式授权，默认分支 private 仍需人工合并。修复 Draft/标签撤销、旧 head 事件竞态和重命名原路径覆盖；不绕过 GitHub 必需 CI 或 Review 对话，实际事件行为仍待复核。

### 公开仓库前的凭据安全准备（INK-252，blocked）

- 将包含宿主机实际 PostgreSQL/Redis 共用密码的本地环境记录替换为可公开的无凭据开发模板，同时移除机器专属 DBX 连接信息。
- 已提交过的凭据仍可能存在于 Git 历史与 PR diff，必须先轮换、再完成历史安全核查；此次修改不宣称历史已清理，也不改变仓库可见性。

### pnpm 与 dev 持续集成配置（INK-248，待完整验收）

- 前端 CI 改用固定 pnpm 11.25.0、已有 pnpm 锁文件及 frozen-lockfile 安装，保留 Creator/Admin 构建；push 增加 dev，保留 main 与 PR 触发。
- 验证指南同步 pnpm 命令和现有 i18n 目录；未修改依赖、锁文件或业务运行路径。完整构建与远端 CI 尚待独立验收。

### 管理端渠道弹窗精简（INK-245）

- 移除接入模型渠道弹窗的供应商选择框；继续根据接口协议自动匹配或注册供应商，编辑已有渠道时保持原绑定。无后端契约或既有数据变更。

### 旧库迁移安全门禁（INK-244，阻塞于旧库与身份规则）

- 替换旧库读取样本与目标库空落库占位：GORM 只读快照核对旧表并抽取，目标落库对完整且可信数据才允许空库事务插入及计数验收；样本 LIVE、缺少源库、未实现 verify-only 均明确失败。当前只有 R1 数据库，旧结构无用户身份且缺新版模型/Agent 必填字段，因此不执行旧库迁移，不宣称 LIVE 迁移可用。
- 运行时 API 契约补齐平台项目日志接口；新后端健康/就绪200、匿名401，管理端已有会话实际查询200，DBX只读确认日志按租户写入 PostgreSQL。跨租户、保留清理和断库故障注入仍待验收。

### 应用数据库访问统一 GORM（INK-243，待运行验收）

- 移除遗留迁移工具独立的 `sql.Open("pgx")`，连接改走平台 GORM PostgreSQL 封装。业务数据访问继续经 GORM；底层连接池、事务选项、空值类型及版本化迁移 SQL 不属于另一个业务持久化驱动。旧库迁移现在接入真实读取和受限写入，但不可兼容数据的零损迁移仍被安全门禁阻断（INK-244）。

### 智能体 CRUD 与右上角通知修复（INK-242，待运行验收）

- 智能体定义删除新增对运行、运行节点及引导创建会话的历史版本引用检查；已有引用应返回 409 而不是数据库外键 500，保留配置和历史，可选择停用。管理端主要页面反馈统一为右上角高于确认框的可关闭通知，字段错误仍保留在表单，删除失败不自动重试；真实服务重启后的 CRUD 与并发竞态仍待验证。

### 审计检索与项目日志（INK-240 / INK-241，待运行验收）

- 管理端审计日志改为按动作和北京时间范围筛选、游标前后翻页与当前页关键词搜索；主表展示可解析的操作人姓名与业务类别，不直接堆砌 ID，导出前明确校验时间范围；不提供审计增删改。
- 项目日志采用 PostgreSQL 独立白名单投影记录已验证租户的 HTTP 结束事件，提供 operator/auditor 同租户只读筛选与分页页面；写入失败不阻断业务，标准输出仍保留。无租户启动、登录前及后台 Worker 日志不进入该页面；真实授权查询和异常注入待验收。

### 管理端日志链路（INK-239，待运行验收）

- 全系统 HTTP 请求增加可关联的开始/结束结构日志和安全错误码；LLM 管理目录写删除按鉴权、校验、锁、仓储与审计暂存输出固定阶段，禁止原始正文、密钥和 SQL 入日志。保持原有路由与响应契约，真实 Admin 登录后 PG 日志关联尚未复验。

### 正在实施（INK-236）

- Creator 首页开书入口进入独立 Agent 引导页并发起真实逐轮访谈：Agent 提出问题及建议，作者可自由输入并在提案后编辑作品信息；仅在作者确认时调用 Content 创建作品。
- 新增持久 Agent guide session、固定 `ask_user` / `propose_novel` 工具参数校验、严格 API Schema 与迁移 000014。本地 Go/前端构建通过；真实登录后 Provider 调用和浏览器问答验收待完成。

### 正在实施（INK-237）

- 管理端渠道与密钥拆分 CRUD；渠道编辑不再重复创建密钥，关联密钥存在时禁止误删渠道；`llm.resource_in_use` 与版本冲突改为双语行内错误。Admin 构建通过，真实密钥增删改与活动任务保护待登录后复验。

### INK-238 验证码修复

- Admin 登录失败后自动刷新一次性图形验证码并清空旧答案；手动更换验证码也同步清除旧输入，避免把过期答案与新验证码 ID 组合提交。双端构建和 i18n 检查通过；真实验证码提交待现场确认后验收。

### INK-238 登录实测

- Admin 登录失败后自动刷新一次性图形验证码并清空旧输入；手动更换验证码时也清除先前输入，避免新旧验证码答案错配。共享 UI 双端构建通过，真实浏览器验证码提交待复验。

### 已交付（Delivered，尚未发版）

**INK-232 / INK-233 / INK-234：六字符密码、root 双标识登录与全局管理员双权限**

- 密码下限统一为6个Unicode字符，前端、Go身份服务与严格API Schema一致；72 UTF-8字节上限和bcrypt cost 12保持不变。
- Admin和Creator均支持邮箱或`root`用户名登录；全局`super_admin`逐tenant显式切换，root在现有tenant具备Creator和Admin成员角色，业务Repo仍保留tenant及Creator owner授权。
- DBX在单事务创建全局root并写入活动tenant membership及审计；数据库只保存bcrypt哈希，不在源码或文档保存密码。
- Go/双端构建、HTTP双端登录与tenant切换、旧session撤销、密码5/6 Unicode边界、Creator owner拒绝、事务化新tenant membership投影、生成Schema、i18n与Go架构护栏通过。

**INK-235：Creator 登录支持 root 用户名**

- Creator 登录表单接受邮箱或用户名 `root`；API 仅解析登录身份，Creator session 仍要求活动 creator membership，创作资源继续执行 tenant 与 owner scope。四种Creator/Admin用户名或邮箱组合均经真实HTTP登录验证，Chromium端到端完成Creator登录并跳转首页。
- 修正共享session将空 `tenant_id` 序列化为字符串，以及双端登录适配丢弃验证码ID的问题；严格Schema请求现仅发送已填写的tenant和成对验证码字段。
- 注册、找回密码与邮箱管理继续要求邮箱；Creator-only 身份不能进入 Admin。

**INK-223：系统预置智能体种子、DAG工作流与人机闭环审阅**

- 实现 admin seed 播种引擎，为租户初始化注入 Provider、Model、3类核心任务策略（inline_polish、novel_architect、content_polish）、3类核心 Agent 定义与标准 Kahn DAG 工作流（workflow_inline_polish）。
- 创作者端 SDK 补齐目录查询与 CreateRun 契约对接，落地 ArtifactReviewModal 呈现原文与 AI 建议 DiffView 对比，由作者显式确认采纳替换正文并落库。

**INK-222：小说核心创作链贯通与创作者端硬编码消除**

- 彻底消除 OutlineRoute、WorldviewRoute、AnalysisRoute 中遗留的全部静态假数据数组，全面对接后端 /v1/outline、/v1/worldview 真实 API。
- OutlineRoute 支持大纲节点与细分场景加载及创建，点击直达正文编辑；WorldviewRoute 动态构建真实角色与世界观词条的 Cytoscape 力导向图谱并支持创建角色；AnalysisRoute 基于 getPacing 真实数据渲染张力曲线。
- 在 EditorRoute 落地 ConfirmChapterModal，获取权威世界观版本并在同一原子事务中调用 confirmChapter 将章节置为 confirmed 并持久化事实账本。

**INK-221：Creator 编辑器纯文本规范、409版本冲突核对与保存恢复**

- 彻底消除 TipTap 富文本框架与 HTML 数据模型，全面对齐 00契约基线§2 与 03分册§2 的 UTF-8 纯文本、LF 换行规范与 Unicode 码点字数统计（不含空白）。
- 正文输入由原生多行文本框 PlainTextEditor 承载，提供纯文本划词 AI 动作工具栏与 IME 拼音输入状态防误触；草稿下载直接导出为纯文本文件。
- 正文保存遇 409 `content.version_conflict` 时自动弹出 ConflictResolutionModal 基于 DiffView 进行三方核对，支持作者选择采用服务器版本或以本地草稿覆盖提交。
- 作品及章节分资源读取不再以网络错误造出空列表或空正文；保存失败保留 dirty/正文/revision，受限错误提示与草稿下载持续可用。
- 冻结写入载荷/幂等键并仅同键重试未知结果，禁跨章迟到正文覆盖；移除 AI 失败后的定时伪正文。

**INK-220：传输错误传播与双端安全诊断**

- 管理端读取失败保留真实错误，五处页面以 ErrorPresenter 提供持久重试并保留既有数据；移除默认管理端凭据与隐式自动登录。
- SSE/WS、会话、创作/管理错误输出收敛为受限机器诊断，不再输出原帧、原始异常或正文；后端现有 JSON/SSE/WS 边界保持不变。
- 启动真实 cmd/server（端口 29080）并探测健康与就绪探针，真实 HTTP 注入验证 401/404/405/400 错误封包；双端浏览器实测登录与安全求值通过。Go/双端构建、i18n 945/945、架构红线 R1–R5、notes/skills 全绿通过。

**INK-218：LLM 渠道协议、图片适配与凭据解析收敛（review）**

- 依 ADR-0025 与 ADR-0026 将 R1 公开 Provider 与基础设施内部 transport 分离，修复渠道池、数据库渠道和图片驱动因默认 OpenAI 协议造成的错误分派。
- 已移除已退出 Imagen `:predict` 路径；OpenAI 图像编辑改为真实 multipart 上传；管理端只接受环境变量 SecretRef，不再接收 API Key 正文。未具备脱敏上游凭据，真实供应商验收待补。

**INK-217：LLM 供应商官方协议来源复核与文档纠偏**

- 以 2026-09-20 供应商官方页面逐端点复核 OpenAI Chat/Responses/Images、DeepSeek、Anthropic、Gemini generateContent/Interactions/图片、Ollama、Z.AI GLM 与 Kimi，重写协议规范并保留 24 个可追溯的 HTTPS 来源链接。
- 明确 R1 的五种公开 Provider 与内部传输模式的边界；去除未核验的“最新模型”清单、错误的 OpenAI Responses WebSocket、错误的 Gemini Interactions 请求对象及已退出 Imagen `predict` 合同。
- 文档记录当前生图实现与官方合同的两项差距，后续由 INK-218 修复：Gemini Imagen `:predict` 退出，以及 OpenAI 图生图需要真实 multipart Images edits 实现；本任务不修改运行代码或外部调用行为。

**INK-216：双端输入法（IME）组合状态与快捷键完善**

- 依照系统设计 RE08（IME 不提交半字）与 DS03（IME 不误触发快捷键）红线，在 `@inklore/ui` 中新增 `isImeComposing` 与 `useImeComposition` 工具，多维度判定原生与浏览器虚拟键码 229；
- 增强 `Input` 与 `Textarea` 基础组件，内置 IME 生命周期跟踪并支持防误触的 `onPressEnter` 与 `onCtrlEnter`；
- 创作者章节写作区接管 ProseMirror 的 `compositionstart` 与 `compositionend` 事件，输入法组合期间主动撤销自动保存计时器，杜绝拼音半字入库，选字结束后恢复计时；
- 支持全局及编辑器 `Ctrl/Cmd + S` 快捷保存正文并阻止浏览器另存为；
- 修复首页开书弹窗（`CreateNovelDialog`）与灵感输入框（`HomeRoute`）在中文拼音输入选词敲 Enter 时的误提交，支持快捷 `Ctrl/Cmd + Enter` 提交；
- 修复管理端渠道弹窗（`ChannelModal`）在输入自定义模型拼音时敲 Enter 误添加的缺陷。

**INK-180：内部维护与未配置模型错误归属核对**

- 核对系统设计文档中提及的 `llm.provider_not_configured` 与 `worldview.projection_corrupt` 的契约边界：确认为内部调度原因与维护作业防御性错误，不新增公开契约错误码。
- 分别对外映射为已公开的 `llm.capacity_unavailable` (503) 与平台通用 500 契约，保证公开错误目录（98 个定义）与冻结 manifest 绝对稳定。

**INK-212：修复 i18n 检查器顶层导出误判与路径漂移**

- 修复 `cmd/i18ncheck` 词法状态机，增加对象字面量作用域判断（`len(stack) > 0`），消除文件顶层 TypeScript 类型注解（如 `: Record<...>`）与导出标识符（`zhCN` / `enUS`）被误判为词条的缺陷。
- 同步更正 CI 与 `scripts/check.ps1` 中的多语言文件路径为 `frontend/packages/i18n/src/locales/`，各语言词典对比顺利通过。

**INK-207：共享业务 SDK 隐式登录与默认凭据清理**

- 移除 `@inklore/transport` 中 ApiSDK 的 `ensureAuthenticated` 方法与内置测试默认凭据（`author@inklore.local` / `Password123!`）。
- 移除所有业务方法内的隐式调用，未登录行为严格由底层 `ApiClient` 捕获 401 并触发用户登录引导，消除安全隐患与测试逻辑污染。

**INK-208：补齐 Agent/Inspiration/Compliance 完整版本化迁移链**

- 依照 ADR-0024，将冻结在 docs/system-design/data/ 的 Agent、Inspiration、Compliance 与 Permissions 最小权限 DDL 规范化为 `migrations/r1` 下的 000006 至 000009 迁移文件。
- 完整覆盖系统设计权威 74 张业务表，与 `check-design.cjs`、`cmd/migrate` 和 `database.Verify` 彻底闭环；隔离库全新 up 及既有主库增量 migrate 均验收通过，废除手工加载 SQL 的例外路径。

**INK-215：Creator普通路径路由**

- 首页、登录和创作工作台改用History路径，旧Hash入口自动转换；登录来源和编辑器作品ID由URL恢复，支持前进后退。
- Vite深链访问与刷新、Nginx静态托管/API分流、Go与双端构建验证通过，见[验收](docs/acceptance-INK-215.md)。真实认证成功回跳仍待补验。

**INK-214：独立登录页面与来源回跳**

- Creator所有登录入口跳转独立页面并保留returnTo，移除登录弹窗；首页灵感跨路由保留，成功认证回到来源页。
- Go与双端构建、匿名路由浏览器检查通过；本地身份接口500，真实登录成功回跳待补验。

**INK-213：首页无需登录即可浏览**

- 公开根路径和首页；创建时弹出登录，匿名不加载私人书架；会话初始化失败不阻塞首页，工作台仍保留鉴权。
- Chrome匿名访问/刷新/登录入口/灵感留存/受保护跳转已验证；本地身份接口500下登录后回归待补验。

**INK-206：创作者文学首页（review） / INK-211：会话初始化并发修复**

- 首页按用户参考重做宋体双栏首屏、纸页钢笔插画、暖金色操作、真实书架、功能区与指南；支持双语、明暗主题和手机导航。
- 真实作品游标分页、已加载数据搜索、创意开书与指定作品编辑入口闭环；加载、空态、错误、过期快照与离线明确区分，无模型/正文/统计占位。
- 会话初始化共享在途 Promise，消除开发模式重复刷新造成的登录退出；不改变原账号密码或服务端安全规则。
- Go 与双端构建、真实 Chrome 创建/留存/分页/失败恢复通过，[截图与验收](docs/acceptance-INK-206.md)已保存；i18n检查器既有误报登记INK-212，真实组合词条一致。


**INK-205：管理端视觉收敛与真实模型协议接线（待复核）**

- 管理端模型配置页移除非必要副标题、数量徽章和主题文字标签；仪表盘/渠道页统一浅色背景、紧凑工具栏、卡片层级、表格边界、悬停与键盘焦点状态，刷新等工具动作使用图标按钮并保留无障碍名称。
- LLM ProviderRouter根据运行配置接入OpenAI Chat Completions、OpenAI Responses内部模式、DeepSeek/Ollama兼容协议、Anthropic Messages和Gemini generateContent及流式SSE；API密钥只从环境变量或渠道解析得到，不写入日志或响应。
- 组合根将真实Gateway接入Agent运行、灵感分析和合规审阅/改写；移除这些生成路径的固定成功文本、固定用量和随机策略ID。无已发布策略或无Provider时明确失败。
- Agent资源、队列和文件适配器已接入真实领域/平台服务：资源 revision/hash 来自实际查询并在审批前复核，jobs 使用 `platform_jobs` 租约状态，灵感文件使用平台文件读写；不再返回固定摘要、样本文本或伪造任务 ID。
- `go build ./...`、`npm run build:admin`、`node scripts/agent-notes.mjs check`、`node scripts/check-project-skills.mjs`与目标文件`git diff --check`作为验收证据；真实模型凭据、PG登录后浏览器截图和上游网络响应待部署环境复核。

**INK-189：管理端渠道页首屏布局与视觉层级优化（待复核）**

- 删除模型渠道页顶部独立标题、副标题、数量徽章和页头操作区，保留刷新与新增渠道操作并收纳至首行筛选工具栏。
- 统一管理端默认浅色主题，收紧筛选控件、表格圆角、边框、行高与表头层级，补充宽表最小宽度约束，改善桌面与窄工作区的扫描体验。
- `npm run build:admin`、`go build ./...` 与目标文件 `git diff --check` 通过；真实 API 未启动，渠道列表登录后浏览器截图仍待复核。

**INK-188：Go 1.27.1 工具链与 Gin 1.12 维护升级（待复核）**

- Go 模块基线升级到 `go 1.27.0`，固定开发与构建工具链为 `go1.27.1`；Docker 构建阶段同步使用 `golang:1.27.1-alpine`。
- Gin 保持已锁定的 `v1.12.0`，避免无关的全量依赖升级；当前 HTTP 层继续使用 `gin.New`、自定义中间件、严格 JSON 解码、统一响应、SSE/WS 分工与标准库优雅停机。
- 任务验收将以全仓构建、静态检查、架构可达性、Skills/notes 检查和 `git diff --check` 为依据；不新增或修改测试文件。

**INK-187：修复 Vite 过期依赖预构建缓存**

- 重启本地 Creator/Admin Vite 并使用 `--force` 重建依赖预构建，恢复 Cytoscape 与 Tiptap 模块加载。
- 复核本地 `.env` 的 `29000/29001` 仅是开发端口覆盖，未改变 R1 默认的 `3000/3001` 双端契约。
- 双端入口与五个依赖的当前 hash URL 返回 HTTP 200，`build:creator` 与 `build:admin` 通过。

**INK-186：修复 Qodana 高风险静态检查结果（待复核）**

- 针对 Qodana 报告中的空值解引用、资源关闭、错误处理、不可达代码和枚举分支进行逐项修复。
- 任务完成前不删除作为 R1 领域契约保留的未使用导出符号；静态检查结果与行为修复分别验收。

**INK-185：业务域目录扁平化与全仓过渡命名消除**

- 消除全部 7 个业务领域（`content`, `outline`, `worldview`, `inspiration`, `compliance`, `agent`, `llm`）内部的 `r1` 嵌套子包，统一各域的四层架构与包声明规范。
- 统一 `internal/agent/domain` 包名为 `domain`，消除 `enginedomain` 冲突；统一 `internal/llm` 领域各层包名，将生成用量重命名为 `GenerationUsage` 以与 R1 审计实体 `Usage` 区分。
- 清理 `internal/llm/transport/http` 未挂载的旧处理函数，更新组合根 `internal/app/domains.go` 与适配器装配路径。
- 修复 `frontend/packages/i18n/src/locales/auth.ts` 语法缺失，恢复双端前端生产构建。
- 全仓 `go build ./...`、`cmd/archguard`（R1-R5 全绿）、`build:creator` 与 `build:admin`、Skills 校验及工程笔记检查全套通过。

**INK-184：清理废弃 legacy novel 代码与消除过渡命名**

- 物理删除旧 `internal/novel/` 粗粒度混合业务包（domain、application、infrastructure、transport/http 四层共 30+ 文件）及 6 个废弃适配器。
- 合并并消除 `internal/platform/config/r1.go` 与 `BootstrapR1` 过渡命名，统一收敛为平台纯净标准配置加载器 `config.Bootstrap`。
- 清理 `internal/app/app.go` 中的 `newLegacyWithConfig` 死代码与已废弃字段，`cmd/archguard` 纯化为标准 7 业务限界上下文。
- 拆分管理端 `AgentGovernanceRoute.tsx` 独立模态框组件，手写单源码文件体积严格控制在 50KB 以内。
- 全仓 `go build ./...`、`cmd/archguard`（R1-R5 全绿）、Skills 校验及工程笔记检查全套通过。

**INK-182：管理端导航语义与智能体页底部操作区优化**

- 收敛管理端导航和智能体页签文案，去除重复页头与无数据来源的循环守卫指标；保留真实资源计数、查询和操作入口。
- 三个资源列表的分页固定于管理工作区底部，内容区保留安全留白；窄工作区下页签、查询和分页改为紧凑可读布局。

**INK-163 / INK-164 / INK-165：首批内容、大纲与事实领域**

- 三个领域四层、81个操作与主组合根接线；冻结DDL原样迁移。真实PG/HTTP逐操作Schema、并发CAS、撤权重放及故障整体回滚通过。
- 内容：正文/版本/目录/proof同事务；大纲：三级节点、场景、伏笔、RR只读节奏；世界观：图谱、整批事实冲突、不可变ledger与维护重建。
- 补齐RR根事务模式、trash授权与typed冲突详情，修复实测时间精度、UTC、错误映射及审计action集成问题；160/161共50组回归通过。
- 验收记录：[163](docs/acceptance-INK-163.md)、[164](docs/acceptance-INK-164.md)、[165](docs/acceptance-INK-165.md)。前端、确认用例与大容量运行演练仍留后续任务。

**INK-167：LLM R1目录与管理transport进行中**

- 000005_llm迁移、Catalog/SecretRef/strict slice、32个冻结管理路由、Budget PG CAS/日桶基础服务和Redis admission四文件已接入或编译；真实Budget GET/PUT/Creator403、Redis recovery/Lua/fail-closed、100并发及1:3 SWRR通过，当前仍4 GAP，driver和完整Gateway验收未完成，任务保持doing。

**INK-166：章节确认与跨域事实事务**

- 新增章节确认HTTP操作与content-owned WorldFactPort，通过app adapter调用worldview同一UoW；空提议world+1、事实来源proof、双CAS、幂等重放、撤权和16表故障回滚证据通过。
- 真实PG/API 9组确认验收及Schema覆盖通过；浏览器未宣称，INK-163原27操作覆盖保持独立。

**INK-162：平台文件、租约、Outbox、通知与观测**

- 文件、租约、Outbox/Inbox、WS、Schema与观测六组真实组件验收通过，文件/状态/WS已接线；实际操作员重放cap与系统审计验证通过。
- ADR-0019已Accepted并应用；304空响应、派生contracts、freeze manifest、canary和真实conditional_download通过。通用jobs机制真实验收；Agent/Content领域消费者留给171/172，采样维护和备份演练留给176。任务已完成。

**INK-161 / INK-178：R1.1身份、成员与审计**

- 按用户明确批准落ADR-0018，保存原冻结清单；补齐四项契约差异，错误95→97，DDL不变。
- 15个后端操作接线；本人改密全会话吊销、审计失败回滚、新会话幂等重放；审计NDJSON先校验31天/10000条后完整返回；405 JSON与精确Allow。
- 24组真实PG/Redis/HTTP与15操作Schema覆盖、26组INK-160回归通过；既有4组双端代理验收保留。真实Chrome与产品前端未验收，留在后续前端/端到端门禁。[证据](docs/acceptance-INK-161.md)。


**INK-160：R1平台事务、授权投影与幂等基座**

- 建立独立R1迁移流与组合根，旧路由不挂载；迁移名称/checksum门禁、事务级迁移互斥、独立配置装载和PG必需启动。
- 实施UoW/Recoverable、scope历史证明与CAS、事务幂等及安全HTTP基座；26组真实PG/HTTP正反例、全仓构建、架构/冻结/技能/notes护栏通过。[验收证据](docs/acceptance-INK-160.md)。
- 冻结设计保持不变；后续操作的契约差异记录于[INK-178](docs/r1-contract-differences-INK-178.md)。

**INK-144：DSH并行开发子代理配置**

- 新建并安装InkLore专用PTC preset，通用spawn/fork及11个角色固定GPT-5.6 Terra/high；单层委派，后台可继续。
- 增加目录Owner、依赖门禁、4个活跃子代理的协调规则和只读审阅工具白名单；公共文件由主代理整合。
- 原生preset发现与配置校验通过；项目保留配置源和验证脚本。保持其他项目默认与已开始对话不变，新对话选择专用preset启用。未执行上游模型请求或业务重构。


**INK-141：R1完整重构设计定稿与旧文档清理（纯设计）**

- 将system-design收敛为唯一施工入口，重写七域功能/应用服务/Port/事务/状态/仓储设计，冻结双端19组作者页面与17组管理页面，以及共享组件与状态契约。
- 新建类型化OpenAPI分片与确定性编译工具：152路径、233操作、95错误码；真实SQL设计74张表及权限脚本；新增SSE/PG作业恢复、CAS/幂等/审批/预算/源修订一致性规范。
- 按cleanup-manifest实际删除30个旧文件，移除旧API目录、旧方案archive、重复DDL/CONTRACT、旧路线图与发布方案；治理文件仅保留执行事实/任务/决策，ADR原位不搬迁。
- 新增ADR-0016/0017与工程因果笔记；旧兼容提案0015不再采用。
- 设计编译及检查、Skills链接、notes、go build与diff检查通过。SQL未连接PG执行，业务代码/数据库迁移/浏览器实现未在本轮交付；R1功能实施按INK-160–177逐项验收。


**INK-143：长期工程记忆**

- 增加 `.agents/notes/` 四生命周期与六分类，明确 notes / ADR / backlog 分工，提供提案、已实施与否决模板。
- 三个项目skills及AGENTS接入改前检索、非平凡变更留因果、同批更新与归档流程。
- 新增无依赖笔记校验/归档工具，验证结构、任务关联、本地链接与归档哈希；CI对照变更前Git基线保护已有封印。
- 首篇流程笔记保留真实备选与验证，业务运行路径不变；证据见 [INK-143](docs/notes-delivery-INK-143.md)。

**INK-142：项目定制开发维护 Skills**

- 新增仓库级 `inklore-development`、`inklore-decisions`、`inklore-maintenance`，融合核心 docs 的按需阅读、七域边界、ADR 历史保护、真实验收与维护流程。
- 新增文档路由、验证矩阵、验收/复盘模板与来源适配说明；沿用 decisions 和唯一 backlog。
- 新增无第三方依赖的技能文档校验器及独立 CI 检查，更新 AGENTS 与文档治理入口。
- 本任务不修改业务运行路径，验证范围与结果见 [INK-142 交付记录](docs/skill-delivery-INK-142.md)。

**M12 · ADR-0014 七域架构治理与大纲域抽取（INK-114，已交付）**

- 新增 [ADR-0014](docs/decisions/0014-domain-boundary-expansion-to-seven-contexts.md)：业务限界上下文由 3 个（`novel`/`agent`/`llm`）扩张为 7 个（`content`/`outline`/`worldview`/`inspiration`/`compliance`/`agent`/`llm`）。**Supersedes ADR-0001 第 2 条**，ADR-0001 顶部已加替代说明（正文按「已接受 ADR 不修改」约定保持原样），`docs/decisions/README.md` 索引同步
- 更新 `AGENTS.md`：§1 架构图改为 7 域拓扑并标注域间单向 DAG；§1.2 域清单、四层骨架义务、跨域红线与唯一受控例外（跨域只读值类型）；§2 领域职责按 7 域重写，事实权威重新划界（作品与正文归 `content`，设定与事实账本归 `worldview`）
- 更新 `cmd/archguard`：`businessDomains` 由 3 域扩为 7 域（`novel` 暂留为过渡容器并加 TODO），R1 跨域护栏即时覆盖新域
- **抽取 `internal/outline`**（ADR-0014 strangler 步骤 2，11 文件 / 4 层）：`domain`（`OutlineNode` 及其中间过程值对象 + `OutlineNodeRepository`）、`application`（`OutlineService` + 消费方自定义的只读引用端口 `ContentReference`）、`infrastructure`（SQL 与内存两档实现）、`transport/http`（独立 `Handler` 与 `RegisterRoutes`，复用 `/v1/novels` 下的 `outline` 子路径，端点不变）
- 新增组合根适配器 `internal/app/adapters/outline_content_reference.go`：把 content 侧仓储适配为 outline 声明的只读端口，只做最小投影、不暴露任何写方法；反向依赖由 content 侧端口 `internal/novel/application/ports/outline_reader.go` 声明
- 新增 `internal/platform/sqlscope/helpers.go`：把 `ErrDatabaseUnavailable` / `TenantFromContext` / `NullableString` / `StringOrEmpty` / `CountWords` / `DefaultTenantID` 上提为跨域共享实现，避免 7 个域的 PostgreSQL 仓储各自复制一套而漂移
- 验收证据：`go build ./...` 通过；`go vet ./...` 无输出；`gofmt -l internal cmd` 为空；`cmd/archguard` R1–R5 全绿；`-reachability` 非 cmd 包 **66/66 可达、不可达 0**；服务启动至 `listening`（gin 路由注册无冲突）

**M12 · 待办（INK-136 ~ INK-140）**

- 抽取 `internal/worldview`（INK-136）、抽取 `internal/content` 并删除 `internal/novel`（INK-137）、建立 `inspiration`/`compliance` 空骨架（INK-138）、前端 `domains/` 按 7 域重组（INK-139）、设计文档包路径与证据同步（INK-140）

**INK-124：功能与异常恢复详细设计（纯文档）**

- 新增14–18五份分册：源码核对的错误码与异常恢复、创作者功能UI/UX、管理端功能UI/UX、设计系统与无障碍、分阶段实施与跨页面验收。新增字段与能力明确标为提案。
- 更新文档入口、修正设计权威顺序与失效链接，为旧数字错误码/OpenAPI/SSE目标契约添加与运行代码不一致的兼容性说明。
- 新增 ADR-0015（Proposed）：保留字符串code、结构化恢复、版本前置与幂等设计；遵守更新后的七域 ADR-0014，不变更运行协议。
- 登记 INK-125–135 后续实施项；本轮未修复运行代码，未执行数据库迁移。构建与文档检查证据见 docs/backlog.md 的 INK-124。

**M12 设计文档体系加固（纯文档，未改动 Go/TS 代码）**

- `docs/system-design/02-TECH-STACK-AND-NAMING-CONVENTIONS.md` 升级为 v3.1：补齐运维技术栈（OpenTelemetry / Prometheus / Grafana / Nginx / Docker）、HTTP 请求头契约、统一错误响应体与 11 条 HTTP 状态码映射、Prometheus 指标命名规则（`inklore_{子系统}_{指标名}_{单位}`）、结构化日志必填字段与脱敏红线
- `docs/system-design/03-DIRECTORY-STRUCTURE-AND-MODULARITY.md` 升级为 v3.1：新增 `deploy/` 运维目录树，前端五层组件化体系（Pages / Domain Organisms / Molecules / Atoms / Design Tokens），SSE 增量渲染性能红线（`useRef` 缓冲，禁止全局 state 高频更新），后端四层依赖与跨域适配器唯一通道
- 新增 `docs/system-design/13-DEVOPS-OBSERVABILITY-AND-OPERATIONS.md`：部署拓扑、`/healthz` 与 `/readyz` 语义红线、优雅停机五阶段（30s 窗口）、日志/指标/追踪三信号、8 条告警策略、备份与恢复 RPO≤5min / RTO≤30min、发布回滚与数据库迁移兼容性规则、3 个故障处置 Runbook
- 新增 `docs/system-design/domains/` 领域详细设计分册（7 域 + 2 份 LLM 网关分册）：每份含限界上下文、领域模型与聚合根、状态机迁移图、关键业务用例与验收标准、生产级 PostgreSQL 16 DDL（CHECK/UNIQUE/FK 约束 + COMMENT ON + 索引）、索引设计意图表、性能容量设计、接口契约表、错误码表、前端组件与状态设计
- 新增 `docs/api/openapi.yaml`（OpenAPI 3.1）：51 条路径 / 73 个操作 / 134 个 Schema / 10 类标准错误响应；自检通过 `$ref` 悬挂 0、`operationId` 全局唯一（73/73）、全部路径与 Schema 均被入口引用、无制表符
- OpenAPI 契约按域模块化拆分以满足单文件 50KB 红线：入口 `openapi.yaml`（19.4 KB）经标准外部 `$ref` 关联 `paths/creator.yaml`（26.0 KB / 30 条）、`paths/agents.yaml`（8.4 KB / 12 条）、`paths/admin.yaml`（6.1 KB / 9 条）、`components/schemas.yaml`（37.7 KB / 134 个）、`components/common.yaml`（2.5 KB）；共 445 处 `$ref` 全部可解析
- 领域详细设计分册中 4 个超出 50KB 的域（设定图谱 65KB、合规 83KB、灵感 89KB、智能体 102KB）按「主文档 + DDL 分册 + 契约分册」三册拆分，共 17 份文件，拆分后单文件最大 47.6KB，内容完整无损
- 新增 `docs/api/sse-streaming-contract.md`：SSE 生成流完整契约（8 类事件、序列不变量、心跳、断开与取消语义、资源释放红线）与 WebSocket 广播契约（5 类主题、封包结构、断线重连）
- 新增 `docs/api/README.md`：契约治理铁律、统一响应封包、HTTP 状态码映射、错误码 10 个段位规划与 21 条核心错误码总表、双端调用范围约束、契约变更流程与自检清单
- `docs/system-design/00-INDEX.md` 与 `docs/README.md` 同步登记全部新增规范；`docs/backlog.md` 新增 P8 里程碑 10 项任务（INK-114 ~ INK-123）

**本轮暴露的阻断级架构冲突（已登记 INK-114，状态 blocked）**

- `AGENTS.md` 第 1.2 节与真实代码为 **3 个业务域**（`internal/novel` / `internal/agent` / `internal/llm`），前端 `frontend/src/domains/` 同样为 `novel` / `agent` / `llm`；
- 而 `docs/system-design/` 体系声明 **7 个业务域**（content / outline / worldview / inspiration / agent / compliance / llm）；
- 该差异会导致按 7 域路径新建包时被 `cmd/archguard` 的跨域依赖校验与组合根装配边界拦截，必须经 ADR 裁决后方可施工；
- 已交付的领域详细设计文档作为「逻辑设计资产」继续有效，其包路径引用以最终 ADR 为准。

**M0 工程基线**

- 建立 Git 仓库与基线提交，新增 `.gitattributes` 统一换行符（INK-001）
- 新增架构护栏脚本 `cmd/archguard`：跨域 infrastructure 导入、platform 反向依赖、单文件 50KB、测试文件、Go 语法五项校验，并可输出运行路径可达性（INK-033）
- 新增 CI 流水线 `.github/workflows/ci.yml`：后端 build / vet / gofmt / archguard，前端 `npm ci` + `tsc -b` + `vite build`（INK-032）
- 新增统一命令：`Makefile` 与 Windows 等价脚本 `scripts/check.ps1`，并提供 `.golangci.yml` 深度检查配置（INK-034）
- 新增容器化资产：多阶段 `Dockerfile`（非 root、镜像注入版本号、HEALTHCHECK）、`.dockerignore`、`docker-compose.prod.yml`（INK-035）
- 拆分两个逼近红线的大文件（INK-030）：`platform/config/config.go` 44.8 KB → `config.go` + `config_defaults.go` + `config_env.go`；`llm/routing/generation_plan.go` 39.5 KB → `generation_plan.go` + `generation_plan_types.go` + `route_profile_eval.go` + `route_profile_hash.go`。拆分后最大文件 32.9 KB，代码行多重集合与原文件完全一致
- 统一时间访问（INK-031）：业务与持久化层 40 处 `time.Now()` 改为 `timeutil.Now()`（UTC 瞬时）；缓存 TTL / 限流窗口 / 延迟测量等技术计时保留 `time.Now()` 以维持单调时钟
- 全库 gofmt 归一（16 个文件，纯格式变更）

**M1 持久化基础**

- 引入 `gorm.io/driver/postgres`，新增 `internal/platform/database`：连接池、Ping 校验、Close、Stats（INK-002）
- 新增迁移执行器：`schema_migrations` 版本表（TIMESTAMPTZ）、幂等 `Up`、校验和变更保护、`Down` 降级段回滚、`Status`；`cmd/migrate` 提供 up/down/status 命令行（INK-003）
- 配置中心接入组合根：`config.Bootstrap` 统一装载入口，生产环境拒绝不完整配置，开发环境可降级启动；新增 `configs/config.example.yaml`，`configs/config.yaml` 加入忽略（INK-006）
- `.env.example` 重写为配置中心真正读取的变量名（`DATABASE_DSN` / `REDIS_*` / `APP_*`）
- 组合根改为返回错误：`app.New() (*App, error)`；数据库连不上即拒绝启动，未配置则开发环境降级并告警
- 新增 `/readyz` 依赖探针；`/v1/architecture` 暴露 `env` 与 `database` 状态
- 迁移 SQL 以 `go:embed` 打包，运行时不依赖工作目录

**M4 前置：工具调用契约（INK-038）**

- `llm/domain.Gateway` 新增 `GenerateChat`：原契约的 `GenerateRequest` 只能承载纯文本消息，无法表达 assistant 的 `tool_calls` 与 tool 角色的 `tool_call_id`，导致 TAO 循环的多轮工具调用在协议层无法成立
- `internal/llm/infrastructure/chat.go`：实现带工具调用的对话补全，出站载荷携带 `tools` / `tool_choice` / `max_tokens`，响应解析 `tool_calls` / `finish_reason` / `usage`；未配置 Provider 时返回 `ErrProviderNotConfigured`，不伪造内容
- `internal/app/adapters/model_execution_gateway.go`：把 llm 域的对话能力适配为 TAO 循环所需的 `ModelExecutionGateway`
- 验证：以本地打桩上游断言 14 项（tools 出站、tool_calls 与 tool_call_id 保留、响应解析、无 Key 明确报错），全部通过

**新发现的 P0 问题（未修，已登记 INK-039）**：`llm/infrastructure/provider_router.go` 的内置引擎（未配置 Provider 时的 `smart_builtin` 路径）会生成模板正文并硬编码一个 `call_fact_check_001` 工具调用，与 README「不会伪造小说内容」的承诺冲突。是否保留该 demo 模式需要产品决策。

**M4：真实 TAO 循环接管 Agent 运行时（INK-004）**

- `internal/agent/runtime/application/service.go` 重写为驱动 `loop` 真实 TAO 状态机，删除全部模拟轨迹与硬编码工具结论
- 新增 `internal/agent/runtime/application/toolkit.go`：把 Agent 定义中的工具 ID 解析为循环可调用的工具项；声明了未注册工具时返回 `ErrToolNotRegistered`，而不是静默忽略或伪造调用
- 显式登记创作工具的能力与审批策略：未注册的工具会被 `CheckToolPermission` 直接拒绝，表现为「模型调用了工具但什么都没发生」
- Observation 内容取自事件载荷 `ToolResultData.Result` 的真实输出，不再使用「工具 X 执行成功」这类无信息量状态文案
- 未配置 Provider 时经 `normalizeGatewayError` 返回 `503 + llm_provider_not_configured`，给出可自愈的明确提示
- 验证：13 项端到端断言（以工具直接执行结果为基准真值比对 Observation、两轮循环、轨迹无硬编码文案、缺失网关与未注册工具均明确报错）全部通过；HTTP 实测返回 503 `llm_provider_not_configured`
- 运行路径可达包由 32 提升到 **52**（`loop` / `tooling` / `promptctx` / `approval` 等进入运行路径），不可达由 28 降到 **11**

**M4：消除内置引擎的伪造输出（INK-039）**

- 新增 `LLM_BUILTIN_FALLBACK` 开关（默认关闭）。未配置 Provider 时 `Generate` / `GenerateStream` / `GenerateEventsStream` / `GenerateChat` 一律返回 `ErrProviderNotConfigured`，不再输出模板正文
- 删除内置引擎中伪造的 `call_fact_check_001` 工具调用与 `success` 回执：伪造的执行轨迹比伪造正文更具误导性
- `ProviderInfo` 如实上报：默认 `type=none`；开启演示后名称/类型/能力均标明离线演示与 `template_based`
- `GatewayService` 与 LLM HTTP 层统一错误映射：`503 + llm_provider_not_configured`，不再被二次包装成通用 `llm_generation_failed`
- 新增 [ADR-0005](docs/decisions/0005-no-fabrication-without-provider.md) 记录该决策
- 验证：探针 14 项断言（默认模式零产出、演示模式 tool 事件恒为 0）全部通过；`POST /v1/llm/generate` 默认返回 503 `llm_provider_not_configured`；`/v1/llm/provider/status` 默认 `type=none`

**M6：前端 i18n 完整性校验（INK-042）**

- 新增 `cmd/i18ncheck`：单遍扫描 TS 对象字面量，支持点分平面 key 与嵌套对象两种写法，返回缺失/多余 key 明细
  · 正确处理字符串、双/单引号、行注释与块注释，避免把文案或注释里的 `"a.b":` 误判为词条
  · 解析结果为 0 条时直接报错，杜绝"解析器不兼容却静默通过"
- 接入 CI 与统一命令：`make lint` / `make ci` / `scripts/check.ps1` 均包含该校验
- 验证：真实 locale 272/272 一致；两类 fixture（嵌套无引号、点分带引号+注释干扰）均正确报出 `缺失 common.hint / 多余 novel.subtitle` 并以 exit 1 结束

**M0：构建信息可观测**

- `platform/version` 接入组合根：`/healthz` 与 `/v1/architecture` 返回 version / git_commit / build_time / go_version，运维无需登录即可确认线上版本

**发现的硬前置（已登记 INK-043）**

- `000001_init.sql` 只有 `llm_providers / llm_models / llm_channels / llm_usage_records`，**没有渠道密钥表，也没有渠道-模型映射表**
- 而 `routing.Pool` 的选路依赖 `ChannelKey`（日限额 / RPM / 并发上限）与 `ChannelModelMapping`（权重 / 优先级 / 上游模型别名）
- 因此 INK-005 必须先在迁移中补齐这两张表，否则"接线 routing"无从谈起

**M3：多 Key 选路 / 加权负载 / 限流 / 熔断生效（INK-005）**

- 新增 `internal/llm/infrastructure/env_channel_registry.go`：由 `LLM_CHANNELS` 环境变量装配渠道注册表，
  同时实现 `routing.ChannelProvider` 与 `routing.ModelProvider`
  · `keys[].name` 是**持有密钥的环境变量名**，只有发起上游请求的瞬间才读取真实密钥，
    JSON 中不含任何明文（ADR-0004）
  · 支持每渠道多 Key、权重、优先级、日限额、RPM 上限、并发上限与区域
- `ProviderRouter` 接入 `routing.Pool`：候选集一次获取、优先级分组、平滑加权轮询、
  密钥并发槽 / RPM 闸门、密钥级熔断；未配置 `LLM_CHANNELS` 时退化为原有单一渠道直连
- 新增 `upstream_target.go`：统一解析本次调用的渠道与密钥，并在调用结束后回报结果
  （驱动熔断与密钥统计），覆盖非流式、流式与工具调用三条上游路径

**顺带修复的两个真实缺陷**

1. `gateway.ClassifyExecutionError` 是恒返回 `unknown` 的桩函数，而密钥级熔断只对
   `provider_unavailable` / `transport_error` 计数 —— 也就是说 AGENTS.md 承诺的
   "故障自动熔断降级"**永远不会触发**。现已实现真实分类（401/403 鉴权、429 限流、
   5xx 可重试、4xx 请求错误、超时/网络错误可重试），并让上游非 200 响应携带
   `ProviderHTTPStatus()`，使分类与熔断得以成立。
2. `MemoryKeyRateLimiter` 在令牌不足时**无限期阻塞等待**，而调用方 `routing.Pool`
   期待的是"是否允许"并据此产出 `RateLimitError`。现改为排队超过 `500ms`
   立即拒绝，消除静默排队。

**验证（本地打桩上游，8 项断言全部通过）**

- 两渠道 1:1，100 次调用分布均衡（各 35%~65%）
- A 渠道持续 5xx：第 5 次连续失败触发熔断（日志 `new_state=open`），
  后 10 次调用全部切到 B 成功
- 单 Key `rate_limit=3`：第 4 次起返回明确的 `RateLimitError`
- 密钥环境变量缺失：报错点名缺失的变量名，可自愈
- 未配置 `LLM_CHANNELS`：构造成功，退化为直连

**M2：认证中间件与租户上下文（INK-007）**

- 新增 `internal/platform/auth/token.go`：平台自签 HS256 令牌（签发 + 校验）
  · 显式拒绝 `alg=none` 算法降级；`hmac.Equal` 常数时间比较；校验 `exp`
  · 手写约 120 行，不引入外部 JWT 依赖
- 新增 `internal/platform/middleware/auth.go`：Bearer 令牌校验 →
  主体写入 gin 上下文 → 租户绑定进请求上下文
  · `AUTH_MODE` 默认随环境：生产 `required`、其余 `disabled`（携带令牌仍解析）
  · `required` 模式下缺 `tid` 直接 401，杜绝"认证了但没隔离"
  · `required` 但未配置密钥时**进程拒绝启动**
- 新增 `cmd/tokengen`：本地签发令牌（密钥只从环境读取）
- 新增 `GET /v1/auth/whoami`：回显主体、租户与请求上下文中的租户
- 新增 [ADR-0006](docs/decisions/0006-platform-token-and-auth-mode.md) 记录认证契约

**验证（11 项断言全部通过）**：无 Token / 伪造 / 缺 `tid` / 过期 → 401 且 code 明确；
有效 Token → 200；`whoami` 租户绑定正确；`/healthz`、`/readyz`、`/v1/architecture`
免认证可用；开发环境默认放行，不打断前端联调。

**新发现（已登记 INK-044）**：`novels / novel_chapters / novel_characters / fact_ledger`
**没有任何 tenant 列**，因此"租户 A 读不到租户 B 数据"无法在数据层达成 ——
认证部分已闭环，隔离部分拆为 INK-044（依赖 INK-003 建表）。

**M1：真实数据库闭环（Docker 就绪后完成）**

Docker 守护进程可用后，此前三条阻塞的验收全部在真实 PostgreSQL 16 + pgvector 上完成：

- **迁移执行器（INK-003）**：`migrate status/up` 建表 11 张（10 业务 + `schema_migrations`）、
  重复执行幂等（`applied=0 skipped=1`）、`status` 输出应用时间；
  无降级段的迁移执行 `down` 时**明确拒绝**而不是破坏数据
- **PostgreSQL 连接装配（INK-002）**：新增启动期自动迁移（`MIGRATE_ON_START`，
  开发默认开启、生产默认关闭以避免多实例抢迁移）。清空 schema 后冷启动 → 11 张表建立
  （000001 的 10 张 + schema_migrations；补上 000002 后当前共 13 张）、
  `/readyz` 返回 `database=ok`；再次启动 `applied=0 skipped=2`
- **生产镜像（INK-035）**：`docker build` 产出 **55.6MB** 镜像；容器以非 root(10001) 运行，
  通过 compose 网络连库成功，`/healthz` 返回构建信息、`/readyz` 返回 `database=ok`
- **渠道密钥与映射表（INK-043）**：新增 000002 迁移 —— `llm_channel_keys`
  （只存 `secret_ref`，**明文密钥列数量实测为 0**，符合 ADR-0004）与 `llm_channel_models`
  （权重/优先级/上游别名/能力状态）；同时为 4 张作品域表补 `tenant_id`（INK-044 的 schema 前置）

**M1：Novel 域 SQL 仓储与租户隔离（INK-010 / INK-044）**

- 新增 `internal/novel/infrastructure/sql_repository.go`：PostgreSQL 实现的 4 个仓储
  （作品 / 章节 / 角色 / 事实），语义与内存实现对齐（NotFound 错误、列表排序、UpdatedAt 维护）
- **租户作用域**：每条查询都带 `tenant_id` 过滤，租户从请求上下文读取；
  AUTH_MODE=required 时由认证中间件保证必有租户，开发环境缺失时退化为 `default`
- **JSONB 显式转换**：`profile` / `fact_value` 写入用 `?::jsonb`、读取用 `->>` 提取。
  踩到一个真实陷阱并修正：`novel_characters` 并没有 `description/background` 列，
  档案整体在 `profile` 里，直接 `SELECT *` 会让字段静默读空
- 新增 000003 迁移补 `fact_ledger.updated_at`（000001 遗漏，不为它改写已应用的迁移）
- 组合根按可用性选择仓储：有数据库用 SQL，无则退化为内存并告警

**验证**：仓储层 18 项断言 + HTTP 端到端 7 项断言全部通过

- 重启留存：租户 A 创建作品 → **停止进程 → 重新启动** → 列表仍有该作品
- 租户隔离：租户 B 列表为空、按 ID 直查返回 404、更新被拒
- 字段往返：章节 outline/ordinal/word_count、角色 JSONB 档案、事实 JSONB + 外键
- 级联删除：删除作品后其章节随外键级联清空

**M1：Agent 运行记录与事件流持久化（INK-011）**

- 新增应用层 Port `ports.RunRecorder`（StartRun / FinishRun）：运行审计属于智能体域自身职责，
  具体落库方式由组合根注入，符合依赖倒置约束
- 新增 `agentrun/recorder.go`：写入 `agent_runs`（租户、字符串 Agent 标识、输入/输出快照），
  事件流以 `trace_events` JSONB 落库，`updated_at`/`error_text` 记录收尾信息
- 运行中注入记录器：成功记 `succeeded`、失败记 `failed` 并保留错误文本；
  **审计写入失败只告警不阻断** —— 不因为审计问题把一次成功生成变成失败
- 新增 000004 迁移：`agent_runs` 补 `tenant_id` / `agent_key` / `session_id` / `user_id` /
  `error_text` / `updated_at` 与租户索引

**验证（14 项断言全部通过）**：运行记录存在且带租户与 Agent 标识、事件流 JSONB 含工具调用与观察、
输入/输出快照正确、租户 B 查不到租户 A 的记录、**跨租户回填被拒绝**、
失败运行记为 failed 且带错误文本、未注入记录器时运行不受影响。

**新发现（已登记 INK-045）**：移植的智能体持久化代码（`agentrun/repository.go`、`artifactstore`、
`approval`、`mcpclient` 等）引用 **12 张表，而本项目 schema 只有其中 2 张** ——
`agent_run_nodes`、`artifact_refs`、`agent_artifacts`、`agent_tool_approvals`、
`agent_mcp_servers`、`agent_mcp_tool_cache`、`agent_activity_logs`、
`role_agent_access`、`role_model_access`、`user_agent_preferences` 均不存在。
因此本次只交付了"运行主记录 + 事件流"这一层（正是 INK-011 的验收范围），
制品外置/审批/MCP 部分需要 INK-045 先建表。

**M6：WebSocket 广播总线（INK-021）**

补齐 AGENTS.md 第 3 节第 3 条要求的 "SSE + WS 双持" —— 此前 WS 一侧完全缺失。

- 新增 `internal/platform/realtime`：按主题的进程内广播中心
  · `Publish` 为**尽力而为**语义：无订阅者静默丢弃、订阅者缓冲满则跳过该订阅者，
    发布方（业务请求）绝不会被慢客户端拖住
  · 订阅者缓冲仅 32 条：广播的意义在"最近状态"，不在补发历史
- 新增 `/v1/ws` 端点（`github.com/coder/websocket` v1.8.15）
  · **主题由服务端按租户上下文决定**，客户端无法指定任意主题，因此不存在跨租户窃听
  · 连接后下发 `connected` 问候；30s 心跳保活，ping 失败即回收连接
  · 该路由单独允许 `?token=`（浏览器 WebSocket API 无法自定义请求头），
    只对这一条路由开启；日志层已对 token 参数脱敏
- 新增应用层 Port `ports.EventBroadcaster` + 适配器：Agent 运行终态自动广播
  `agent_run_finished`（含 run_id / status / steps），与 INK-011 的运行记录同一收尾点
- `/v1/architecture` 暴露 `ws_topics` 与 `ws_subscribers`

**验证（11 项断言全部通过，真实服务 + 真实 WS 客户端）**：无令牌连接被拒；
带令牌经 `?token=` 连接成功并收到问候（主题标明 tenant-a）；
触发 Agent 运行后收到 `agent_run_finished` 且载荷带 status=failed 与 run_id；
**租户 B 在 3 秒内未收到租户 A 的任何广播**（主题隔离成立）。

**M5：章节确认制与事实版本化（INK-016）**

平台核心承诺「长篇不崩设定」的第一步：让事实账本真正可校验、可回溯。

- 新增 `internal/novel/domain/consistency.go`：纯函数 `EvaluateFactProposals`
  用现有**活跃**事实评估一批提议，判定规则是「同类别 + 同 key 但取值不同」——
  这正是长篇创作里最常见的设定漂移
- 新增 `internal/novel/application/confirm.go`：章节确认流程
  · 校验章节归属作品；跨作品/跨租户一律按未找到处理，不泄漏存在性
  · 冲突时**整批拒绝、零写入**，账本不会进入半吊子状态
  · 无冲突时写入新事实并记录 `source_chapter_id`，章节状态置 `confirmed`
  · `allow_override=true` 时才允许覆盖，且旧值只置 `is_active=false` 而不删除
- 新增 `POST /v1/novels/:novelID/chapters/:chapterID/confirm`
  · 冲突返回 **409 + 结构化冲突清单**（含旧值、提议值、来源章节、原因）
  · 冲突经由 `ConfirmResult` 传递而非 error 通道 —— error 无法承载结构化清单
- 新增 [ADR-0007](docs/decisions/0007-chapter-confirmation-and-fact-versioning.md)

**验证（18 项断言全部通过，真实 PostgreSQL）**：首次确认入账且可回溯来源章节、
章节状态转 confirmed；同值重复确认识别为 unchanged 且不重复写入；
冲突被拦下且**零写入**（账本仍只有 1 条活跃事实）；
授权覆盖后账本呈现「1 条活跃 + 1 条历史」；跨租户 404；空 key 400。

**已知未验证**：本机无 PostgreSQL 且 Docker 守护进程需管理员权限，建表与重启留存未实测；详见 `docs/backlog.md`「待验证事项」。

**M3：模型调用用量审计落库（INK-018）**

- 新增 `internal/llm/domain/usage.go`：`UsageRecord` / `UsageRecorder` 端口；记录刻意**不含** Prompt 明文与密钥
- 新增 `internal/llm/infrastructure/persistence/usage_recorder.go`：写入 `llm_usage_records`（租户、请求 ID、模型、渠道、输入/输出 token、延迟、状态、失败原因），失败只告警不阻断
- `ProviderRouter` 三条上游调用路径（非流式 / 流式 / 工具调用）统一在 `defer` 中调用 `recordUsage`；流式路径旁路捕获终态事件的 usage，不改变原有事件语义
- 组合根在 `application.db != nil` 时通过 `SetUsageRecorder` 注入；无数据库环境保持可用
- 新增迁移 `000005_llm_usage_metadata.sql`（补 `tenant_id` / `error_text` 与两个索引）与 `000006_llm_usage_channel_nullable.sql`

**验证（真实 PostgreSQL + 本地 OpenAI 兼容桩）**：`POST /v1/llm/generate` 两次 —— 成功行 `ink018-ok-002` 记 11/22 token、`status=succeeded`、`channel_id` 为空（直连）；上游 500 的失败行 `ink018-fail-002` 记 `status=failed` + 上游错误文本。
探针同时暴露一个真实缺陷：`channel_id` 带外键约束，直连部署写 `0` 会 100% 写入失败（首轮实测 `llm_usage_records` 0 行），经迁移 000006 修复后写入正常。

**已知边界**：流式路径的 token 仅在提供方返回终态 usage 事件时才有值，否则记 0；延迟与成败仍然准确。

**M1：制品库存储选型定案（INK-020）**

- 新增 [ADR-0008](docs/decisions/0008-artifact-store-postgres-jsonb.md)：Agent 制品库统一落 PostgreSQL JSONB，**不引入 MongoDB**
- 依据：两份编排文件本就只启动 postgres + redis，`MongoStore` 从未接线（不可达包）；项目红线是模块化单体，第二套数据平面会额外带来备份、监控与租户隔离成本（现有租户隔离建立在每条 SQL 带 `tenant_id` 之上）
- `docs/architecture.md` §6.2 存储职责表据此改写：MongoDB 行标注为不引入，原先的 ⚠️「待 ADR 决策」说明撤除
- 登记 INK-046（删除 `artifactstore` 的 Mongo 实现）与 INK-045（制品表随统一建表迁移落地）

**验证**：`grep -rn "mongo" docker-compose.dev.yml docker-compose.prod.yml` 无匹配。

**M7：就绪探针区分「依赖在线」与「依赖已配置」（INK-036）**

- 新增 `internal/platform/cache/readiness.go`：`RedisHealthChecker` 惰性 Redis 客户端（构造不连接、不返回错误），单次探测 2s 超时
- `/readyz` 改为探测**已配置**的依赖：数据库与 Redis 均在线 → 200 `{database: ok, redis: ok}`；任一已配置依赖不可用 → 503 `not_ready` + `unavailable` 列表；未配置的依赖 → `disabled` 且不影响就绪（开发环境降级不误报）
- 错误文本经 `logger.SanitizeURL` 脱敏后才进入响应体
- 惰性构造的意义：依赖抖动不会让进程起不来，依赖恢复后探针自动转回就绪

**验证（真实 PostgreSQL + Redis，三态实测）**：双依赖在线 `/readyz` 200 且 `/healthz` 200；`docker stop inklore-redis-1` 后 `/readyz` **503 `not_ready`**（`redis: unreachable`、`unavailable: [redis]`）而 **`/healthz` 仍 200**；`docker start` 后未重启进程即恢复 200。


**M1：删除 artifactstore 的 Mongo 实现（INK-046）**

- 删除 `internal/agent/infrastructure/artifactstore/mongo_store.go`（172 行，含 `MongoStore` / `MongoOptions` / `ensureIndexes`）
- 去 Mongo 化改名：`BackendMongo = "mongo"` → `BackendPostgres = "postgres"`；`MongoID` → `StorageObjectID`；列名与 JSON 键 `mongo_id` → `storage_object_id`（覆盖 `artifact_refs`、`agent_artifacts` 及 `agentrun/cleanup.go` 的 LIKE 匹配）
- 移除 `Document` / `BinaryRef` 的全部 25 处 `bson` 结构标签 —— 不再假设存在文档数据库映射
- `go mod tidy` 后 InkLore 代码**零处** `go.mongodb.org` 引用；`sashabaranov/go-openai`、`klauspost/compress`、`xdg-go/*`、`youmark/pkcs8` 等仅服务该驱动的传递依赖被移除，`go.sum` 减少 36 行。残留的 `go.mongodb.org/mongo-driver/v2` 是 `gin → gin/binding → bson` 的间接需求（`go mod why -m` 可复现），非本项目代码
- 登记 **INK-047**（实现 `artifactstore.Store` 的 PostgreSQL 版本，ADR-0008 的落地）与 **INK-048**（清理 `MongoConfig` / `MONGODB_*` 死配置面）

**验证**：`go build ./...`、`scripts/check.ps1`（gofmt/build/vet/i18n/archguard）与 archguard R1–R5 全部通过；`grep -rn "go.mongodb.org" internal cmd` 为 0；`grep -rn "bson:" internal` 为 0。

**M1：智能体持久化 schema（INK-045）**

- 新增迁移 `migrations/000007_agent_persistence_tables.sql`：10 张表（`agent_run_nodes`、`artifact_refs`、`agent_artifacts`、`agent_activity_logs`、`agent_tool_approvals`、`agent_mcp_servers`、`agent_mcp_tool_cache`、`user_agent_preferences`、`role_agent_access`、`role_model_access`），并补齐 `agent_runs` 的 `input_json`/`output_json`/`error`/`started_at`/`finished_at`/`execution_key` 六列
- `agent_runs` 三处约束调整：`request_id` 补默认值、`tenant_id` 放开 NOT NULL、移除 `agent_id` 外键（`CreateRun` 传 0 必然违约）
- 两个 `ON CONFLICT` 目标索引按代码语义建为**部分唯一索引** / `NULLS NOT DISTINCT`：`artifact_refs(idempotency_key) WHERE idempotency_key <> ''`；`agent_mcp_tool_cache(user_id, server_name, server_url, auth_fingerprint) NULLS NOT DISTINCT`（PG15+）——后者顺带修掉匿名缓存永不收敛的问题
- 列类型按「代码实际绑定」确定：JSON 语义列落 `TEXT`（只有 `recorder.go` 显式写 `?::jsonb`），id 一律 `BIGSERIAL`，时间列一律 `TIMESTAMPTZ`

**探针发现并修复的 2 个真实缺陷**

1. `agent_runs.input_snapshot` 为 NOT NULL 且无默认值，而 `CreateRun` 走 `input_json` 不写该列 ⇒ 该路径 100% 违反 not-null（SQLSTATE 23502）。修复：迁移 `000008_agent_runs_snapshot_default.sql` 补默认值 `'{}'::jsonb`。
2. `AgentMCPToolCacheRepository.GetFresh` 的 `(user_id IS NULL AND ? IS NULL)` 中未定型的 NULL 参数让 pgx 无法推断参数类型，查询报 `could not determine data type of parameter $2`（SQLSTATE 42P18）。修复：改为 `?::bigint` 显式转型。

**验证（真实 PostgreSQL，探针 0 失败，程序已删除）**：10/10 表列集合与代码读写列完全一致；`CreateRun → BindExecutionKey → GetRunByExecutionKey → CreateNode → MarkNodeWaiting → ResumeWaitingNode → ListNodes → FinishRun` 全链路通过；`artifact_refs` 幂等键 `ON CONFLICT` 重放收敛到同一 ref 且 `attempt_count` 递增 1；MCP 工具缓存匿名 `NULLS NOT DISTINCT` 收敛为 1 行；`role_agent_access` / `role_model_access` / `user_agent_preferences` 的 `ON CONFLICT` 通过，且不存在的 agent 被外键正确拒绝（返回「智能体不存在」）。

**已知边界**：10 张新表没有 `tenant_id`（移植代码不提供该列），目前只有 `user_id`/`owner_user_id` 级隔离，已登记 INK-049；`artifact_refs`/`agent_artifacts` 为元数据表，制品正文的 JSONB 存储随 INK-047 落地。

**M1：Provider / 模型 / 渠道注册表落库（INK-019）**

- 新增 `internal/llm/infrastructure/persistence/provider_registry_repository.go`：`SQLProviderRegistryRepository` 实现 `domain.ProviderRegistryRepository` 全部 18 个方法
  · 语义与内存实现对齐：唯一键冲突映射 `ErrDuplicateCode` / `ErrDuplicateModel`，未命中映射三个 `Err*NotFound`，删除 Provider 在一个事务内级联删除其模型与渠道
  · `capabilities` 为 JSONB 列，写入用 `?::jsonb`、读取用 `capabilities::text` 后显式反序列化 —— 不依赖驱动的隐式转换
- 组合根在 `application.db != nil` 时装配 SQL 实现（无库开发档位仍用内存种子实现，启动日志区分）
- 新增迁移 `000009_provider_registry_seed.sql`：幂等写入 4 Provider / 5 模型 / 4 渠道（沿用内存实现的种子内容），保证切换后管理端初始视图不变


**里程碑状态更新（M1 已验收；M7 缺口登记）**

- **M1 · 持久化落地 → ✅ 已验收**。四条验收标准逐条实测：① 冷启动自动建表（多轮「已应用/跳过」）；② 作品域 SQL 仓储跨重启留存（INK-010 留档）；③ `agent_runs` 完整 JSONB 事件流（INK-011/053 探针）；④ **断库降级**：无 `DATABASE_DSN` 启动时 `/healthz=200`、`/readyz=200` 且 `database=disabled`，日志给出「开发环境降级为内存仓储 / 进程重启将丢失数据」两条 WARN，进程无 panic
- **M7 · 可观测性与交付** 补齐缺口登记：**INK-056**（指标端点与模型/渠道维度，对应验收 2）、**INK-057**（发布与回滚流程留档，对应验收 3）；验收 1 已由 INK-036 实测

**M3：悬空包治理清零（INK-037）**

治理标准：**有真实消费方就接线，没有就删除**。实测 `go run ./cmd/archguard -reachability`：**不可达 0**（61 个非 cmd 包全部在运行路径上）。

**接线（1 个）**

- `agent/infrastructure/persistence/agentactivitylog`：新增 `ports.ActivityLogStore` 端口与 `adapters.ActivityLogRecorder`，运行终态写入一句话摘要 + 逐步轨迹（`agent_activity_logs`，含 `tenant_id`），并新增 `GET /v1/agents/runs/:runID/activity`。写失败只告警，绝不影响运行结果

**删除（6 个）**

- `agent/domain/accesscontrol`：与 `approvalcontrol.Principal` 重复的平行主体模型且缺少权限字段
- `agent/infrastructure/persistence/agentaccess`：角色-智能体/模型访问的权威来源（认证中心）不存在，保留只能靠手写 SQL 维护的空表仓储没有意义；INK-054 随之关闭
- `agent/infrastructure/persistence/agentartifact`：被 INK-047 的制品管线取代（`artifact_refs` 管引用、`artifact_documents` 存正文）
- `agent/infrastructure/persistence/agentevent`：事件流已由 `agent_runs.trace_events`（JSONB）承载，`RunRecorder` 是唯一写入方
- `platform/mathutil`：全仓库无调用方，Go 1.21+ 内置 `min`/`max` 已覆盖其能力
- `platform/crypto`：INK-049 删除重复的 MCP 服务后失去唯一消费方；ADR-0004 的密钥来源是环境变量/Secret Store

**验证**：`archguard -reachability` 由 `67 ｜ 61 ｜ 不可达 6` 变为 `61 ｜ 61 ｜ 不可达 0`；活动日志端到端实测 —— 运行后 `agent_activity_logs` 新增一行（`run_id=38`、`tenant_id=default`、`summary=运行 succeeded：2 步`、content 含轨迹首行），`GET /v1/agents/runs/38/activity` 返回 1 条。`scripts/check.ps1` 与 archguard R1–R5 全绿。

**保留但无人使用的 schema**：`agent_run_nodes`、`agent_artifacts`、`user_agent_preferences`、`role_agent_access`、`role_model_access` —— 迁移不可改写，随仓储删除成为未使用表，已在架构文档登记，待真实需求出现时连同仓储一起引入。

**新登记**：**INK-055** 记录 M3 验收 1–3（加权分布 / 熔断切换 / RPM 限流错误）缺实测证据。

**M1：`artifact_refs` 的租户化收口（INK-053）**

- 新增迁移 `000013_artifact_refs_tenant_idempotency.sql`：把部分唯一索引由 `(idempotency_key) WHERE idempotency_key <> ''` 重建为 `(tenant_id, idempotency_key) WHERE idempotency_key <> ''`，与 `Begin` 的 `ON CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key <> ''` 逐字对应 —— 否则 PostgreSQL 直接报 no unique or exclusion constraint
- `ArtifactRefRepository` 全部 9 个方法落地作用域：`Begin`/`Create` 写入 `tenant_id`，`MarkReady`/`MarkStatus`/`AttachOwners`/`Get`/`ListByAgentRunID`/`ListByWorkflowID`/`ListReconciliationCandidates` 一律带 `tenant_id` 条件；原先 4 个无 `ctx` 的遗留方法统一补 `ctx`，`artifactRefSagaStore` 端口签名同步

**验证（真实 PostgreSQL，15 项 0 失败，探针已删除）**

- **跨租户幂等空间已隔离**：同一 `idempotency_key` 在两个租户各产生一行（id=20 / id=21），而不再是复用同一行
- 同租户重放仍收敛到同一行且 `attempt_count` 递增
- 跨租户 `Get` 返回空、`MarkStatus`/`AttachOwners` 返回错误、`ListByAgentRunID` 为空、`ListReconciliationCandidates` 不含对方条目
- 遗留 `Create` 路径同样记录调用方租户

**M1：智能体表的租户作用域（INK-049）**

- 新增迁移 `000012_agent_tenant_scope.sql`：11 张表（000007 的 10 张 + `artifact_documents`）补 `tenant_id TEXT NOT NULL DEFAULT 'default'` 与 `(tenant_id, …)` 索引；MCP 工具缓存的唯一键改为 `(tenant_id, user_id, server_name, server_url, auth_fingerprint)` —— 否则两个租户的同名 Server 会互相覆盖缓存
- 采用 `NOT NULL DEFAULT 'default'` 而不是可空列：未改造完成的写入路径落入**显式默认租户**，既不会写空值也不会因约束失败
- `platform/tenantcontext` 新增 `OrDefault(ctx)`；作品域、agent 持久化与制品库共用同一「缺租户归默认租户」语义，`artifactstore` 因此不必反向依赖 agent 持久化（避开 import cycle）
- **已装配的四条路径完整落地作用域**：工具审批（含 `List`/`GetByID`/`GetByTokenHash`/`Consume`/`RotateToken`/`Reject`/`DeleteExpired`）、MCP Server（增删改查与工具清单回写，方法统一补 `ctx`）、MCP 工具缓存、制品正文（Put/Get/Delete/去重查询）
- 顺带删除 `runtime/application/tooling/mcp_service.go`：它是已被 `enginemcp` 取代、全仓库无人引用的重复 MCP 服务，且其鉴权头走自加密而非 ADR-0004 的环境变量引用模型

**验证（真实 PostgreSQL，22 项 0 失败，探针已删除）**

- 审批单：两租户同 `user_id` 各建一单，行内 `tenant_id` 各自正确；tenant-a 列表只看到自己；tenant-b 按 ID 查不到、`Reject` 影响 0 行
- MCP Server：tenant-b 列表为空、按 ID 查不到、`Delete` 影响 0 行且 tenant-a 的行仍在
- MCP 工具缓存：tenant-b `GetFresh` 取不到 tenant-a 的缓存，tenant-a 自己能命中
- 制品正文：tenant-b 取不到也删不掉 tenant-a 的制品（返回未找到），tenant-a 自己能取到
- 无租户上下文写入**成功**且落到 `default`（不会因 NOT NULL 失败）

**已知缺口**：`artifact_refs` 的幂等键仍是全局唯一（跨租户共享幂等空间），其 4 个无 ctx 的遗留方法未作用域化，3 张未装配表的主键仍为 `user_id`/`role_id` —— 已登记 **INK-053** 收口。

**M5：pgvector 语义召回接入（INK-017）**

- LLM 域新增可选嵌入能力：`domain.EmbeddingGateway` + `ProviderRouter.Embeddings`（POST `{baseURL}/embeddings`，与生成路径共用渠道解析、密钥来源与上游回报；未配置 Provider 时明确报错，绝不返回伪造向量），`GatewayService.Embeddings` 在网关不支持时返回 503 `llm_embeddings_unavailable`
- 新增迁移 `000011_novel_embeddings.sql`：单表承载章节与人物语义片段（`embedding vector(1536)` + HNSW 余弦索引 + `(tenant_id, novel_id, kind, ref_id)` 唯一约束），避免给每张业务表加 `embedding` 列
- 作品域新增 `SemanticIndexRepository` 端口与 SQL 实现、`ports.EmbeddingProvider` 端口与组合根适配器（模型名取 `EMBEDDING_MODEL`，默认 `text-embedding-3-small`）、`SemanticService`（索引构建 + 召回）
- 新增端点 `POST /v1/novels/:novelID/embeddings/index` 与 `POST .../embeddings/recall`
- **生成路径接入**：上下文包用「本章标题 + 目标」作查询召回 Top-3，作为【相关历史片段】小节注入 Prompt；索引未装配或召回失败时该小节自动省略，只告警不阻断

**验证（真实 PostgreSQL + pgvector 0.8.6 + 确定性嵌入桩，探针已删除）**

- 索引：3 章 + 2 人物 → `indexed=5 total=5`；另一部作品 → `indexed=1`
- 召回：查询 `断剑铭文` 的 Top-1 为「第二章 断剑」（相似度 0.8165），并按其后的相似度降序返回
- **作用域隔离**：召回结果不含另一部作品中的 `别书 断剑`（`novel_id` + `tenant_id` 作用域）
- **生成路径**：`POST .../generate-chapter` 返回 200，被捕获的真实模型请求体包含【相关历史片段】、命中片段标题与相似度

**已知缺口**：M5 验收标准 4 要求「Top-3 命中率在抽样集上可用（人工评估记录留档）」。本次用确定性嵌入桩验证的是**链路、排序与作用域**，不能等同于真实嵌入模型下的命中率结论；已登记 **INK-051**，M5 因此保持「进行中」。

**M4：上下文压缩绑定辅助模型（INK-050）**

- 新增配置段 `agent_auxiliary.compression`：`enabled` / `model_key` / `system_prompt` / `user_prompt` / `max_tokens`（默认 1024）/ `temperature`（默认 0.2），默认提示词模板带 `{{.Conversation}}` / `{{.Instruction}}` 占位符，由 `promptctx.RenderAuxiliaryPrompt` 渲染
- 新增 `adapters.ResolveCompressionBinding`：模型只按 `model_key` 声明，**到 Provider 注册表（`llm_models` + `llm_providers`）解析**出模型 ID 与供应商，再注入 `RunRequest.Compaction` —— 保证「配置里写的模型一定是管理端真实存在且启用的模型」
- 组合根在启动时解析并打印绑定结果；未启用 / 模型不存在 / 模型已停用 / 注册表不可用都只是**告警并降级为本地确定性交接文档**，绝不阻断运行

**验证（真实 PostgreSQL 注册表 + 真实模型网关 + 模型桩，探针 12 项 0 失败，探针已删除）**

- 配置装载：`agent_auxiliary.compression` 小节解析成功，`max_tokens`/`temperature` 走默认值，模板含占位符
- 绑定解析：按 `model_key=gpt-4o` 解析出 `id=1 / model=gpt-4o / provider=openai`，任务带辅助模型 ID 与预算
- **模型可用时交接文档由模型生成**：桩返回的可辨识文本出现在 `GenerateHandoffDocument` 的结果中
- 未启用 → 不产生绑定且不报错；未绑定 → 模型侧明确失败（`上下文压缩未配置或已禁用`），但 `OptimizeMessages` 仍生成本地交接文档（**不阻断**）
- 模型不在注册表 → 明确报错 `辅助模型 "no-such-model" 不在 Provider 注册表中`

**里程碑状态**：**M4 · Agent 运行时生产化 → ✅ 已验收**。四条验收标准均有实测证据（真实工具返回值 / Provider 未配置明确报错 / 审批中止并记录原因 / `loop_guard` 拦截），五项交付物全部落地。

**M5：章节生成上下文包（INK-015）**

- 新增 `internal/novel/domain/context_pack.go`：`ContextPack` 把作品聚合收敛为五段固定小节（世界观 / 角色最新状态 / 时间线 / 前章摘要 / 未回收伏笔），`Render()` 输出带 `【小节】` 标题的 Prompt 片段；空小节也输出标题与「（无）」，使「Prompt 里带了哪些上下文」始终可判定
- 事实只取**活跃**条目：`IsActive=false` 的伏笔视为已回收，不再喂给模型；角色状态与时间线按类别分流，输出顺序稳定（利于 Prompt 前缀缓存）
- 新增 `Service.BuildChapterContextPack(ctx, novelID, chapterID)`；`chapterID` 为空表示「生成下一章」（前章取序号最大者），指定时取严格在其前的一章
- 新增端点 `GET /v1/novels/:novelID/chapters/:chapterID/context-pack`，同时返回结构化字段与渲染后的 Prompt
- **生成路径接入**：`generate-chapter` 与流式生成先构建上下文包，并以**服务端权威上下文覆盖请求体**（客户端不再自行拼世界观与角色）；构建失败只告警、不阻断生成

**验证（真实 PostgreSQL + 捕获真实模型请求体的桩，探针已删除）**

- 上下文包端点：五段标题齐全；含服务端世界观、两名角色、时间线条目、前章摘要与活跃伏笔；**已回收伏笔（`旧钥匙`）被排除**；`chapter_index=2` 正确
- 生成路径：`POST .../generate-chapter` 返回 200；**被捕获的真实模型请求体**同样包含五段标题、前章摘要与活跃伏笔；请求体里注入的「客户端伪造的世界观」**未出现在 Prompt 中**，服务端世界观生效

**M1：制品正文落 PostgreSQL 并接入运行路径（INK-047）**

- 新增 `internal/agent/infrastructure/artifactstore/postgres_store.go`：`PostgresStore` 实现 `Store` 端口（Put/Get/Delete/Health/Close），语义与 `MemoryStore` 对齐（同 id 不同 `content_hash` 视为幂等冲突，未命中返回 `ErrNotFound`）
- 新增迁移 `000010_artifact_documents.sql`：`payload JSONB` + `payload_json` / `text_content` / `metadata JSONB` + `content_hash` 与 `artifact_refs.storage_object_id` 对应；`created_at`/`updated_at` 为 TIMESTAMPTZ
- **体积策略**：单条超过实现层上限（默认 4MB，可用 `PostgresStoreOptions.MaxPayloadBytes` 覆盖）返回 `ErrPayloadTooLarge` 并**零写入**，绝不静默截断（ADR-0008）
- **运行路径接入**：`agentrun.RunRecorder` 新增 `SetPayloadStore`，在阈值（32KB）以上把事件流/输出快照经 `AgentPayloadExternalizer` 外置到制品库，运行行内只保留可解析的 marker；外置失败只告警，不影响运行记录写入

**验证（真实 PostgreSQL，探针 27 项 0 失败，探针已删除）**

- 往返一致：JSONB `payload`、`metadata`、`text_content`、`content_hash` 全部原样读回；`Resolve` 对 `agent_run_trace` 的 marker 还原出**完全相同的 2413 字节**事件流
- 幂等与冲突：同 id 同内容重复 Put 通过；同 id 不同内容返回 `idempotency conflict`
- 体积上限：911 字节正文在 256 字节上限下返回 `ErrPayloadTooLarge`，且 `artifact_documents` **零行**（不静默截断）
- 删除语义：首次 Delete 成功，之后 Get/Delete 均返回 `ErrNotFound`
- 运行路径：大事件流被外置（运行行 `trace_events` 变成 marker、含 `artifact_backend`/`payload_size_bytes`），小输出快照保持内联；`artifact_documents` 1 行、`artifact_refs` 1 行且 `storage_backend='postgres'`；小事件流不外置且不新增制品行

**探针发现并修复的真实缺陷**：`Put` 中的 `if err := db.Raw(...).Scan(&x); err != nil` 误把 `*gorm.DB` 当作可判空的错误 —— `*gorm.DB` 永不为 nil，因此**每次 Put 都会提前返回零值且错误为 nil**，写入被静默丢弃（`artifact_documents` 始终 0 行）。改为检查 `.Error` 字段后写入正常。

**M4：MCP 客户端接入（INK-013）**

- 新增 `internal/agent/runtime/application/mcp`：`Service` 负责注册 MCP Server、拉取工具清单（TTL 缓存）与调用工具；`MCPClient` 端口 + `ClientFactory` 便于替换传输实现
- 工具清单缓存落 `agent_mcp_tool_cache`（键与表唯一约束一致：`user_id, server_name, server_url, auth_fingerprint`），并把清单回写到 `agent_mcp_servers.tools_json`、记录 `last_test_status`
- 新增 HTTP 面：`GET/POST /v1/agents/mcp/servers`、`DELETE /v1/agents/mcp/servers/:id`、`GET .../tools`（`refresh=true` 强制回源）、`POST .../tools/:name/call`
- **密钥合规**：`auth_header` 列只接受 `env:NAME` 引用，明文凭据注册即被 400 拒绝（`ErrMCPAuthHeaderMustBeEnvRef`）；缓存键只使用鉴权头的 `sha256:` 指纹，任何路径都不落明文（ADR-0004）

**验证（真实 PostgreSQL + 进程内传输桩，探针已删除；0 失败）**

- 明文鉴权头被拒；注册后 `SELECT auth_header` 得到 `env:PROBE_MCP_TOKEN`，库中不含明文
- 首次 `ListTools` 回源解析出 2 个工具（含 `inputSchema`）；第二次 `from_cache=true` 且**回源计数仍为 1**；`refresh=true` 后回源计数变 2 —— 缓存命中与强制刷新都可复现
- `CallTool` 成功（回源计数 1）；`tools_json` 已回写、`last_test_status=ok`；缓存行只存 `sha256:` 指纹
- 删除 Server 后查询返回 `ErrMCPServerNotFound`
- HTTP 层：明文凭据 → **400 `mcp_auth_header_must_be_env_ref`**；注册 → 200 且响应体不含 env 引用；列表/删除 → 200；无令牌 → 401

**安全边界（如实说明）**：MCP 地址经 `httpguard` 校验，**回环与私网地址会被拒绝**（探针实测 `host resolves to blocked ip 127.0.0.1`）。因此本地伪装的 MCP Server 不能作为被调目标，真实传输链路的端到端验证需要一个公网可达的 MCP 端点；本次以进程内传输桩验证了服务层逻辑与落库行为。

**M4：Prompt 装配、Prefix-Cache 与上下文压缩降级（INK-014）**

- 新增 `internal/agent/runtime/application/prompt_assembly.go`：把 Agent 定义装配为四段 Prompt
  （`core_instruction` / `agent_definition` / `tool_guidelines` / 易变的 `user_input`），
  **稳定前缀**按内容哈希缓存在 `platform/cache.ServiceCache`；缓存键刻意不含用户输入，否则永不命中
- 运行路径接入 `promptctx`：`RunRequest` 携带 `prompt_sections` 并注册 `PromptSectionMiddleware`，由中间件在模型调用前合成 System Prompt 并生成 `prompt_cache`（ephemeral 断点）元数据；运行轨迹新增 `prompt_assembled` 记录（段列表、易变段、缓存命中、Redis 状态）
- `platform/cache.ServiceCache` 新增**可选**的本机回退（`EnableLocalFallbackOnRedisError`）：Redis 已启用但读写失败时回退本机内存并打印 WARN。默认关闭，跨实例强一致的调用方不受影响
- 上下文压缩补上确定性回退：模型侧 `GenerateHandoffDocument` 失败（未绑定辅助任务/模型）时改用 `fallbackExtractHandoffDocument`，压缩不再完全失效；原先该函数**从未被调用**（死代码）

**验证（真实 PostgreSQL + Redis + 本地模型桩，探针已删除）**

- Prompt 装配与 Prefix-Cache：运行轨迹含 `prompt_assembled`，四段齐全，且带 `prompt_cache` 元数据；`redis-cli FLUSHALL` 后首跑 `stable_cache_hit=false` 且 Redis 键数 0→1，次跑 `stable_cache_hit=true`
- Redis 故障降级：`docker stop inklore-redis-1` 后 `/v1/agents/run` 仍 **200** 且内容正常，`stable_cache_hit=true`（命中本机内存），日志出现 WARN「Redis 不可用，已回退本机内存缓存」；`docker start` 后继续 200
- 压缩回退：不绑定辅助任务时 `OptimizeMessages` 仍生成交接文档（61 条消息压缩 59 条、保留 3 条，文档含 `[上下文交接文档]`）

**已知缺口**：模型侧交接压缩需要「辅助任务 + 辅助模型」的配置来源，而 `AgentAuxiliaryTask` 目前只有领域类型与校验、没有任何读取来源，已登记 **INK-050**。

**M4：工具审批端到端接入（INK-012）**

- `platform/auth` 新增 `WithPrincipal` / `PrincipalFromContext`，`middleware.Auth` 校验通过后把认证主体**同时**写入 gin 上下文与请求上下文，使域内授权判定无需依赖传输层（新增 [ADR-0009](docs/decisions/0009-principal-context-and-approval-authorization.md)）
- 新增 `adapters.NewApprovalPrincipalResolver`：只信任请求上下文中的主体，要求 `UserID` 与审批归属一致、角色照搬；主体不带权限时按「当前无权限中心、平台令牌即操作员凭证」补通配权限；认证强制且无主体时直接拒绝
- `runtimeapp.Service` 新增审批服务依赖；`RunInput` 透传 `UserID` / `ApprovalTokens` / `PermissionLevel`，并把运行记录 ID 作为 `RunID` 关联到审批单（审批单可按运行追溯）

### Gin HTTP 收敛（INK-190–203，实施中）

- 新增 ADR-0020、原生 Terra/high 子代理协作说明、路由/契约/删除验收矩阵。
- 请求 Schema 从冻结机读源生成并嵌入二进制，启动离线编译；严格输入、身份幂等、SSE 与中间件收敛验收状态见 backlog。
- 七域JSON请求统一，移除Body回填/反射必填；LLM与幂等规范化复用纯JSON解析。
- Agent身份/幂等旁路删除，当前身份及资源权限在事务内外复核；SSE编码/写截止/FlushError/续传与断线资源处理收敛。
- Gin Allow/OPTIONS、精确CORS、安全日志与可信代理统一；删除旧响应、未挂载认证和realtime广播链。完整交付仍待认证扩展契约裁决，见INK-203。

### R1.x认证扩展（INK-209，验收中）

- 用户授权保留验证码、独立creator注册和密码找回，新增ADR-0021并补齐严格机读契约。
- 注册/重置统一密码边界；重置令牌事务失败可重试，并发请求不能双消费；未配置邮件投递时不返回虚假发送成功。
- PostgreSQL已确认使用系统17.11，dbx读取通过；本机现有库无需跨主版本迁移。

### PostgreSQL运行基线（INK-210）

- 按用户选择采用系统PostgreSQL17.11，新增ADR-0022，Compose新部署使用pgvector/pgvector:pg17。
- dbx已实际读取inklore_r1；现有库已在17，无跨主版本数据迁移。旧PG16卷升级前必须独立备份/迁移，不允许只换镜像原地挂载。

- INK-201/203：补齐安全HTTP日志request_id白名单；真实PG17只读完整App启动与11HTTP场景通过，中间件panic/流后恢复/日志脱敏探针通过。
- INK-199/200/201：修复Agent及文件DELETE忽略未声明body，拒绝重复/额外revision查询；删除SSE的last_event_id查询后备。公共guard七请求场景通过。

### INK-250：模型配置手写组件拆分（review）

提取受控 Token 字段组件，修复 ModelConfigRoute 超过50KB的问题，保持保存与校验路径。16项生成契约超限仍待独立裁决，未宣称完整护栏通过。

### INK-249：七个 Go 文件机械格式修复（review）

标准 gofmt 调整七个已记录源码文件，未改业务语义；核实无具体并发修改后补齐两个 novel_creation.go 的格式，不接管 INK-236 功能。全仓格式与 Go 构建通过，不代表其他门禁或独立验收完成。

### INK-248 补充：dot 本地构建入口（待完整验收）

复用 Makefile 聚合全包构建、文档/架构护栏、pnpm 冻结安装与双端构建；显式支持已核验快照的 BUILDVCS=false，不跳过 notes 历史基线。dot 单项构建通过，完整门禁和独立验收仍阻塞，未合并或发布。

### INK-251：源码大小上限500KiB（review）

按用户明确裁决更新AGENTS与archguard，512000字节以内允许；保持其他门禁与扫描范围。新增ADR0037，不改历史ADR/冻结资产/文档独立长度门禁。真实构建、架构护栏与边界验证通过，未合并。
