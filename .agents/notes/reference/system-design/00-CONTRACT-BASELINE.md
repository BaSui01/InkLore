> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# InkLore 重构设计基线 R1

状态：冻结设计；尚未实现。任务 INK-141。日期2026-09-18。本文件锁定全部分册共享语义；详细字段由 api/*.json 的 OpenAPI 3.1 定义，实际建表由 data/*.sql 定义。此处“必须”指重构验收要求，不代表旧代码已经满足。旧实现不限制新设计，不提供旧路由兼容层。

R1.1勘误：[ADR-0018](../decisions/0018-minimal-contract-errata.md)补齐本人改密幂等头、下载416、审计导出422与未声明方法405错误定义；不改变业务边界、权限、TTL和DDL。

## 1. 边界与交付

七域 content、outline、worldview、inspiration、compliance、agent、llm；platform 是共享技术基座，app 是唯一装配根。HTTP API与Worker同仓库同二进制，可按role运行；不是微服务。领域只通过消费方Port与app/adapters联系，不跨读写私有表。只读值类型可跨域，写对象不可共享。content拥有作品/卷/正文/版本；outline拥有大纲/节拍/场景/伏笔；worldview拥有角色/设定/关系/账本；inspiration拥有外部资料和建议资产；compliance拥有检查策略/报告；agent拥有执行/工具/审批/产物；llm拥有模型调度/策略/用量。

依赖是能力方向：content→outline/worldview/compliance（导出仅读风险审阅结果），agent→content/worldview/outline/inspiration/compliance/llm，inspiration→llm，compliance→llm；outline/worldview/llm不依赖其他业务域。只读作品归属验证由platform ScopeAuthorizer提供认证授权投影，不允许outline引用content仓储；投影事务由app适配维护。AI执行统一由agent调用领域能力，content不反向调用agent。UI发起生成到agent，作者接受后写content。跨域消息只触发投影/失效，不成为正文写入后门。

R1含全部核心创作与管理闭环：作品生命周期、正文编辑版本确认、三级大纲/场景/伏笔、角色设定图谱、拆书文风灵感、质检、Agent工作流/审批、模型与工具治理、文件导入导出、作品内全文检索、北京时间写作统计。外部平台自动投稿、协同实时编辑、图片生成、TTS、订阅支付、SSO和跨租户全局搜索明确不在R1；不得在界面放假入口。所有核心R1功能都必须实现，不以待定/后补结束设计。

## 2. 标识与全局值

所有实体ID、tenant_id、user_id为UUID v4，HTTP小写连字符字符串；revision为int64正整数，从1开始，每次成功变更+1。同一资源业务修订只增不回退，历史版本另有UUID。租约心跳、lease_epoch、fencing_token是技术并发字段，不递增业务revision；业务状态迁移仍递增。机器key用小写snake_case，错误code用domain.reason（如content.version_conflict），API字段snake_case。时间TIMESTAMPTZ存UTC；HTTP RFC3339 UTC Z；UI/自然日Asia/Shanghai。正文规范为UTF-8纯文本、LF换行，保留用户空行/缩进；字数统计Unicode码点（不含空白）；客户端选区UTF16须转换为码点offset再发后端，源revision+content_hash绑定所有报告/建议定位。

通用标题1..120码点；角色/用户显示名1..60；简介0..2000；正文每章≤200000码点；模型上下文上限min(模型声明,60000Token)，输出预留从预算扣除。JSON请求≤2MiB；输入文件≤20MiB；导出产物≤100MiB。所有限制服务端最终校验，超限明确拒绝、不截断；租户可降低运行预算但不能绕过硬上限。

## 3. HTTP冻结规则

前缀/v1，按域：/v1/content、/v1/outline、/v1/worldview、/v1/inspiration、/v1/compliance、/v1/agents、/v1/llm；技术端点/v1/platform、/v1/auth。参数{novel_id}/{chapter_id}统一snake_case。无旧/v1/novels路由兼容。创建POST=201，更新/命令/删除=200，后台任务受理=202，GET=200；删除也返回JSON，不使用204。

成功：{code:"ok",data:<typed value>,request_id:<uuid>,trace_id:<32hex>}。失败：{code:"domain.reason",message_key:"errors.domain.reason",details:<safe object>,request_id:<uuid>,trace_id:<32hex>}；业务判断不依赖本地化文案。details允许field_errors[{field,reason}]、current_revision、retry_after_seconds、conflicts、operation_id，禁止SQL/堆栈/正文/密钥。错误HTTP：400请求格式、401认证、403明确权限、404不存在或跨作用域、405已知路径不支持方法（带Allow）、409状态/版本/幂等冲突、413体积、416非法Range、422业务约束、429容量/预算、500未知、502上游协议、503依赖、504上游超时。

可变实体PUT/PATCH/DELETE以及基于当前状态的命令必须携expected_revision（DELETE查询参数；其余JSON），缺失400 platform.validation_failed，过期409 <domain>.version_conflict且不写。创建/更新/删除/有副作用业务命令都必带Idempotency-Key UUID，查询不需要。认证login/refresh/logout/ws-ticket及ADR-0021的注册/验证码/密码找回命令使用专用身份事务而非通用结果缓存，不缓存令牌响应。幂等表键(tenant,actor,method,canonical_path,key)，规范化请求HMAC-SHA256（独立环境注入密钥，避免密码请求摘要成为离线字典），24h保存；同键异载荷409 platform.idempotency_conflict；进行中409 platform.operation_in_progress并Retry-After:1；完成重放同状态与原结果；响应丢失后同键可安全重试。不可把request_id当幂等键。副作用外呼另有执行边界，未知上游结果不自动重发。

列表统一cursor分页：limit默认20范围1..100，opaque cursor绑定tenant/actor/filter/sort；data={items:[],next_cursor:null|opaque,has_more:boolean}。固定排序updated_at DESC,id DESC（不可变事件created_at/id或seq）；cursor失效400 platform.cursor_invalid。空列表200，不用404。错误全局表见14；分册不得另设数字错误码或ERR_*别名。

## 4. 身份与权限

私有部署、多租户；R1.x依据ADR-0021允许公开注册独立creator租户，禁止公开注册管理身份。platform持有用户、成员关系、会话、授权投影；账号由CLI引导或管理员创建。密码按[ADR-0028](../decisions/0028-password-minimum-length.md)要求至少6个Unicode字符、最多72 UTF-8字节，bcrypt cost12；不内置账号口令；保留用户名root可在Creator与Admin audience登录；登录按audience校验活动membership，Creator持续执行owner scope（ADR-0031）。角色creator/operator/auditor，角色并集但接口audience严格creator/admin；访问令牌JWT HS256有效15分钟，签发密钥只环境注入，claims sub/tid/aud/roles/session_id/exp。刷新令牌随机256bit，服务端只存hash，轮换、30天绝对到期、重复使用吊销整个session。两端不同HttpOnly刷新cookie名；Secure生产、SameSite=Strict、Path=/v1/auth；AccessToken仅内存。刷新/退出校验Origin与CSRF token，双端不是靠端口代替服务端授权。

creator仅访问自己所属tenant且拥有作品scope的创作资源；operator可管理本租户配置/成员但不读取作者正文；auditor看脱敏运行与配置审计，不看原始正文。审批仅发起运行的本人可决定，管理员不可代批。所有对象读写含tenant与资源owner scope；跨scope404。运行详情正文仅本人且当前仍具作品权限；审计默认只显示摘要/指标。权限版本变化下次请求立即生效，JWT角色只作提示，服务端按session+membership revision校验。R1不支持管理员跨租户通配权限。

## 5. 保存、确认与跨域事务

正文保存与chapter_versions同一content事务；正常编辑将confirmed→draft，不改变已确认旧事实。回滚是新版本，永远draft。章节确认请求含expected_revision、expected_world_revision及作者审阅的fact_proposals；content应用通过WorldviewFactPort在同一PostgreSQL UnitOfWork提交确认标记与只追加账本。UnitOfWork由platform提供不透明context，domain不暴露*gorm.DB；app/adapters传递同一事务上下文，各域仓储只操作自有表。锁顺序：scope授权→novel→chapter→worldview_revision，顺序固定防死锁。worldview维护可更新fact_heads投影，但fact_ledger_events本体只INSERT；修正用supersede/retract事件。旧正文事实保留来源版本，不能宣称回滚自动撤销。冲突整批拒绝409 worldview.fact_conflict；覆盖必须明确逐条resolution与理由，不提供allow_override全局绕过开关。

ID幂等记录、资源写、版本、outbox同事务提交。长模型调用/文件解析禁止持数据库事务。投影失败可重放outbox，不改变已提交结果。跨域导出/运行取快照，不直接join私表。软删除30天，恢复同一ID与新revision；被引用事实仍可审计，权限即时撤销；到期擦除正文/文件，账本保留去标识化证明而非原文，90天安全审计保留。

## 6. Agent作业与SSE/WS

POST /v1/agents/runs 返回202持久化run_id；GET /v1/agents/runs/{run_id}/events 为该请求生成过程SSE订阅，GET不会重新调用模型。后台任务PG队列(非Redis队列)，lease30秒、每10秒心跳、最大技术重试3次指数退避1/2/4秒；运行副作用通过execution_key与工具幂等登记防重。状态queued/running/waiting_approval/cancelling/succeeded/failed/cancelled；终态不可回到running。POST /runs/{id}/cancel是幂等命令；暂停审批后approve/reject通过CAS消费票据并恢复同一run，绝不重跑已完成副作用。票据15分钟有效，绑定run/node/tool/arguments_hash/source_revision；过期终止该节点，运行failed，不自动批。

SSE事件id为run内递增seq，事件名run.started、text.delta、reasoning.summary、tool.started、tool.completed、approval.required、node.updated、usage.updated、run.succeeded、run.failed、run.cancelled；原始隐藏推理不向用户披露。envelope={run_id,seq,occurred_at,payload}，每15秒注释心跳；Last-Event-ID回放7天事件，过期410 agent.event_cursor_expired（先GET run取终态）；重订阅只重放，无模型调用。服务端持久化事件后发送，客户端seq去重；终态恰一条，无[DONE]第二完成标记。取消与终态竞争CAS终态为准；断开订阅不取消后台run；用户离页明确告知继续运行，可主动取消。

WS /v1/platform/events仅低频通知{event_id,topic,resource_id,revision,occurred_at}；不传正文、Token增量或工具参数。使用POST /v1/auth/ws-ticket获60秒一次性票据，URL只传票据不传JWT；断连退避1..30秒，重连拉权威资源，通知不保证完整历史。

## 7. 复用和基础设施

PG17(+pgvector1536维、pg_trgm)唯一业务权威；文件统一platform_files的bytea，20MiB输入/100MiB输出、SHA256、tenant/owner授权，不能用Base64混入JSON。下载走鉴权端点；没有对象存储/外链签名URL第二方案。Redis7仅缓存、跨进程限流/并发租约；Redis不可用时AI admission返回503 llm.capacity_unavailable，非AI CRUD继续；readyz仅检查PG与必要启动配置，状态标degraded。

前端两个独立app creator:3000/admin:3001，strictPort；共享UI/HTTP/SSE/i18n/主题/状态组件，禁止导入另一端app。React19/TS5.8/Vite7/antd6/Lucide；ReactRouter7路由、TanStackQuery5服务端缓存、Ajv8+formats3表单schema验证，锁版本入lockfile；后端Go1.25/Gin1.12/GORM1.31/pgx5，JSON Schema使用santhosh-tekuri/jsonschema/v6(Draft2020-12)，其余沿go.mod。正文编辑用原生textarea +领域控制器，不新加富文本库；图用可复用SVG/Canvas+等价表。共享不是通用god-service：见20分册的边界/输入输出/依赖/错误定义。

本地恢复固定IndexedDB，按tenant/user/novel/chapter键，停输2秒写最新副本，7天TTL每章最多3份，每主体100MiB软上限；AccessToken/刷新token不入库。退出主动清除该主体副本，shared-device模式默认关闭恢复并提示。无持久化权限/配额失败显示持续告警，允许文本下载；不能保证断电最后2秒不丢。自动保存30秒停输/60秒最大间隔，单章单在途写；冲突后停自动提交、三方合并人工确认。

## 9. R1.x Agent策略版本与中途澄清增量（ADR-0027，2026-09-24）

仅新增Creator安全任务策略目录与Run不可变策略版本绑定，以及Agent中途澄清。其余模块化单体、七域DAG、Agent只产Artifact建议、Run配置快照、幂等/CAS、PG持久作业、SSE持久回放与未知外部副作用不重发均不变。

Creator `GET /v1/agents/catalog/task-policies` 用creator audience/role及tenant scope，cursor分页；只投影已发布且当前启用、tenant内可用并能力授权的策略：policy ID/version ID、task_key、title/version、版本配置的task_kinds、安全输入Schema及预算概览。task_kinds由Admin写入不可变LLM策略版本；既有版本升级后映射为空，不猜测或自动映射。禁止系统指令、Provider secret、内部路由和工具参数。Agent经consumer Port调用LLM应用服务读取，不读LLM私表。CreateRun七个创作分支改为必需`task_policy_version_id`，禁止与自由`task_key`并传；服务端同Run/Job/幂等事务验证tenant、启用/current version、task_kind兼容并派生task_key。既有`policy_version_id`为Run的冻结字段；实际Worker每次Gateway调用消费其版本，不能按key重查当前版本。Admin配置发布新版本不漂移旧Run；停用只阻止新建。旧task_key只在服务/客户端滚动部署的单一兼容窗口内经tenant启用策略解析，未知拒绝；升级完成后关闭并拒绝缺version请求，不做静态映射。

中途澄清与工具副作用`approval.required`严格分离。Run/Node新增`waiting_clarification`；单Run最多8个Question且同时最多1个open。Question ID/version、node、task policy/definition版本、源revision/hash、发起人和期限绑定；只允许安全单选/多选/文本Schema子集，最大8KiB、深度8、题面4KiB、选项20、答案4KiB；拒绝远程引用、组合/条件Schema、HTML、权限/配置/秘密字段。title/help使用locale keys，纯文本上下文摘要不超1000码点。答案作untrusted data，不覆写正文/事实。

`question.required`与`question.answered`是agent_run_events持久SSE事件，统一run_id/seq/occurred_at/payload并用Last-Event-ID回放7天；answered事件仅含answer hash/actor，不回显答案。question状态、run/node checkpoint、事件和job挂起/唤醒在同一PG UoW与固定锁序/CAS下提交。提问释放lease、挂起原job retry_wait，不新增Run/Job、不计重试，恢复使用原job及原checkpoint。回答POST带question revision/version、结构化answer与Idempotency-Key；本人/tenant/source scope重验，幂等重放返回原响应，同题并发不同键单CAS胜出。总Run 30分钟deadline包含等待，Question 1..15分钟且不越过Run deadline。取消、答复、超时、撤权竞争以CAS先提交者为准；过期将Run fail并写终态事件；撤权拒绝读答且保留服务端期限收敛。断流/退出不取消，前端GET Question/Run并从SSE游标恢复同Run；隐藏推理、工具参数、Secret/正文全文不得入事件。

DDL唯一Owner源为`data/agent.sql`（R1.x Question增量追加于该Agent域DDL）；向前运行迁移为`migrations/r1/000011_agent_clarification.sql`。升级事务保留旧Run/events，downgrade禁止且应用回退保留扩展schema。


## 10. 冻结与校验

R1为实施基线，任何相互矛盾条款必须在本轮消除，所有核心功能的实现选择必须在本轮明确。以后新增需求或发现设计缺陷需R1.x变更记录，列影响接口/迁移/前后兼容/验收；不得声称技术设计永不可能修正。不改动业务代码以假装设计达标。删除旧文档前确保有替代位置和链接重定向；旧决策已迁至.agents/notes/reference/decisions作依据，历史冲突由有效替代关系裁决，不建立平行决策库。设计检查校验OpenAPI refs、路径参数、operationId、封包/权限、SQL表Owner、功能映射、局部引用、文件<50KB。

认证扩展裁决见[ADR-0021](../decisions/0021-auth-extensions.md)：验证码可选且成对；注册返回200 Session；登录tenant_id可省略；找回未接入邮件投递时非开发环境返回503，开发响应sent=false且可含短期dev_token。
