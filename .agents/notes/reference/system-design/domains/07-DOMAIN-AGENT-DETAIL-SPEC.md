> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../../AGENTS.md)；实现以源码和本轮验证为准。

# Agent 详细设计 R1

冻结目标，INK-141；遵守 [基线](../00-CONTRACT-BASELINE.md)。入口 [域规范](../07-DOMAIN-AGENT-WORKFLOW-SPEC.md)，DDL [agent.sql](../../../../../docs/system-design/data/agent.sql)，契约 [运行](../../../../../docs/system-design/api/agent-runs.json)、[治理](../../../../../docs/system-design/api/agent-admin.json)、[类型](../../../../../docs/system-design/api/agent-schemas.json)。

## 1. 聚合与不可变量

所有表 UUIDv4主键、tenant_id→platform_tenants(id)、created_at/updated_at TIMESTAMPTZ；可变聚合 revision 从1递增。作者 FK→platform_users(id)。同域外键含tenant以阻止跨租户关联，业务域外引用仅UUID，不join私表。

| 聚合/表 | 字段与不变量 |
| --- | --- |
| AgentDefinition / agent_definitions | key、title、enabled、current_version_id；模板指令≤8000码点、允许tool版本列表和task_kind；禁在代码内长提示词 |
| DefinitionVersion / agent_definition_versions | immutable版本号、system_instruction、tool_ids、task_kinds；PUT发布新版本，旧run冻结version_id |
| Workflow / agent_workflows + agent_workflow_versions | 可变头与immutable manifest；1..64 nodes，≤128 edges；工作流版本保存完整typed manifest |
| Tool / agent_tools + agent_tool_versions | tool_key、input/output JSON Schema、side_effect、approval_required、timeout_seconds、MCP绑定；新版本不改变旧执行输入 |
| MCPServer / agent_mcp_servers | HTTPS endpoint、env_ref、allowed_methods、enabled；只接受部署许可清单内的endpoint与env_ref组合，不返回秘密 |
| AgentRun / agent_runs | owner_user_id、platform_job_id（同tenant唯一FK至platform_jobs）、kind、task_kind、resource_type/resource_id、novel_id、source_revision、instruction、definition/workflow/policy UUID、status（含waiting_clarification）、revision、next_seq、deadline；diagnostic无作品绑定 |
| AgentQuestion / agent_run_questions | question_id/version、owner、run/node、policy/definition版本、source revision/hash、限定response_schema、status、expires_at、answer/hash/actor/time；最多一个open/run，答案只本人读 |

| Node / agent_run_nodes | node_key、depends_on、role、phase、status、attempt、lease_owner/lease_until/job_epoch/fencing_token、next_attempt_at、iteration；(tenant,run,node_key)唯一 |
| Execution / agent_executions | execution_key、run/node、tool_version_id、arguments_hash、冻结参数、status、result_file_id、upstream_receipt；同一逻辑动作仅一个key |
| Approval / agent_approvals | owner、run/node/execution、tool_version_id、arguments_hash、source_revision、expires_at、status、decision_reason；票据绑定不可改 |
| Artifact / agent_artifacts | run/node、kind、text_content或file_id二选一、content_hash、source_revision、target_resource_id；只读建议 |
| ContextSource / agent_context_sources | source_domain/type/id/revision/hash、role、token_count、disposition、memory_parent_ids；来源证明不等于事实 |
| Event / agent_run_events | run、seq、event_name、payload、expires_at；append only，(tenant,run,seq)唯一 |

运行任务资源分支：chapter_draft/chapter_rewrite绑定chapter，outline_suggest绑定novel，worldview_suggest绑定novel；inspiration_analyze绑定已创建的analysis_id；compliance_audit绑定已创建report_id；compliance_rewrite绑定已创建rewrite_id。所有创作分支必须显式definition_id，workflow_id可省略（省略时生成一个该definition的root节点）；creator从GET /v1/agents/catalog/definitions与/catalog/workflows分页选择已发布安全目录，不读取管理端系统指令。inspiration_analyze的resource.novel_id可空/省略，ScopeAuthorizer验证本人analysis及其asset授权投影，不要求作品；显式关联novel仍须同时验证作品权限，不得用空值跳过asset归属。其它创作分支novel_id必填；所有分支source_revision绑定PrepareExecution返回的源快照版本，由ScopeAuthorizer与对应域PrepareExecution验证对象存在、归属、源版本、可执行状态，再冻结资源快照。chapter分支可传resource.selection={start_offset,end_offset,source_hash}，以Unicode码点半开区间且start<end≤源正文长度校验，源revision/hash不符409 agent.source_changed；省略表示全章。creator不能提交diagnostic任务类型；诊断专门入口仅允许probe_key=text_connectivity/structured_validation、≤128输出token、无自由提示词。

