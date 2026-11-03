> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# 管理控制台功能与交互合同 R1

状态：INK-141 冻结目标，尚未实现。遵循 [00 基线](00-CONTRACT-BASELINE.md)、[17 设计系统](17-DESIGN-SYSTEM-AND-ACCESSIBILITY.md)、[20 复用合同](20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md)；HTTP wire由 [API入口](../../../../docs/system-design/api/openapi.json) 定义。本册能力名用于页面与领域适配器连接，不创建另一套HTTP协议。

## 1. 身份、导航与共同约束

Admin为独立端口3001、strictPort应用；admin audience允许operator或auditor，creator-only主体不能访问。角色并集不扩大租户：operator维护本租户配置与成员，不读取作者正文；auditor只读脱敏运行/配置/审计，不读正文、完整prompt、工具敏感参数或密钥。两端没有切换按钮，也不导入另一端页面。

/login 输入email或username二选一、password、tenant_id、audience=admin；email合法邮件格式且≤254字符，tenant_id可省略，提供时为UUIDv4；无注册和内置口令；username仅允许root；Admin须有活动operator/auditor membership或global super_admin，Creator username登录另见ADR-0031。AccessToken仅内存，15分钟有效；HttpOnly session cookie与creator名称不同，刷新轮换、30天绝对到期，重复使用由服务器吊销session。启动先GET /v1/auth/session-csrf?audience=admin取nonce，再POST /v1/auth/refresh；刷新/退出校验Origin与X-CSRF-Token，失败清会话并重新登录。本人改密码PUT /v1/auth/password携current_password/new_password及principal.user_revision作为expected_revision，成功后重新登录。/account显示name、roles、tenant、session及改密码/退出；密码至少6个Unicode字符且≤72 UTF-8字节（ADR-0028），展示名1..60码点，日志不含口令。

路由ReactRouter7，查询TanStackQuery5，表单Ajv8 Draft2020-12加formats3，控件antd6/Lucide。PageGate先验证会话及权限再请求资源；跨tenant/scope404，明确禁止操作403。JWT roles仅用于初步导航，服务端按membership/session revision即时检查。成员权限变更事件到达清scope缓存；未收到事件下次请求仍必须服务端拒绝。

页面共同字段：所有资源id/tenant_id可省略，提供时为UUIDv4、created_at/updated_at为UTC时间、可变资源revision正整数；UI北京时间显示。列表limit20默认、1..100，游标绑定筛选/排序，默认updated_at DESC,id DESC；审计按created_at/id倒序。只读查询不发幂等键；所有写入必带Idempotency-Key，PUT/DELETE/状态命令必带expected_revision。DELETE revision放query。表单不让用户更改tenant_id、Owner或系统时间。

## 2. 全页状态、表单与命令恢复

下表适用于A01..A17所有页面；各页额外列出主要empty、partial与stale情形，不能据此省略共同态。

|状态|显示/动作|恢复|
|---|---|---|
|loading|骨架、aria-busy，未知不显示0；禁依赖读取的写入|成功后才初始化表单|
|empty|真实200空集合，说明创建入口或只读无数据|无权限者不显示可写空态按钮|
|error|首次失败区域显示错误码/request_id与重试|保留筛选/脏表单；不能默认造数据|
|offline|持续离线条，配置缓存只读，禁命令|联网先重读配置revision，危险命令不自动排队|
|stale|最后成功时间、过期徽章|刷新不覆盖dirty，409进入DiffView比对|
|partial|独立区块成功保留，失败区独立重试|关键依赖缺失仅禁相关命令|
|forbidden|清敏感缓存、取消请求、显示无权限|恢复权限需重新认证/授权读取，不循环刷新|
|ready|按能力显示操作|写后权威返回及相关列表失效|

表单clean→dirty→validating→submitting→clean；422回dirty，409 conflict，断线/超时unknown。unknown同键同载荷核对重放，绝不新键自动再执行探测或工具。上游结果未知的外呼不能自动重发；展示operation_id与运行检查器。500/502/503/504均按是否已受理区分，不以网络失败推断业务回滚。429按retry_after_seconds显示恢复时间，不自动换模型。配置保存显示返回revision/current_version_id；容量观测显示独立capacity_observed_at，不能杜撰applied_revision字段；新run固定快照，旧run不被配置编辑改写。

