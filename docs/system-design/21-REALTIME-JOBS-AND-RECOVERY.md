# 21 · SSE、后台作业与恢复协议

R1冻结。事件Schema唯一源[events.json](api/events.json)，运行HTTP见[OpenAPI](api/openapi.json)。此协议替代旧POST直接生成流、session_*与[DONE]事件。SSE仍独占某次请求的生成过程；GET订阅只是读取该run，不触发新模型调用。

## 1. 创建与订阅时序

1. 客户端冻结输入资源ID/revision/hash、task_kind、选区及策略；POST /v1/agents/runs带Idempotency-Key，服务端鉴权+scope+完整schema验证，通过同事务写run(queued)、job、input snapshot及outbox，202返回run_id/revision。
2. UI GET /v1/agents/runs/{run_id}/events，Authorization Bearer，Accept:text/event-stream；采用fetch ReadableStream，不能用无法携Bearer头的原生EventSource。首次不传Last-Event-ID。
3. Worker真正开始后写run.started；文本先保存事件再发送，不能先显示后落库。服务端每50ms或累计4096码点合并文本片段落一条text.delta，发送顺序seq递增。
4. 各LLM attempt先独立结算usage并返回receipt；run终态事务保存artifact、引用已结算receipts的汇总、run终态和唯一终态event，不重复插入或延后所有attempt结算；终态之后不再写任何新进度，断开连接。正常没有额外done或[DONE]。

GET错误发生在响应头前用JSON4xx/5xx；已经建流后应用失败持久化run.failed（Worker与cancel/reject命令都通过同一RunService.Terminate执行CAS+终态事件事务），仅订阅传输故障直接断开，由客户端重订阅查询，不替worker编造run.failed。服务器panic不能把JSON封包混写SSE。

## 2. 帧格式与终态

~~~text
id: 12
event: text.delta
data: {"run_id":"<uuid>","seq":12,"occurred_at":"<UTC Z>","payload":{"artifact_id":"<uuid>","delta":"正文片段"}}

: heartbeat

~~~

seq正整数且不跳回，occurred_at UTC；事件名与Schema oneOf判别匹配。客户端以run_id+seq去重，乱序暂缓存最多100条/5秒，不能补齐则重新GET回放，不排序后假装完整。

| 事件 | payload语义 | UI动作 |
| --- | --- | --- |
| run.started | task_kind/model_id（路由未定可null） | 从排队改执行，不据此宣称产出 |
| text.delta | artifact_id+delta | 追加对应候选；不是正文自动保存 |
| reasoning.summary | 简短执行摘要 | 折叠展示，不暴露隐藏推理 |
| tool.started | node/tool/invocation标识 | 真工具执行卡 |
| tool.completed | success/result_ref/error_code | HTTP成功与工具业务成功分开 |
| approval.required | approval/node/expires_at | 显示本人审批卡，参数摘要另GET票据 |
| node.updated | node_id+status | 更新图和等价表，不伪造百分比 |
| usage.updated | 累计tokens/cost_micros/CNY/estimated | 替换累计数而非再累加，防回放重复计费展示 |
| run.succeeded | artifact_ids+revision | 解锁完整候选采纳 |
| run.failed | code+retryable+revision | 保留partial标识；retryable只提示人工新请求可能性 |
| run.cancelled | reason+revision | 显示取消；不能以客户端abort确认服务端取消 |

只有run.succeeded/failed/cancelled为终态，数据库partial unique(run_id) where name terminal保证最多一条。tool.completed false或node失败不等于终态，调度可按版本化策略恢复；UI直到run终态才结算任务结果。

## 3. 回放与订阅恢复

事件7天保留，订阅每次scope重新验证；Last-Event-ID为该run的最后完整seq，回放seq>cursor，边界不重复。游标不存在/大于最大值400 agent.event_cursor_invalid；已清理且不能连续回放410 agent.event_cursor_expired，客户端读取run详情和已保存artifact，不悄悄开启新run。run不可见404，角色不能以重连绕过。

