# Agent 与工作流域 R1 冻结设计

状态：INK-141 冻结目标，尚未实现。唯一跨域契约为 [基线](00-CONTRACT-BASELINE.md)。本域细则见 [详细设计](domains/07-DOMAIN-AGENT-DETAIL-SPEC.md)，实际表见 [agent.sql](data/agent.sql)，机读接口见 [运行](api/agent-runs.json)、[治理](api/agent-admin.json)、[类型](api/agent-schemas.json)。不以旧代码、旧路由决定设计。

## 1. 职责、角色与用例

Agent 拥有运行、工作流版本、节点、工具注册、审批、执行凭证、建议产物、可回放事件与上下文来源；不拥有正文、事实、大纲、灵感或质检报告。UI 的所有 AI 生成首先创建 AgentRun；不存在公开 llm generate 接口。creator audience 的 creator 仅操作本人且仍有作品权限的 run；独立inspiration_analyze按本人analysis/asset权限校验，可无novel；admin audience 的 operator 管配置并发起受控 diagnostic；auditor/operator 仅能读取脱敏审计，不读作者提示词、正文、工具参数和产物。本人审批不能由 operator 代替。诊断 run 只访问系统合成输入，无 novel_id、无用户数据工具、无外部写能力。

| 用例 | 发起者 / 输入 | 结果及边界 |
| --- | --- | --- |
| AG-01 创建运行 | creator；definition_id、task_kind、task_key、resource(novel_id/resource_id/source_revision)、instruction、可选workflow_id | 202 run_id；冻结策略、工作流、工具版本与来源；PG 排队 |
| AG-02 查询/回放 | 本人 GET run/nodes/artifacts/events | GET 不驱动模型；事件保留7天；断线后台继续 |
| AG-03 审批 | 本人；approval_id、expected_revision、arguments_hash、source_revision | 一次 CAS 消费票据；恢复同一 run 与冻结节点 |
| AG-04 取消 | 本人；run_id、expected_revision | 200 当前状态；取消和完成竞争以首次终态 CAS 为准 |
| AG-05 工作流治理 | admin operator；节点、依赖、版本 | 完整校验后发布不可变版本；已有 run 不漂移 |
| AG-06 工具治理 | admin operator；Schema、副作用类别、MCP env 引用 | 注册不等于授权；禁止任意 shell/任意 URL/事实写工具 |
| AG-07 诊断 | admin operator；task_key、固定 probe_key | 202 受控 diagnostic run；LLM 用量照常入账 |
| AG-08 审计 | admin operator/auditor | 仅状态、节点类型、耗时、费用、错误码与追踪ID |

instruction 1..8000 码点；task_key 为已启用策略 key，1..64 snake_case；每 run 最多64节点、128边、每节点15个 TAO 轮次、最多8个并行工具调用、run并行节点最多4；run墙钟上限30分钟（包括审批等待），节点单次模型调用上限180秒、工具上限120秒，审批15分钟。参数 JSON ≤2MiB，单工具参数≤64KiB、结果≤256KiB；大产物正文最多200000码点，文件由 platform FilesPort 保存，不塞 Base64。新 run 每作者最多3个非终态，超额429 agent.run_capacity_exceeded。

## 2. 主流程与事实接受

RunService.Create 在鉴权后读取来源 revision、冻结任务策略版本与工作流 manifest，完成 DAG/能力/预算预检；同一 UoW 经JobsPort.Enqueue创建run级platform job并写run.platform_job_id、nodes、上下文来源、幂等结果与outbox，返回202。worker先领取唯一run job，再在有效JobLease内执行ready节点，Think 经 GatewayPort 产生文本或结构化工具意图，Act 经 RegistryPort/ApprovalService/ExecutionService，Observe 仅将通过 Schema 的结果及来源写回上下文。完成节点后开启下游节点；所有必需节点succeeded时调用RunService.Terminate CAS写run.succeeded；worker与cancel/reject命令共用该唯一终态入口。

正文生成、润色、大纲建议、设定建议都成为 immutable Artifact。UI 显示来源 revision/hash 和适用目标，作者接受时调用对应 Owner 的写接口（content 保存正文、outline 写大纲、worldview 写设定）；必须携 Owner expected_revision 与新的 Idempotency-Key。Agent 不提供直接 accept-to-facts 写入口，不自动调用 ConfirmChapter。内容确认另走 content→worldview 原子事实落账。产物只是建议，即使质量检查通过也不是事实。旧 revision 产物允许阅读，接受前必须明确合并，不能覆盖新正文。UI在本人Agent产物读取鉴权后取得文本与hash，向Owner提交明确文本、来源标识与expected_revision；Owner仅校验自身scope/目标revision/输入hash，不反向依赖Agent。接受回执属于Owner，Agent的展示标记不是事实权威。

## 3. TAO、调度与持久事件

run：queued→running→waiting_approval→running；任何非终态可→cancelling→cancelled；queued/running/waiting_approval 可→failed；running→succeeded。终态 succeeded/failed/cancelled 无出边。node：pending→ready→running→waiting_approval→ready；running→retry_wait→ready；running→succeeded/failed；waiting_approval因到期/源变→failed；未完成节点→cancelled/skipped。run 只有在无可运行/在途节点且存在待审批节点时才 waiting_approval；有并行可运行节点时保持 running，审批票据仍可存在。