危险确认弹窗列资源名、影响引用、当前revision、动作后果，默认取消。密钥界面只有secret引用及脱敏元数据；环境变量/Secret Store注入，禁止输入API Key正文、明文入库、响应回显或复制。所有诊断结果脱敏，管理端没有作品正文查看权限。

## 3. 页面与功能追踪

### A01 模型大盘 /overview

字段来源：GET /v1/llm/channels 的breaker_state/inflight/remaining_rpm/capacity_observed_at/capacity_available；GET /v1/llm/usage 的status/input_tokens/output_tokens/cost_micros/duration_ms；GET budget的spent_micros/reserved_micros/remaining_micros。组件OverviewCards+TimeRange+HealthTable+UsageChart及等价表。主刷新，次跳转渠道/用量；operator/auditor只读。无独立GetStatus聚合接口；图表仅统计当前已加载usage集合，标题标“已加载N条”、筛选范围及has_more，未加载全量不得声称租户总调用量/百分位。分页继续读取由用户操作，不假装后台聚合。
empty无调用样本，不显示100%成功；渠道与usage独立加载；容量不可用显示unknown/degraded，不把null当0；部分请求失败partial保留成功区，stale显示capacity_observed_at。enabled仅配置，不等于健康。追踪A01 / FR-22：HealthTable→llm_list_channels，UsageChart→llm_list_usage/llm_get_budget→llm；验收范围标识准确、Redis故障AI503而非非AI CRUD失败、跨租户不聚合。

### A02 供应商 /providers

字段：Provider(code,title,protocol,enabled,revision)，protocol=openai|deepseek|anthropic|gemini|ollama；title1..120，code按Schema机器键。base_url属于Channel，不在Provider表单发送。组件ProviderTable+SchemaForm+ConfigDiff；主operator创建/保存，次禁用/删除与查看渠道；auditor只读。GET/POST providers、GET/PUT/DELETE providers/{provider_id}；PUT完整字段+expected_revision，写入带幂等键。
empty引导登记；渠道引用区partial不能声称无引用；最终删除由服务端约束，409保留安全原因。stale人工比较；禁用影响新admission，旧run保持快照。追踪A02 / FR-21：ProviderForm→llm_*_provider→llm；验收未知protocol拒绝、并发409、被引用删除不级联。

### A03 渠道 /channels

字段：Channel(provider_id,model_id,title,base_url,upstream_model,priority,weight,rpm,concurrency,enabled,revision)及breaker_state=closed|open|half_open、inflight、remaining_rpm、capacity_observed_at、capacity_available。base_url禁止userinfo/凭据query，受服务端网络策略约束；priority0..100、weight1..1000、rpm1..100000、concurrency1..1000；title/upstream_model1..120、base_url≤2048，0不等空。组件ChannelTable+ChannelForm+RuntimeStatus；主配置，次启停/删除/查看引用，operator写、auditor读。
empty先建立Provider与Model；依赖partial禁提交；stale显示采样时间与配置revision。没有指定渠道探测端点，因此渠道不放“测试本渠道”按钮；诊断跳转A10并明确按task_key策略选择渠道，结果不能当指定渠道健康证明。breaker_state只读，编辑enabled不冒充熔断复位。追踪A03 / FR-21：ChannelForm→llm_*_channel→llm；验收禁用影响新调度、旧run快照不变、容量不可用显示未知而非零。

### A04 凭据引用池 /keys

字段：Key(channel_id,title,secret_ref,weight,rpm,concurrency,enabled,fingerprint,health,revision)，secret_ref={kind:env|secret_store,locator,version}，health=unknown|healthy|degraded|invalid。组件KeyReferenceTable+SecretReferencePicker+RotationDialog。主operator从GET /v1/platform/secret-references允许列表绑定引用并保存，次启停/解绑/调权重；auditor只读API返回的脱敏元数据，不调用仅operator允许的引用目录。任何角色都不返回Key正文，UI不提供粘贴API Key输入框。
GET/POST keys及GET/PUT/DELETE keys/{key_id}；新引用按服务端允许项组装typed secret_ref，禁止自由输入任意环境变量。empty解释部署注入后绑定；引用目录partial禁保存；stale重新读；解绑只解除注册引用，不删除环境secret。追踪A04 / FR-21：SecretReferencePicker→platformListSecretReferences，KeyForm→llm_*_key→platform/llm；验收网络/日志无明文凭据，同键不重复，禁用不删除渠道。