Run状态：queued→running→waiting_approval/waiting_clarification→running；任何非终态可→cancelling→cancelled；queued/running/两种waiting状态可→failed；running→succeeded。Question等待只收集Creator输入，approval等待只授权具体工具副作用；两者使用独立聚合/API/事件。终态succeeded/failed/cancelled无出边。进入任一waiting前暂停其他node并checkpoint，Run最多一个open等待令牌；单Run Question总数最多8。Question answered只恢复发问node和原run级job。

## 2. 应用服务与Port调用

| 方法 | 输入→输出 | 调用与事务 |
| --- | --- | --- |
| RunService.Create(principal,CreateRun,key) | RunView | ScopePort.Authorize；TaskResourcePort.PrepareExecution；Agent TaskPolicyVersionPort.ResolveAndValidate(tenant,version,task_kind,definition_version)，同一UoW锁定LLM策略头验证enabled/current/published并派生task_key；GatewayPort不按key重解析；同UoW创建Run/Job/nodes/sources/outbox/幂等 |

| RunService.Get/List | run或cursor页 | scope约束仓储；详情含nodes但不含隐藏推理；本人仍持有scope才可读内容 |
| RunService.Cancel(id,revision,key) | RunView | job→run锁+revision CAS；queued/等待调用Terminate，running进入cancelling并取消上下文；终态原状态返回，过期revision仍409 |
| RunService.Terminate(txctx,run_id,target,reason,proof) | RunView | worker与cancel/reject/失效命令共用；同事务终态CAS、节点/票据收敛、job完成、artifact与已结算usage receipt引用、唯一terminal事件+outbox |
| Scheduler.ClaimRun(worker,now) | JobLease | JobsPort.Claim唯一领取run级platform job，lease_epoch递增；不存在第二个节点全局队列 |
| Scheduler.StartReadyNodes(jobLease,now) | NodeLease[] | 持有效run JobLease；job→run→node锁序，验证依赖和最多4并发；冻结job_epoch并增加node fence |
| Scheduler.Heartbeat(jobLease,nodeLeases) | LeaseAck | 同事务JobsPort.Heartbeat与节点租约延长；两层任一失效立即停止外呼，不增加业务revision |
| LoopService.Advance(lease) | checkpoint | ContextAssembler→GatewayPort.Generate；工具意图Schema验证→ApprovalService或ExecutionService；Observe checkpoint事务 |
| ExecutionService.Execute(key,args) | ToolResult | RegistryPort.ResolveFrozen；执行记录claim；DB工具与记录同UoW，MCP外呼在事务外 |
| ApprovalService.Decide(id,revision,decision,hash,source,key) | ApprovalView | owner/scope/有效期检查；platform job→run→node→execution→approval锁序；CAS消费+原node ready+JobsPort.Wake或通知有效worker+outbox同事务 |
| EventService.Subscribe(run,lastSeq) | persisted Event stream | ReadEventsAfter，持久后发送；订阅中每30秒复核membership/scope，撤权关闭流 |
| ArtifactService.Read(id) | Typed artifact | ScopePort + Repository；大型文件再走FilesPort授权下载 |
| DefinitionService/WorkflowService/ToolService.Put | versioned view | validate→append版本→CAS头→审计outbox；禁止原地改版本 |
| MCPService.Put | MCPServerView | DeploymentAllowlistPort校验env_ref和endpoint，DNS/网络策略由平台SSRF组件执行 |

Definition发布固定为：POST/PUT enabled=false完整校验并追加draft版本，current_version_id指向该版本且禁止新run使用；enabled=true完整校验、能力检查后在同事务追加published不可变版本并切头。不存在进程内发布状态。返回publication_status按头enabled推导；PUT false不会破坏旧run冻结版本，但立即阻止新dispatch中被撤销的工具权限。