PG17 platform_jobs为唯一队列，粒度run，FOR UPDATE SKIP LOCKED领取；lease30秒、10秒心跳、job lease_epoch递增。每个node attempt冻结job_epoch并递增fencing_token；所有worker提交双校验job epoch与node fence。锁序platform job→run→node→execution→approval，heartbeat只改技术字段不加业务revision。技术重试最多3次（首次之外3次，退避1/2/4秒），只用于已证明未dispatch、明确未受理或无计费无副作用read_only白名单。外部副作用结果未知进入 execution unknown，节点 failed、run failed（已接受取消则run cancelled并保留同一error_code），错误 agent.execution_outcome_unknown；不能自动重发或重跑整个 run。只读和本地事务工具凭 execution_key 去重；上游支持幂等仍须查询结果或携同一上游幂等键，禁止换键重试。

模型未知结果llm.upstream_outcome_unknown在Agent映射为agent.execution_outcome_unknown，retryable=false；不得因模型为读取型调用而重试。审批pending→consumed并恢复ready同事务，无approved中间态；实际来源变化同事务失效票据/节点/run并写终态事件，提交并缓存409 agent.approval_source_changed，不回滚失效。

每个状态变更与agent_run_events同事务提交，按固定锁序持job/run分配seq；事件先落库后发送。唯一终态事件索引保证最多一条，状态 CAS 事务保证成功终态必有一条。GET /v1/agents/runs/{run_id}/events 是请求级 SSE，只订阅和回放，Last-Event-ID 是该 run 的十进制 seq；缺省从1回放。事件名与 envelope 完全遵循基线及主公共 SSE 协议，禁止隐藏推理，仅 reasoning.summary。15秒注释心跳；事件7天TTL，游标过期在打开流前410 agent.event_cursor_expired；先 GET run 获取终态。流已打开之后失败必须写 run.failed，不改发 HTTP JSON；WS仅发送无正文终态通知。

## 4. 安全和复用

Agent ContextAssembler→ContentContextPort/OutlineContextPort/WorldviewContextPort/InspirationContextPort；Agent ReviewTool→CompliancePort；上述端口由 app/adapters 调用对应域应用服务。Agent GatewayPort→LLM GatewayService；Agent FilesPort→platform FileService；Agent ScopePort→platform ScopeAuthorizer。工具本地执行器按注册 key 绑定明确 Port，不能通过模型字符串动态导入包。外部 MCP 仅 HTTPS allowlist，连接凭证仅 env://引用，禁止 stdio 任意命令；授权固定 tenant/tool/method/capability，模型输出不能提升权限。

所有写命令幂等24h；PUT/DELETE/状态命令 expected_revision 强制 CAS；同键异载荷409。跨 scope404。公共错误与通用 schemas 引用公共分片；领域常见错误 agent.invalid_dag、agent.approval_expired、agent.approval_binding_mismatch、agent.context_budget_exceeded、agent.loop_detected、agent.execution_outcome_unknown、agent.version_conflict。所有失败封包无正文、Schema实值、Secret和堆栈。

## 6. R1.x 增量（ADR-0027，2026-09-24）

Creator安全任务策略目录与版本消费归llm策略Owner；Agent提供consumer Port，经app/adapters读取只投影已发布、tenant启用、能力授权的策略，Creator不调用Admin API。目录投影限定policy_id、policy_version_id、task_key、title/version、LLM策略不可变版本中由Admin配置的task_kinds、安全输入Schema、预算概览。R1既有版本因没有task_kind mapping迁移为空，Creator目录过滤为空数组；不做隐式/硬编码映射。CreateRun创作请求以task_policy_version_id唯一指定策略，服务端原子核验tenant/current/enabled/task_kind，再派生task_key；既有agent_runs.policy_version_id冻结该版本。真实worker的每次LLM调用必须传此版本ID，绝不运行时按task_key重查头。Admin发布新版本不漂移旧Run，停用拒绝新Run。旧task_key请求仅滚动升级兼容窗口内服务端fail-closed解析，之后sunset。

Agent中途澄清是独立Question状态机，不是工具副作用审批。Run/Node新增waiting_clarification，Question绑定policy/definition版本与resource revision/hash，最多8题、1题同时open；安全单选/多选/文本schema限8KiB、深度8，题面4KiB、20选项、答案4KiB。答案是untrusted data，不改正文/事实。持久`question.required`/`question.answered`事件不含隐藏推理或原始答案；问题状态、Run/Node checkpoint、事件和同一PG job挂起/唤醒原子提交。本人回答以revision/version及Idempotency-Key CAS，恢复同Run/job/checkpoint；Last-Event-ID回放和GET恢复，不创建新Run。过期、取消、source变化及权限撤回遵守锁序与CAS，deadline等待计入Run 30分钟上限；过期fail并终止，撤权拒绝读答，外部未知结果不重发。Creator前端呈现题目并恢复订阅，Admin只看脱敏摘要。完整wire与验收见ADR-0027、domains/07细则和21分册。

1. 重复提交同一创建键只生成一个run，GET/SSE重连不增加模型调用；seq去重后文本一致。
2. 动态DAG含环/悬空边/未知工具/超过64节点，在队列写入前422；合法并行节点最多4个。
3. 杀死worker后租约到期可恢复；旧fence提交失败；已完成execution不重复；未知外部副作用明确失败。
4. 15分钟票据过期不能批准；另一个作者或管理员代批失败；参数/来源变化不能复用票据；批准恢复相同run。
5. 取消与成功同时发生，只保留一个终态和一个对应终态事件；取消后的晚到delta不外发。
6. 硬锚点超限拒绝；弱来源被裁剪但留下来源清单；最大窗口含输出预留≤min(模型窗口,60000)。
7. 产物不修改正文；作者接受发生Owner CAS冲突时不覆盖；审计接口不出现原文与工具参数。