### A05 模型与价格 /models

模型字段：Model(provider_id,upstream_name,title,capabilities,context_window,max_output_tokens,embedding_dimensions,enabled,revision)，不是model_key/name。capabilities=text|tools|structured|embedding；上下文正整数且实际≤min(模型声明,60000)，输出预留计入同预算，embedding_dimensions按Schema固定能力。组件ModelTable+CapabilityTags+ModelForm；主operator登记/修改，次禁用/删除/看策略，auditor读。empty无目录不内置模型，依赖partial禁提交，stale不自动换第一项。
价格页签 /models?tab=pricing：Pricing(model_id,currency,input_micros_per_million,output_micros_per_million,cache_micros_per_million,effective_at,version)。主operator新增价格版本POST /v1/llm/pricing，次GET列表/详情；价格只追加，不提供PUT/DELETE。微单位价格按Schema整数校验，使用Intl格式化显示但提交整数，不累计二进制浮点金额；effective_at传UTC显示北京。empty提示未定价，error禁止以0价估算；旧Usage.price_snapshot固定，新增价格不改旧费用。追踪A05 / FR-21、FR-22：ModelForm→llm_*_model；PricingForm→llm_create_pricing/list/get→llm；验收引用删除409、价格新版本不重算既有用量。

### A06 任务策略 /task-policies

字段：Policy(task_key,title,enabled,task_kinds,primary_model_id,fallback_model_ids,max_output_tokens,temperature,timeout_seconds,max_retries,required_capabilities,output_mode,current_version_id,version,revision)。output_mode=text|structured|embedding；temperature0..2、timeout_seconds1..180、max_retries0..3、max_output_tokens1..16384、fallback_model_ids最多3个且唯一；task_kinds为1..16个唯一task kind并随不可变策略版本发布；模型/回退能力与预算服务端校验。组件PolicyMatrix+RowEditor+ConfigDiff+ImpactList。主operator创建或PUT保存发布新策略版本，次启停/删除/看引用；auditor读。task_key来自Schema允许键，失败不硬编码可保存业务行。
GET/POST task-policies，GET/PUT/DELETE task-policies/{policy_id}，不把task_key当路径ID。创建/发布新版本必须显式配置1..16个适用task_kinds；既有无映射版本显示“未配置”，不可进入Creator目录，不能自动推断。单行命令独立反馈，不宣称多行原子。empty可创建，模型partial禁保存，stale某行冲突保留其他dirty；task_kinds 1..16个唯一项随新不可变版本发布；旧版本空task_kinds不提供给Creator。备用模型必须用户显式配置，不自动替换失效模型。追踪A06 / FR-22：PolicyMatrix→llm_create_policy/llm_publish_policy/llm_delete_policy及查询→llm；验收temperature=0保持0、timeout_seconds正确、发布产生新current_version_id、旧run仍旧策略版本。

R1.x安全策略目录面向Creator的安全投影由Agent适配器消费LLM公共查询服务构造；Admin仍仅经本端LLM策略治理能力发布不可变版本。发布新版本不改既有Run；停用立即阻止新Run但允许Run消费其冻结策略版本。此处不增加Admin问答/代答界面，Question事件和用户答案不得进入Admin审计正文，只展示脱敏状态与审计摘要。