依赖能力由消费方定义Port。Agent ContentContextPort→content ContextService.ReadSnapshot；OutlineContextPort→outline QueryService.ReadPlan；WorldviewContextPort→worldview ContextService.ReadFacts；InspirationContextPort→inspiration AssetService.Read；CompliancePort→compliance ExecutePreparedAudit/ExecutePreparedRewrite；InspirationExecutionPort→inspiration ExecutePreparedAnalysis；LLM GatewayPort→llm GatewayService.ResolvePolicy/Generate/Embed；均由app/adapters装配。inspiration/compliance若自身调用模型，使用自己的GatewayPort，绝不回调Agent。Agent run负责调度，领域service负责领域job状态与报告，执行receipt以run/node/execution_key关联，幂等回查得到既有结果。跨域不持锁外呼、不读私表。

## 3. 动态DAG验证和调度

Manifest节点key 1..64 snake_case且唯一；role限定supervisor/structure/writer/polisher/reviewer/tool；依赖只引用同图节点。Kahn拓扑排序须覆盖全部节点；拒绝自环、重复边、悬空引用、无终点、孤立非根节点、超过64节点/128边。每节点tool_version_id/definition_version_id必须租户可用且能力被父定义授权。输入绑定只允许依赖祖先的已声明output字段，校验JSON pointer、源/目标Schema类型兼容；禁止从未完成节点读取。required节点失败导致run failed；optional节点失败标skipped并提供显式空结果，下游仅可接收nullable声明的输入。

LLM planner只能提出manifest，不直接入队。动态扩展仅在supervisor自身checkpoint且没有在途子节点时，以run revision CAS附加尚未执行的节点；不得修改已有节点、已审批参数、上游输出或超出冻结工具能力集。整体重新拓扑验证后同事务插入节点和node.updated。禁止递归workflow-as-tool；复合工作流先展开到一个图，嵌套深度上限3且展开后仍≤64。

唯一可领取队列是platform_jobs，每run固定一个job，run.platform_job_id由同事务JobsPort.Enqueue返回。job的domain=agent、job_kind=run、resource_id=run_id、execution_key=run_id字符串；必须由JobsPort验证这些绑定，Agent仓储不直接改platform表。worker先ClaimRun取得30秒JobLease，再在该run内按next_attempt_at,id选择最多4个ready节点；StartReadyNodes不是独立队列claim。所有状态事务固定锁序platform job→run→node→execution→approval，平台锁/校验经JobsPort.WithLease(txctx)完成，同一UoW共享，不跨域join。

节点每次attempt冻结job_epoch=JobLease.lease_epoch、lease_owner=worker、fencing_token递增；execution dispatch同时记录job_epoch/fence。提交checkpoint、模型delta、工具结果及worker终态时必须同时满足platform job仍leased且owner/epoch匹配且lease未过期、node.job_epoch等于job epoch、node owner/fence匹配且lease未过期。任一不符拒绝全部业务写。每10秒心跳在同事务先续job再续节点（均30秒），只改技术字段，不推进业务revision。run换worker即增加job epoch，即使旧节点fence尚未回收也不能提交。node job_epoch为历史值，不建立指向当前lease_epoch的FK。审批唤醒或崩溃恢复后，只能在job→run→node→execution锁内将从未dispatch的prepared execution重绑定到新job_epoch/node fence；execution_key、冻结参数和已消费票据保持不变。已succeeded直接回读receipt，dispatching/unknown不能用重绑定绕过结果核查。

纯等待审批时释放run job为retry_wait，available_at取最早票据expires_at；批准在同事务Wake该job到queued，不换job_id。若尚有有效worker则保留其lease，只将node置ready并通知；job锁串行化避免双worker。等待/正常checkpoint调度不计技术失败；job attempt只统计首次执行及崩溃/技术重试，审批唤醒沿用attempt，真实技术重试首次之外最多3次。扫描器每5秒经JobsPort回收过期job，新owner读取旧checkpoint：无已dispatch模型/工具可重试，transactional先回读receipt，仅列入无计费且无副作用部署白名单的read_only工具可技术重试；其他已dispatch无receipt一律unknown。节点与job attempt分别受上限控制，不因唤醒重置计数。

TAO每轮持久phase=think/act/observe及iteration。Think模型返回final_text或tool_calls（最多8条），完整严格Schema校验后才进入Act；partial JSON不得执行。Act依声明dependency串行或并行；Observe保存每项结果hash和错误摘要，完成后下一轮。15轮硬限制，13轮注入收敛指令；相同(tool_version,args_hash)连续3次且结果hash不变判agent.loop_detected；空输出连续2次失败。Observe后崩溃从checkpoint恢复，不重做已完成Act。技术重试计数独立于TAO轮次。