心跳15秒注释帧，无id、不入库。60秒无字节判断传输异常；客户端最多8次重订阅，退避1/2/4/8/16/30/30/30秒+0..20%抖动，不重POST。达到上限保留结果和“重新连接/查询状态”。JWT过期刷新单飞后重新GET；刷新失败停止订阅显示登录覆盖层。终态已在GETrun看到但SSE终态未收，可显示“已结束，正在同步产物”，产物读成功后完成。

页面切走默认后台继续，显式文字提示；组件卸载abort只断订阅。停止生成按钮POSTcancel，携run expected_revision+幂等键；409先GET终态，不重发换键。晚到成功与取消CAS竞态以服务器终态为准。

## 4. PG队列与租约

platform_jobs调度状态queued/leased/retry_wait/succeeded/failed/cancelled，不与agent_run状态混用。SQL用FOR UPDATE SKIP LOCKED领取，lease30秒、心跳10秒、lease_epoch每次递增；每个run只对应一个platform job，CreateRun同UoW入队并记录platform_job_id；每次节点attempt记录job_epoch与独立node fencing_token。所有checkpoint同时验证job当前owner/epoch/lease和node fence，旧worker即使网络恢复也不能提交。锁序platform job→run→node→execution→approval；节点不能绕过平台job独立领取执行权。

job handler输入只domain/kind/resource_id与少量安全元数据，正文从领域快照读。常规技术重试最多3次（初次+3次=4 attempts），退避1/2/4秒；数据库已提交结果必须先查询receipt再执行。已发出模型/有副作用工具请求而结果未知，不因lease到期重发；Gateway内部llm.upstream_outcome_unknown在Agent边界转换agent.execution_outcome_unknown且retryable=false，原码仅安全cause审计，预算按unknown结算。唯一自动重试例外是登记为read_only且无计费/无副作用的白名单工具，仍限3次且复用源快照；模型调用即使读语义也有计费，绝不适用例外。幂等只保证本地副作用，不能保证无幂等上游exactly-once。

崩溃恢复：读持久化节点checkpoint→跳过succeeded/skipped→pending/ready可执行→inflight无receipt标unknown→等待审批恢复计时→已终态只补outbox/预算对账。等待审批时Suspend同run job为retry_wait，available_at为最早票据到期；approve后Wake为queued（有效持有者仍在时只通知），不得创建第二个job。正常等待/唤醒不计技术失败attempt；attempt计初次外加技术重试最多4次，租约claim次数由epoch记录。队列不会把已终态run重新改running。重试生成是新run并重新提示计费，不能修改历史run。

## 5. 审批

票据绑定本人、tenant、run/node/tool版本、arguments_hash、source_revision；15分钟到期，状态pending/consumed/rejected/expired/cancelled，不存在approved中间态。Approve/reject必须expected_revision和Idempotency-Key；按固定锁序锁job/run/node/execution/票据，approve同事务从pending CAS到consumed、写一次执行授权并将原节点ready唤醒同run，worker再执行已登记的invocation_key，不二次消费许可。参数变化要求新节点票据，不能携旧批准继续。到期由worker定时检查后将该节点/run失败，释放未花预算；拒绝终态cancelled(reason approval_rejected)。operator不可代批。

UI批准后不重POSTruns；如果响应丢失，同key查询/重放命令不会二次执行。源章在等待期修改使票据失效agent.approval_source_changed；命令检测到source changed时，在同一失效事务CAS票据cancelled、node/run失败、run.failed事件，并将409 agent.approval_source_changed作为已提交的命令结果缓存；不能抛error导致失效事务回滚。用户重新发起新run，不静默取最新正文授权。

## 6. WS

POST /v1/auth/ws-ticket返回60秒票据；GET /v1/platform/events?ticket=...升级101，一次消费且校验Origin/audience。日志删除query；不传JWT进URL。消息仅{event_id,topic,resource_id,revision,occurred_at}，topic白名单run.terminal、job.terminal、resource.changed、budget.warning、system.degraded；服务端scope过滤。消息至少一次但不承诺离线完整，客户端event_id去重、按revision失效对应QueryKey，重连后重读当前页面数据。