字段：Budget(daily_limit_micros,business_date,spent_micros,reserved_micros,remaining_micros,over_budget,currency,revision)；可写仅daily_limit_micros（0..9000000000000000安全整数）+expected_revision，PUT /v1/llm/budget。没有token_limit/period/concurrency_limit表单；运行Token预算在CreateRun/领域预算，渠道并发在A03。金额为微单位整数，Number.isSafeInteger及Schema范围验证；北京时间business_date显示自然日预算。组件BudgetForm+UsageTable+UsageChart+LimitImpactDialog；主operator保存，次查用量，auditor只读。
Usage明细含invocation_id/attempt/reservation_id/policy_version_id/model_id/channel_id/key_id/pricing_revision_id/input_tokens/output_tokens/cached_input_tokens/cost_micros/adjustment_micros/price_snapshot/estimated/estimate_reason/status/error_code/duration_ms；status=succeeded|failed|cancelled|unknown。GET usage列表/usage/{usage_id}详情；reservation_id是追溯字段，不虚构Reservation列表接口。empty无用量不等于无限预算，partial未知预留不填0；降低限额显示可能阻断新admission，不抹掉spent/reserved。预留→结算/释放由LLM原子执行，前端不发送退款命令。追踪A07 / FR-22：llm_get_budget/configure_budget/list_usage/get_usage→llm；验收并发不透支、unknown不自动再次外呼、429区分容量/预算、auditor写403。

### A08 智能体定义 /agents

字段：Definition(definition_key,title,enabled,system_instruction,tool_version_ids,task_kinds,current_version_id,version,publication_status,revision)。system_instruction属于配置，不在前端硬编码；tool_version_ids引用不可变工具版本，task_kinds按Schema。组件AgentDefinitionTable+DefinitionForm+ToolPermissionList+ConfigDiff；主operator创建/保存，次启停/删除/查看版本标识，auditor读取安全配置。GET/POST definitions、GET/PUT/DELETE definitions/{definition_id}；PUT完整输入+expected_revision生成配置版本，publication_status=draft|published按返回呈现，未暴露独立发布命令。
empty引导创建，工具依赖partial禁工具修改提交，stale保留对照；新版本不修改旧run快照。受控诊断是A10固定probe，不是任意definition试跑。追踪A08 / FR-23：DefinitionForm→agent_*_definition→agent；验收版本持久化、禁用阻断新选择、工具版本引用校验、管理请求无作品正文。

### A09 工作流治理 /workflows

字段：Workflow(workflow_key,title,enabled,manifest,current_version_id,version,manifest_hash,revision)；manifest.nodes元素{node_key,role,depends_on,required,definition_version_id?,tool_version_id?,input_bindings}，binding={target_parameter,source_node,source_pointer}。没有独立edges或input_schema wire字段；图的边从depends_on派生，输入绑定按JSON Pointer选择前驱typed输出。role=supervisor|structure|writer|polisher|reviewer|tool。组件WorkflowTable+WorkflowGraph+NodeTable+SchemaForm+VersionSummary。主operator保存新版本，次启停/删除/查看引用；auditor读。GET/POST workflows、GET/PUT/DELETE workflows/{workflow_id}；PUT是保存配置版本，不调用不存在PublishWorkflow/DisableWorkflow，启停是enabled字段CAS更新。
节点键唯一、依赖存在且DAG无环，required输出可达；empty新建，依赖Schema partial禁保存，stale人工合并；图位置只本地偏好。运行固定workflow_version_id，新保存不改旧run。追踪A09 / FR-16：WorkflowGraph→agent_create_workflow/update_workflow/delete_workflow及查询→agent；验收环拒绝、绑定指针校验、键盘表能完成图编辑、旧run版本不变。

### A10 工具目录与受控诊断 /tools

字段：Tool(tool_key,title,enabled,input_schema,output_schema,side_effect,approval_required,timeout_seconds,mcp_server_id,remote_tool_name,current_version_id,version,revision)，side_effect=read_only|transactional|external。组件ToolTable+SchemaForm+RiskPanel+DiagnosticInspector。主operator保存工具配置，次参数Schema校验预览/启停/删除；auditor只读。GET/POST tools及GET/PUT/DELETE tools/{tool_id}，SchemaForm验证结构及实例，但“参数预览”不执行工具，不暗造execute端点。
受控诊断POST /v1/agents/diagnostics，仅输入task_key与probe_key=text_connectivity|structured_validation；显示会产生用量，返回持久化诊断run后GET diagnostics/{run_id}轮询status/output_summary/error_code。不接受任意prompt/工具参数/作者正文，不通过creator run/events/cancel或approvals路径。离开只停轮询，受控探针继续至终态；无诊断cancel API则不设取消按钮。
empty无工具；Schema失败禁配置提交/校验预览；partial缺风险配置禁启用，stale工具版本变化重验表单。追踪A10 / FR-23：SchemaForm→agent_*_tool；DiagnosticInspector→agent_create_diagnostic/get_diagnostic→agent；验收只有两种probe、请求无任意正文、超时核对同run不自动新建诊断。