## 4. 执行幂等、审批与取消

execution_key为服务端随机UUID，首次意图commit时生成，之后绑定run/node/iteration/call_index且不变化；数据库唯一约束阻止重复claim。状态prepared→dispatching→succeeded/failed/unknown。read_only仅在部署注册的“无计费且无副作用”白名单内可在lease回收后技术重试；模型调用从不属于该白名单；transactional工具必须通过平台UoW将结果receipt与execution成功同提交；external工具必须人工批准且上游支持幂等key才允许自动技术重试。除上述无计费read_only白名单，任何dispatching外呼在超时/崩溃时无法证明未执行，都设unknown，禁止自动重发；有上游幂等能力也不能以更换key绕过未知结果。Gateway的llm.upstream_outcome_unknown由Agent GatewayAdapter转换为node/run agent.execution_outcome_unknown，run.failed.retryable=false，原LLM码只作脱敏审计cause；已发模型未知绝不重试。若cancel已先CAS进入cancelling，则未知结果记录execution unknown并在cancelled的error_code保留agent.execution_outcome_unknown；不得以取消抹去外部副作用的不确定性。管理员可查看脱敏receipt，不能把unknown改回prepared；人工核对后的继续需要新的run且明确说明前次不确定结果，不能伪称自动补偿。

审批status=pending→consumed/rejected/expired/cancelled，无approved中间态。approve传expected_revision/hash/source_revision并校验当前来源仍一致；冻结参数从execution记录取，不接受请求重传参数。单事务把票据consumed、node置ready（continuation=act）、run有条件恢复running并写node.updated。重复相同幂等键返回原结果；不同key对已消费票据409 agent.approval_consumed；来源变化采用committed-error：同事务将pending票据置cancelled且decision_reason=source_changed、该node failed、调用RunService.Terminate使run failed并写run.failed(code=agent.approval_source_changed,retryable=false)，再写幂等completed缓存409 agent.approval_source_changed；参数hash/工具版本不匹配409 agent.approval_binding_mismatch。reject在同事务调用RunService.Terminate迁cancelling→cancelled，run.cancelled.reason=approval_rejected（用户决定不是run.failed错误）；过期由worker调用同一Terminate使run failed（agent.approval_expired），其余未完成节点cancelled；审批到期使用PG now()而不是客户端钟。只允许发起本人。

取消先检查run revision；若running则CAS cancelling，停止领取与新事件delta，向在途Gateway/工具发context cancel。等待至已有调用deadline（最长180秒）以便记账/记录副作用，再经RunService.Terminate取消未完成节点并CAS cancelled。成功若先CAS终态，cancel返回succeeded；cancelling先赢则后续succeeded提交拒绝，仅结算usage与安全审计，不能再输出文本。只要最终失败/取消，已产出的partial artifact标is_partial=true且默认不提供一键接受。

RunService.Terminate是唯一终态写入口，worker、cancel/reject命令、审批过期及来源失效均调用它。固定锁序platform job→run→node（多节点按UUID升序）→execution→approval；节点事务先做应用层scope校验，但scope不反向取得以上锁。worker proof必须通过job epoch+node fence双校验；用户命令proof为实时owner授权+expected_revision，在job锁内使终态job失效以阻止旧worker提交。失败/拒绝收敛会取消其他在途context；返回晚到结果不能写进度，LLM结算/对账仍可独立完成，不能因失去Agent lease丢账。

Terminate同事务写run状态/revision/finished_at/error_code/terminal_reason/retryable、未完成节点/票据收敛、JobsPort.Complete(txctx,job_id,target,proof=JobLease或服务端AuthorizedCommand)、artifact、run.usage_receipt_ids（跨域只存UUID）、唯一终态event和outbox。usage本体与日桶由每attempt提前结算，Terminate通过GatewayPort.ReadUsageReceipts取可信已结算凭证后引用，禁止重复INSERT usage或跨域join。run succeeded/failed/cancelled分别对应job succeeded/failed/cancelled；AuthorizedCommand仅由已验证本人权限和revision的应用服务生成，客户端不能提交或伪造。所有终态竞争以第一个CAS为准；重复调用不再递增revision或补第二条终态。queued/waiting的取消与拒绝可在命令事务完成，running取消先cancelling后由完成/超时回收走同一Terminate。