## 8. R1.x Agent Question恢复协议（ADR-0027）

澄清不是approval.required：Approval授权具体副作用执行，Question只收集完成当前node所需的Creator输入，不创建工具执行许可。Run/Node新增waiting_clarification，Run最多8条question、同时1条open。Question持久记录question_id/version=1、tenant/run/node/owner、冻结policy_version_id/definition_version_id、source revision/hash、限定response_schema、locale title/help keys、纯文本context_summary、created/expires、status与answer hash。题目schema≤8KiB、深度≤8；只有单选、多选（≤20）或文本，拒绝远程ref/default/组合条件/HTML/secret/权限/task-config字段。题面4KiB，answer≤4KiB。答案是untrusted data，仅加入Run checkpoint，不修改作品正文、事实或源版本。

发问事务在有效job/node fence下锁定job→run→node，插入Question、将node/run设waiting_clarification、持久化checkpoint及`question.required`+`node.updated`，将原platform job挂起为retry_wait并释放lease；所有写同一UoW，job_id保留、deadline不重置、等待不算技术重试。只允许一个open问题防止多个并发node恢复冲突。等待时不发delta/进度，不持有模型/DB事务。

Question由GET `/v1/agents/runs/{run_id}/questions/{question_id}`恢复；Answer为POST同资源`/answer`，Idempotency-Key必需，body expected_revision、question_version、schema answer。以job→run→node→question锁序验证本人、实时tenant/作品scope、权限、source revision/hash、policy/definition binding、expires及status后CAS answered；answer/hash/actor/time、node ready、run queued、原job wake、`question.answered`和outbox同事务。相同键载荷重放原结果，不再次wake；并发不同键仅首个提交，输方409；幂等同键异载荷使用公共冲突。客户端断开无影响，Answer成功后UI只重新订阅同run，worker从同checkpoint继续。

Question deadline为1..15分钟，不能超过Run固定30分钟deadline；PG now()定时过期，question expired、node/run failed并写唯一run.failed终态事件。cancel/answer/timeout/source change按锁序CAS先胜；权限撤销使GET/answer/SSE立即403并清客户端敏感缓存，worker仍按原超时/总deadline收敛。question.required/answered写入agent_run_events与状态原子提交，SSE envelope及7天Last-Event-ID规则不变；required传验证schema/题目/期限/source绑定，answered只传question_id/version/answer_hash/actor_id/time。不得传隐藏推理、答案内容、工具参数、Secret或正文全文。410恢复先GET权威Run/Question；绝不新建run。Question过期/取消不撤销或重放外部执行；未知副作用仍按agent.execution_outcome_unknown。

验收增量：在question required写事务故障时状态/Question/event/job全部回滚；回答同键重放不二次唤醒；两个不同key并发仅一胜；权限撤销/来源revision变更/取消/过期竞态只出现一个状态结果和顺序正确的持久事件；杀worker后原job/checkpoint恢复且模型/工具已完成次数不增；SSE从Last-Event-ID恢复两种Question事件，7天后410后GET恢复，不泄露answer正文/隐藏推理；无回答时不伪造进度，按TTL/Run deadline失败或用户显式取消。

RT-01 创建POST丢响应同键只一run；RT-02 SSE断连重新GET无新模型调用；RT-03 text.delta回放去重；RT-04 cursor过期410读artifact；RT-05已发模型后kill worker不自动再次计费；RT-06审批两窗口只一批准，恢复原run；RT-07源修订变化使票据失效；RT-08cancel与成功竞态只有一terminal；RT-09过期JWT/跨scope重连不泄露；RT-10代理15秒心跳持续刷新无buffer；RT-11隐藏推理/Key无event泄露；RT-12WS重复/漏通知只影响刷新，不覆盖权威正文。