**R1.x 现场变更（INK-247，ADR-0035 Proposed，优先于上述 R1 冻结目标的工具配置 UI 条款）**：工具不再由管理员通过 REST 创建、编辑、启停或删除。Admin 工具页只读调用 `GET /v1/agents/builtin-tools`，目录条目来自 Go `ToolExecutor` 已安装 handler，当前租户的状态基于内部不可变工具版本核验；没有版本时显示“当前租户未接通”，不能标为可执行。原 POST/PUT/DELETE 工具配置端点及机读 operation 已移除，GET /tools 仅保留历史与冻结配置读取。部署 CLI 只注册代码允许的内置能力，不开放任意工具定义。其他工作流、智能体定义与诊断契约不因本段增加新的在线配置能力。

### A11 MCP服务器 /mcp-servers

字段：MCPServer(title,endpoint,env_ref,allowed_methods,enabled,revision)，无transport/url/auth_ref/health假字段。组件McpServerTable+McpServerForm+BoundToolTable；主operator登记/更新，次启停/删除和查看已有工具绑定；auditor脱敏配置读。endpoint遵守服务端网络策略，env_ref从授权引用选择，禁止Key/header明文；allowed_methods按Schema明确方法列表。GET/POST mcp-servers、GET/PUT/DELETE mcp-servers/{mcp_server_id}；绑定工具来自GET tools按mcp_server_id筛选，只有当前加载范围可声明。
无refresh/discover专用API，不显示自动发现/刷新远端工具按钮；注册远端工具通过A10 ToolInput的mcp_server_id/remote_tool_name/input_schema/output_schema配置并版本化。empty引导登记；工具列表partial不影响服务器只读，影响判断不声称全量；stale CAS重读；最终删除由服务端引用约束。追踪A11 / FR-23：McpServerForm→agent_*_mcp_server，BoundToolTable→agent_list_tools→agent；验收URL策略拒绝、env_ref不泄secret、修改工具版本使旧参数校验不能复用。

### A12 运行审计 /runs 与 /runs/:runId

字段：AuditRun(id,revision,created_at,updated_at,kind,task_kind,status,node_count,duration_ms,cost_micros,error_code)。组件RunFilter+RunTable+RunInspector(mode=redacted-summary)。GET /v1/agents/audit/runs与audit/runs/{run_id}，operator/auditor可读；管理端不调用creator-only runs/{id}/events/artifacts/cancel。没有nodes细节/actor_id/model_id/工具参数等字段，不造完整轨迹；RunInspector只接受typed摘要，缺少明细不显示假空节点列表。
主查看安全摘要，次刷新/复制诊断标识/跳转本端审计日志。运行中可每10秒GET当前摘要，页面隐藏停止轮询；WS仅失效后重读。empty无运行；error不冒充空，usage关联失败partial不补0，stale显示最后GET时间；offline只读快照，无代作者取消/重试/采纳入口。追踪A12 / FR-17：RunInspector→agent_list_audit_runs/get_audit_run→agent；验收网络响应也无正文、跨租户404、刷新不创建run。

### A13 审批审计 /approvals

此管理页只观察安全审计，真实票据读取与approve/reject仅creator audience的本人C15拥有。字段来自Platform Audit：actor_id/action/resource_id/outcome/request_id/trace_id/created_at与安全摘要，按审批动作过滤；不调用creator-only /v1/agents/approvals，不读取arguments_json。组件AuditTable+ApprovalAuditSummary+RunInspector(redacted-summary)，主看记录，次查看对应安全run摘要；operator/auditor均只读，无决定按钮和15分钟操作倒计时。
empty无匹配审计；某run摘要不可读partial显示受限，不猜票据；offline保留脱敏缓存，stale可刷新，403清scope。审批事实仍是pending→consumed/rejected/expired/cancelled；本页记录只作审计，不作为当前票据权威状态。追踪A13 / FR-17、FR-24：platformListAudit+agent_get_audit_run→platform/agent；验收管理员没有代批/本人诊断审批假入口、伪造creator票据请求拒绝、审计不泄参数。