CommandOutcome固定为Success(value)或CommittedError(code,http_status,safe_details)，与Abort(error)区分。审批来源失效必须先确认请求revision及本人授权有效、请求冻结绑定正确，再以平台当前源证明与票据冻结源比较；伪造请求hash只返回普通409，不触发源失效。实际源变化时，将失效与终态event、409幂等结果同一UoW提交，返回CommittedError而不是抛异常导致rollback。客户端收到409必须GET run/approval，旧run已经failed；同key重放同409不再消费票据。PG失败则全部回滚并503，不缓存虚假的409。

复合FK冻结数据库归属：execution(tenant,run,node)→node(tenant,run,id)，approval(tenant,run,node,execution,tool_version)→execution对应五列，artifact(tenant,run,node)→node；拒绝同租户不同run混绑。技术job_epoch/fence只验证所有权、不作为跨版本FK；续租不增加业务revision。

## 5. 上下文预算与记忆来源

TokenBudget B=min(model.context_window,60000)，输出reserve为冻结policy.max_output_tokens，协议/tool Schema成本计入input；input+outputReserve≤B。源优先级：不可裁剪system规则/用户当前指令/当前目标定位与事实约束；其次当前大纲、出场角色、当前章选区；再最近两章正文、长期摘要、灵感建议、旧工具结果。同层按相关性降序、source_id稳定排序。先去重source_id/revision/hash；再移除低优先级完整chunk（段落边界，≤2000token）；历史工具结果改为带来源hash的摘要，最后压缩历史记忆；不可裁剪段本身超限422 agent.context_budget_exceeded。不得无提示切掉用户指令、硬事实或把未确认建议当事实。

80%input预算触发压缩，ContextSummarizer经GatewayPort使用summarization任务，成本独立计入同run。压缩失败不伪造摘要：继续移除允许丢弃chunk直至满足预算，否则失败。计数优先模型tokenizer；未知tokenizer按UTF8字节数保守上界并标estimated，不按中文字符/4低估。送出前Gateway二次校验，供应商context拒绝不自动更大窗口重试。

记忆类型confirmed_fact/owner_snapshot/derived_summary/external_suggestion/tool_observation。每条记录source_domain、source_type、source_id、source_revision、content_hash、token_count、included/compressed/dropped及parent source UUIDs；derived_summary必须可追溯到owner快照，引用失效则重新读取并生成新来源，不覆盖旧来源。memory不是跨作品共享事实。禁止把外部文本中的指令升级为system；工具结果只作data段。run上下文与产物30天软删除窗口，scope删除立即拒绝，之后清正文/文件；90天保留去标识化指标；事件正文仅7天。

## 6. 工具与MCP契约

工具Schema固定Draft2020-12，输入/输出根object、additionalProperties:false，每属性有类型/限制；schema≤32KiB、嵌套深度≤16、$ref仅本Schema $defs，禁止远程ref；Ajv表单校验辅助，后端jsonschema/v6完整校验为权威。tool参数≤64KiB，结果≤256KiB，所有字符串和数组有界；注册校验Schema本身及允许能力。注册状态enabled/disabled，更新新增版本；禁用阻止新dispatch，已获批准仍需执行前重检，不以冻结版本绕过撤权。

side_effect=read_only/transactional/external；external必approval_required，且不得成为内容/事实写后门。内置工具key及调用：read_content→ContentContextPort、read_outline→OutlineContextPort、read_worldview→WorldviewContextPort、search_inspiration→InspirationContextPort、analyze_inspiration→InspirationExecutionPort、audit_text/rewrite_suggestion→CompliancePort、save_artifact→ArtifactService、plan_workflow→DAGValidator。工具定义中的model/tool version权限独立于角色名称。

MCP endpoint只允许部署审批的https scheme/host/port/path，禁重定向、localhost、metadata、私网（显式部署局域MCP allowlist例外通过精确IP与证书绑定），每次解析检查DNS结果并pin连接地址。env_ref只能匹配部署映射如INKLORE_MCP_BOOKTOOLS_TOKEN，不能引用PATH、JWT密钥或LLM密钥；API仅保存引用，SecretResolver只向指定host注入Authorization，禁止模型访问值。methods白名单如tools/list、tools/call；remote tool schema取回后仍需operator明确注册版本，远程变化不能自动扩权。MCP传出文本需本人审批显示目标host及数据类别，诊断禁止MCP。