### A14 成员与会话 /members

字段：Member(id,user_id,email,display_name,roles,status,revision)，status=active|disabled；email为合法邮件格式且≤254字符。会话区仅提供本租户该成员全部会话撤销确认与返回revoked_count，不虚构会话列表接口；roles只能creator/operator/auditor非空集合，显示名1..60，账号按Schema唯一约束，初始化密码至少6个Unicode字符且≤72 UTF-8字节，提交后清空内存。组件 MemberTable + MemberForm + RoleMatrix + SessionDrawer + DisableDialog。主operator创建成员/更新角色，次停用/恢复成员、撤销会话、重置密码；auditor仅脱敏成员审计投影，不能看会话安全细节或写入。无邀请邮件发送机制、公开注册仅创建独立creator租户，不创建管理身份。
防止移除本租户最后一个可用operator，服务端事务校验，UI预检不替代并发锁。停用立即使下次请求拒绝；清除用户角色并不删除作品/事实。密码重置POST /v1/platform/members/{member_id}/reset-password，由operator输入新密码并携expected_revision，成功后清空输入，不自动发送邮件、不记录日志、不由服务端生成并缓存明文。仅账号有一个active tenant membership可重置；多租户账号403 auth.cross_tenant_account，指引本人修改密码或部署CLI。撤销POST /v1/platform/members/{member_id}/revoke-sessions携expected_revision，仅该租户会话。empty仅首次引导已有CLI operator后的成员列表，不允许“零管理员”自动造账号；session partial不妨碍读成员但禁依赖清单的批量撤销；stale角色409保留拟定选择。
追踪 A14 / FR-24：MemberForm/SessionDrawer→Platform.ListMembers/GetMember/CreateMember/UpdateMember（角色、active/disabled）/RevokeSessions/ResetPassword→platform；验收两个operator同时自降权至少一请求冲突、已撤销session下次请求401/403、角色变更不等JWT15分钟到期、不同tenant看不到名字。

### A15 配置与安全审计 /audit

字段：Audit(id,actor_id,audience,action,resource_id,outcome,request_id,trace_id,safe_changes,created_at)；只追加、无更新删除按钮。组件 AuditFilter + AuditTable + SafeDiff + ExportDialog。主筛选查看，次导出授权范围的脱敏审计文件；operator/auditor均需audit读取能力，导出独立鉴权。列表时间范围受90天保留窗约束；导出GET /v1/platform/audit-events/export，from/to窗口≤31天且≤10000条，返回application/x-ndjson，每行typed Audit；超限422要求缩小筛选，禁止默默截断。
empty无匹配事件，error不能显示“无违规”；diff partial不能把缺失字段当删除；stale只表示最后刷新时间。导出是同步鉴权GET直接下载NDJSON，不创建后台job或platform文件记录，不传JWT URL；审计保留90天，正文软删证明去标识化不留原文。
追踪 A15 / FR-24：AuditTable→platformListAudit（行详情使用返回的typed Audit）/platformExportAudit→platform；验收所有配置写与失败权限请求有安全审计、导出不含密钥/正文、过滤游标不可跨主体重放。

### A16 系统观测 /operations

字段：Status(observed_at,service_version,state,dependencies,queue,outbox)，state=ready|degraded|not_ready；queue={queued,leased,failed,oldest_wait_seconds}，outbox={pending,dead_letter}；dependencies为typed依赖状态。组件 DependencyCards + QueueTable + OutboxTable + IncidentDetails。主刷新，次查看安全诊断标识；operator/auditor只读，R1不在UI暴露任意SQL、Shell、直接改队列表。健康指标由platform安全聚合能力提供，不把Prometheus纯文本按成功封包解析。
readyz仅检查PG与必要启动配置；Redis故障标degraded，AI503，非AI CRUD继续。队列lease30秒/每10秒心跳，技术重试最多3次1/2/4秒；UI只展示queue中的真实计数与最老等待秒数，不构造未返回的逐job attempt/lease列表。outbox失败由worker重放，不向用户提供无审计“补写数据库”。
empty队列无待办不是观测失败；单依赖error可partial；时间过期stale禁止断言系统正常；offline保留最后快照但状态未知。
追踪 A16 / FR-24：DependencyCards→platformStatus：GET /v1/platform/status 一次返回依赖/队列/outbox typed概览→platform/各域只读Port；验收Redis断开仍可改非AI配置、PG不可用ready失败、worker lease过期可接管且execution_key防重。

### A17 质检规则治理 /quality-rules

字段：RuleSet(name,version_no,state,checksum,revision)，state=draft|published|retired；Rule(rule_set_id,kind,expression,severity,category,advice,weight,enabled,revision)，kind=sensitive_literal|ai_phrase|ai_pattern，severity=warning|block，expression1..256、category1..60、advice≤2000、weight1..100。组件RuleSetTable+RuleEditor+PublishSummary+RuleValidationErrors；主operator新建/克隆草稿、编辑草稿规则并发布，次看历史；auditor只读。POST rule-sets输入name与可选base_rule_set_id克隆，规则POST/PUT/DELETE各带expected_revision；发布POST rule-sets/{id}/publish。没有UpdateRuleSet/DisableRuleSet入口，已发布集不直接编辑；新发布使原版本retired，规则enabled在草稿内修改。
Report固定原规则版本，新发布不改历史。empty创建；单规则失败partial保留输入但整集发布需全部校验；stale重验整集；offline禁发布。管理端不调用creator扫描接口获取作者正文，表达式合法性由保存/发布验证。追踪A17 / FR-15：compliance_create_rule_set/list/get/list_rules/create_rule/update_rule/delete_rule/publish_rule_set→compliance；验收非法expression422、克隆不改源集、发布CAS无半版本、报告仍旧规则版本。

## 4. 领域状态与失败补偿总表

|对象|UI允许状态行为|权威提交/补偿|
|---|---|---|
|Provider/Model/Channel/Key|enabled开关与运行health分开|LLM配置CAS+幂等+outbox；失效传播失败重放outbox，不把已提交配置撤销|
|Workflow/RuleSet|Workflow用PUT保存版本与enabled开关；RuleSet创建/克隆草稿再publish|Owner验证整份快照；RuleSet新发布使旧版retired，不可变历史不被编辑覆盖|
|Run|queued/running/waiting_approval/cancelling/succeeded/failed/cancelled|agent持久队列与CAS；终态不可返回running；未知副作用人工核对|
|Approval|pending→consumed/rejected/expired/cancelled，仅creator本人消费|agent校验本人、hash、source revision、15分钟；恢复原run|
|Budget reservation|预留、结算、释放由账项显示|LLM负责原子admission及补偿；UI不直接减已用量|
|Membership|active/disabled及角色并集|platform同事务更新revision、session授权投影、审计/outbox；最后operator保护|
|AuditEvent|只读不可变|platform只追加；90天保留策略由服务端执行|

读性能需求：LLM目录按(tenant_id,updated_at,id)，策略按(tenant_id,task_key)，用量按(tenant_id,occurred_at,id)，run按(tenant_id,status,created_at,id)，审批按(tenant_id,actor_id,status,expires_at,id)，成员按(tenant_id,user_id)唯一及(tenant_id,updated_at,id)，审计按(tenant_id,created_at,id)和(tenant_id,resource_type,resource_id,created_at,id)。实际DDL由data分册Owner交付；前端不得读取其他域私表或在共享包建立领域仓储。跨域大盘由各域只读Port组合，局部失败可独立重试。

## 5. 验收矩阵

A01..A17每页记录：正常真实数据→八态→字段上下界→权限角色→并发CAS→响应丢失同键重放→成功后GET→键盘/读屏。账户与成员另外覆盖session刷新轮换、重复refresh吊销、最后operator保护。安全证据检查浏览器网络响应而非仅DOM：auditor无正文、所有身份无Key正文、跨scope404无资源名。

运行链验收：配置保存revision N，新run引用N；发布N+1后原run仍N；审批必须本人；断开SSE不取消；显式cancel与成功竞争服从权威终态；GET订阅无新执行。外呼未知不自动重发，保留operation_id进行核对。预算/Redis降级、文件导出、审计追加各用真实请求与故障注入证据，不以静态图表或假进度作为通过。本文不宣称实现已验收。