## 8. R1.x策略版本Port与澄清Answer事务（ADR-0027）

RunService.Create的Creator命令只带task_policy_version_id。Agent `TaskPolicyCatalogPort`/`PolicyVersionPort`由app/adapters实现，调用LLM public application query/service；同一PG UoW内按tenant与策略聚合锁定头，验证enabled、published current version及task_kind/definition能力后冻结policy_version_id并从该版本读task_key。LLM GatewayPort Generate输入必须包含policy_version_id；LLM应用服务校验tenant绑定并从immutable version读取模型路由/预算，不按task_key重新取current。策略版本ID只作为跨域UUID，无LLM表外键。Creator旧task_key仅在滚动升级临时兼容命令解析，严格从当前tenant enabled catalog取值，未知拒绝；新字段与旧字段共存请求拒绝，客户端部署完关闭兼容。

TaskPolicy Creator投影字段为policy_id/policy_version_id/task_key/title/version/task_kinds/input_schema/budget。仅当前tenant已发布、enabled、未删除且能力授权的版本；输入schema由LLM应用层白名单投影，绝不透出system instruction、模型/provider/channel/key/路由、工具参数。分页opaque cursor绑定tenant/actor/query且稳定排序task_key,id。目录读失败保留表单但禁Run。

Clarification question issue/answer遵循锁序platform job→run→node→question，用户权限检查实时且不取得反向锁。Issue只允许当前运行owner同scope，写Question(open,version=1)、run/node waiting_clarification、checkpoint、job retry_wait、question.required/node.updated事件及outbox同一UoW；释放lease，保留原job_id、deadline和execution receipts。Question schema限单选、多选、自由文本，题面≤4KiB、schema≤8KiB/depth≤8、选项≤20、答案≤4KiB；locale title/help key和context摘要≤1000码点。禁止远程ref/HTML/隐藏推理/工具权限/task config/secret，Agent只从定义的可澄清字段约束生成问题。

Answer API只接受expected_revision、question_version与schema-validated answer及Idempotency-Key；服务端验证actor=run owner、scope仍有效、question/run/node/policy/definition/source revision/hash完全绑定、未过期及open。首个命令同事务记录answer/hash/actor/time、Question answered、node ready、run queued、Wake原job及question.answered/node.updated/outbox和幂等结果。相同key+请求体重放原答复，不再次Wake；同题不同键CAS失败agent.question_already_answered；source不匹配agent.question_binding_mismatch；过期agent.question_expired并收敛run.failed。答案在checkpoint中作为untrusted data，不回写Owner域，也不改变source revision/hash。


RunRepository.ListByOwner(tenant,owner,status,cursor)索引(tenant,owner,updated_at desc,id desc)；admin AuditList投影不select instruction。NodeRepository.ListReady在持有效run JobLease后使用(status,next_attempt_at,lease_until)索引；worker所有写同时验证job epoch与node fence，用户命令验证tenant/id/revision并持job锁；ApprovalRepository.ListPending索引(tenant,owner,status,expires_at)；EventRepository.ReadAfter按(tenant,run,seq)，TTL按expires_at；ArtifactRepository.List按(tenant,run,created_at,id)。cursor签名绑定tenant/actor/filter/sort，默认20最大100。审计不可返回原始SQL、正文、tool arguments或secret refs值。

设计验收操作：制造A→B→A图收到422且无run；双worker争领同node仅一个fence获准；dispatch后杀worker，external未知收到agent.execution_outcome_unknown；本人批准恢复同run且completed execution数不变；到期边界在PG now()==expires_at拒绝；同时cancel/finish仅一终态；SSE从seq=42重连第一条43，清理后410，GET不增加调用；60000预算测试以来源台账证明裁剪顺序及tokenizer标记；accept stale artifact在Owner发生409且正文不变。集成验收另含：run创建与唯一platform job要么同提交要么均不存在；job epoch变化但旧node fence未变时提交仍拒绝；审批等待唤醒保持同job_id且不消耗技术重试；实际源变收到409后run已failed并仅一terminal事件，同key重放不增revision；model unknown映射agent.execution_outcome_unknown且retryable=false；跨run节点/审批与跨provider渠道由FK拒绝；终态引用既有usage receipt不二次扣费。此清单是R1实施验收要求，不宣称已执行实现验收。
