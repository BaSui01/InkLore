# LLM Gateway 详细设计 R1

冻结目标，INK-141。依据 [基线](../00-CONTRACT-BASELINE.md)、[域规范](../09-DOMAIN-LLM-GATEWAY-SPEC.md)。SQL [llm.sql](../data/llm.sql)，机器契约 [配置](../api/llm-admin.json)、[策略](../api/llm-policy.json)、[类型](../api/llm-schemas.json)。

## 1. 数据聚合与状态

每表UUIDv4、tenant FK、UTC timestamps；可变头revision正整数。引用其他业务域仅opaque UUID。llm不join Agent表，不推断调用者内容。

| 实体/表 | 字段与状态 |
| --- | --- |
| Provider / llm_providers | code/title/protocol/enabled/revision；protocol创建后不可变；enabled→disabled拒绝新调用 |
| Model / llm_models | provider_id/upstream_name/capabilities/context_window/max_output_tokens/embedding_dimensions/enabled；能力更改必须与驱动验证一致 |
| Channel / llm_channels | provider/model、base_url、priority、weight、rpm、concurrency、enabled；一个渠道绑定一个模型，(tenant_id,provider_id,model_id)复合FK强制模型属于同一provider，提供明确upstream模型映射 |
| KeyMetadata / llm_keys | channel、title、secret_kind/locator、fingerprint、secret_version、weight/rpm/concurrency、enabled；无secret列 |
| TaskPolicy / llm_task_policies | task_key/title/current_version_id/enabled/revision；无默认环境模型后门 |
| TaskPolicyVersion / llm_task_policy_versions | version、task_kinds（1..16；旧版本upgrade回填空集合且不进入Creator目录）、primary_model_id、fallback_model_ids、max_output_tokens、temperature、timeout_seconds、max_retries、required_capabilities、output_mode；immutable，旧版本无 task_kind mapping 时保留空数组并不进入Creator目录 |
| PricingRevision / llm_pricing_revisions | model_id、version、input/output/cache price、effective_at、currency=CNY；immutable，时间不回溯 |
| Budget / llm_budgets | daily_limit_micros、revision；每tenant一行 |
| BudgetDay / llm_budget_days | business_date、limit_snapshot_micros、reserved/spent_micros、revision；每tenant/date唯一 |
| Reservation / llm_reservations | invocation_id、attempt、business_date、model/key/pricing UUID、reserved_micros、dispatch_at、lease_until、status；reserved→dispatched→settled，reserved→released |
| Usage / llm_usage_records | invocation/attempt、reservation、policy/model/channel/key/pricing、token/cost/estimate、status/error/trace；immutable |
| UsageAdjustment / llm_usage_adjustments | usage_id、delta_micros、reason、evidence_hash、actor；只追加，更新BudgetDay余额同事务 |

删除均逻辑停用并设置deleted_at；被有效策略/渠道引用返回409 llm.resource_in_use，必须先重新绑定再删除。不可变版本/usage不删除业务记录；审计90天后去标识化actor与trace，不保留prompt。配置并非全球共享，所有唯一code包含tenant。

## 2. 应用服务与调用契约

GatewayPort由每个消费域声明，app适配调用GatewayService。内部输入：Invocation{invocation_id UUID,tenant_id,actor_id,task_key,policy_version_id,messages[],tools[],output_schema?,max_cost_micros?,trace_id}；Message{role system/user/assistant/tool,text,tool_call_id?}，最多128条、合计≤2MiB，不能带provider key。消费域负责业务上下文；Gateway再次检查总token、能力和policy版本授权。输出GenerateResult{status,text?,structured_json?,embedding?,usage[],error_code?}；stream callbacks TextDelta/ReasoningSummary/ToolCallDelta/Usage/Terminal。只有一个Terminal，Terminal之后所有chunk丢弃。ReasoningSummary只接受可披露摘要，供应商隐藏thinking/redacted_thinking不转发。

| 服务方法 | 事务、Port及结果 |
| --- | --- |
| CatalogService.Create/Put/DeleteProvider/Model/Channel/Key | ScopeAuthorizer admin/operator；SecretRefPolicy校验；Repository CAS+platform幂等+审计outbox同UoW；返回typed元数据 |
| CatalogService.List/Get | admin operator/auditor；tenant过滤；key返回引用和指纹，绝无明文；cursor绑定过滤 |
| PolicyService.Publish | 验证所有模型capability、fallback无重复；锁policy头，追加version+切current version+outbox同事务 |
| PricingService.Create | model归属校验；effective_at≥PG now；按model锁分配version；同一effective_at唯一 |
| BudgetService.Configure | CAS预算头并锁当日桶；新limit≥spent+reserved，写limit_snapshot；未来日按最新配置创建 |
| GatewayService.ResolvePolicy | 冻结policy_version与候选model能力；disabled配置使新run失败 |
| GatewayService.Generate/Embed | Reserve→AcquireRedis→MarkDispatch→SecretResolve→Driver→Validate→Settle；返回内部结果，无公开HTTP生成 |
| UsageService.List/Get | 脱敏统计或详情；过滤date/model/channel/status；无prompt选择字段 |
| GatewayService.ReadUsageReceipts(tenant,invocation_ids) | 返回已settled的usage UUID/attempt/cost/estimated；只读并校验调用主体；消费方终态引用凭证，不重复结算 |
| Reconciler.RecoverReservations | 找过期lease；reserved未dispatch释放，dispatched无结果保守结算，存在usage直接幂等回收 |

Platform复用：Gateway的ClockPort→timeutil，UnitOfWork→PG事务、SecretResolverPort→platform crypto/env/SecretStore，AdmissionPort→platform Redis客户端，AuditOutboxPort→platform outbox，ConfigAllowlistPort→部署配置，ScopeAuthorizer→platform授权。驱动实现位于llm infrastructure；llm domain不导入SDK。provider驱动归一化HTTP/流格式，绝不含业务提示词模板。

新租户创建的app编排事务调用BudgetService.Provision(tenant)，初始化daily_limit_micros=0、revision=1。GET budget只查询、不隐式创建；operator首次PUT带expected_revision=1配置额度。缺失初始化被视作配置损坏而非无限额度。平台不依赖llm，由app编排调用两者。

## 3. 选路、容量和熔断算法

1. 解析冻结策略候选模型primary+fallback（≤3），剔除disabled/deleted/能力不符/endpoint未授权的模型、channel、key；能力是text、tools、structured、embedding集合，embedding要求1536维，text输出不伪造embedding。
2. 候选按priority升序，最低存在可准入候选的priority组内SWRR；渠道权重和key权重分别轮询。Redis Lua以tenant为cluster hash tag，将选择状态、渠道/Key RPM与并发租约在同slot原子计算；不得先发请求再扣配额。
3. RPM使用ZSET精确60秒滑动窗口，score=Redis TIME毫秒，member=invocation/attempt；先清过期再检查两层数量。并发lease ZSET 30秒，调用每10秒续租并检查token ownership，结束释放；过期占用清理与新claim同Lua。队列不在Redis；容量不足立即429，由Agent PG节点1/2/4秒有限重试，且无外呼不计usage。
4. 每渠道/Key连续5个502/503/504或网络错误开路30秒；closed→open→half_open，半开仅1个租约探测。成功归零closed；失败冷却翻倍至300秒。429按Retry-After 1..300秒隔离且计失败审计，但不纳入5次服务故障；401/403凭证故障即标invalid，禁自动轮转该key恢复。取消、不合法用户参数、JSONschema失败不计供应商健康故障。
5. Redis失效，未dispatch释放PG预留并503；已dispatch调用停止新尝试，已有lease续租失败取消上游并正常结算。Redis恢复后设置30秒admission隔离，等待旧lease最长期，PG in-flight投影确认无未过期dispatch lease再开放；不以空Redis当无限容量。已失联但上游不可取消的调用仍由timeout约束，隔离到最迟调用deadline确保真实并发不超额。
6. MaxRetries=0..3为一次invocation首次之外总重试上限，不能每fallback重置。仅在未输出可见delta且上游明确拒绝/连接前失败时自动重试；已发送模型请求但响应不确定，一律不自动重发（不以幂等支持或read_only名义重发），返回llm.upstream_outcome_unknown并保守结算；消费方Agent GatewayAdapter映射为agent.execution_outcome_unknown且run.failed.retryable=false，内部原码只作审计cause。首delta后绝不换模型拼接；流中失败交给Agent记录run.failed，用户可新建run。

SWRR权重不是费用优先，也不是公平保证；每个attempt冻结实际模型、channel、key、policy/pricing版本。配置更新outbox使缓存失效，但dispatch前重读enabled/revision；撤销立即阻止新调用，已有dispatch完成仅用于结算。

## 4. 预算预留、日界与结算

额度CNY微元BIGINT，最大9e15；价格每百万token微元0..1e12，服务端用128位/任意精度中间乘积，ceil后检查int64范围。cost=ceil(non_cached_input*input_rate/1e6)+ceil(cached_input*cache_rate/1e6)+ceil(output*output_rate/1e6)；cached_input≤total_input。供应商不区分缓存则全部按input，cache=0且estimate_reason仅在usage本身估算时设置。

Reserve：先按attempt开始时Asia/Shanghai算business_date，锁budget头→当日bucket→reservation；不存在桶则insert on conflict再lock，limit快照取当前budget。以最坏输入计费（缓存命中不预先折扣）+max_output计价计算R，要求spent+reserved+R≤limit；增加reserved，创建reservation(status reserved，lease_until=now+30s)，唯一(tenant,invocation,attempt)。调用成本上限更低时同时检查。缺预算配置429 llm.budget_not_configured，不默认为无限。

Redis准入后，短PG事务MarkDispatch写dispatched_at、lease、实际渠道/key；提交后外呼。两者间崩溃视作可能dispatch，用最大预留保守收费，宁可标估算而不漏记费用。每10秒续租PG reservation到调用deadline+30秒；不可用即取消上游。取消/超时均进入结算。

Settle：锁当日bucket→reservation，若已settled/released重放既有usage。可信供应商usage按实际计费A，否则A=R且estimated=true（missing_usage/disconnected/crash/invalid_usage）。同事务reserved-=R、spent+=A、INSERT usage、reservation settled、audit outbox。真实A>R时仍如实记账允许over_budget标记，阻止新admission并告警，不丢费用、不强行截为R；供应商usage异常例如负值/超cap判invalid_usage采用R。token未知采用计数估算并标记。

00:00不搬迁旧预留，旧attempt无论何时完成都结算原日；次日重试创建次日新reservation。预算PUT同步当日limit且不得低于已承诺；过往日不修改limit。价格选择attempt预留时effective_at≤now的最大version，费用记录固化version与单价快照；无有效价格422 llm.pricing_missing。新增价格不能回溯改变既有reservation/usage。

可信后续对账由内部SettlementAdjustmentService持evidence_hash提交，去重(tenant,usage,evidence_hash)，追加负/正delta并原日spent同步变更；不能改原usage。负delta不能令总cost<0；operator不能凭UI手填任意退款。每5秒reconciler处理过期reservation：reserved释放，dispatched查询provider receipt（驱动支持时）或R估算结算。PG失败保留未结算记录，拒绝发送Terminal success直到usage事务成功；重试结算不重发模型。每attempt结算事务独立于Agent终态；Gateway输出usage receipt IDs，Agent Terminate仅引用这些已结算凭证，不跨域重新写usage。Agent job lease失效不阻止LLM依法结算，晚到费用只能作usage/对账审计，不向终态run追加进度事件。

## 5. Secret与Schema严格校验

SecretRef={kind:env|secret_store,locator,version}，locator≤256字符、仅已授权mapping；env格式大写变量名，secret_store固定部署namespace内别名，不接受任意路径。写Key只存引用、HMAC fingerprint（服务端计算，独立部署pepper）、fingerprint缺值为null并enabled=false；创建/启用前SecretResolver验证存在与目标provider绑定。读取只元数据；不输出key前后缀，不通过错误描述探测任意secret是否存在。轮换PUT增加secret_version并重置invalid，历史usage按旧版本指纹审计，旧值仅进程短时内存且不缓存到Redis。

endpoint URL只HTTPS；Ollama精确部署allowlist可HTTP局域地址；禁止user-info、query凭证、fragment、重定向和SSRF。启动加载allowlist，数据库配置不能扩大allowlist。Secret只发往被绑定host，TLS校验不能禁用。管理审计包含字段名与前后安全元数据，不记录密钥解析结果。

output_schema完整编译Draft2020-12；根object/array按任务定义，≤32KiB、深度16、$defs内部引用，无网络ref；启用format assertion，递归预算与regex执行时间限额，拒绝不受支持的关键字（禁止悄悄忽略）。流式JSON缓冲≤2MiB，UTF8/parser禁止重复属性与尾随内容，解析完成后验证全部schema，成功前不发布structured artifact。ToolCall累加按call_id，单调用64KiB，finish后严格输入Schema校验再交Agent，任何不完整调用整体失败。embedding返回数组长度1536且全部有限number；维度错误502 llm.upstream_protocol_error。结构化失败最多由新的Agent Think轮次明确请求修正，不在Gateway偷偷改数据，仍消费预算。

## 6. 查询、错误及验收

索引：各配置(tenant,updated_at desc,id desc)，key(channel,enabled)，channel(model,enabled,priority)，policy(task_key)唯一，pricing(model,effective_at desc,version desc)，usage(tenant,created_at desc,id desc)/(tenant,model,created_at)，reservation(status,lease_until)，budget_day(tenant,business_date)唯一。List limit20/1..100，cursor绑定租户角色与所有filters；统计按北京时间日期转换半开UTC范围，拒绝>90天窗口。

错误：llm.version_conflict409、resource_in_use409、capability_mismatch422、pricing_missing422、structured_output_invalid502、upstream_protocol_error502、upstream_timeout504、upstream_outcome_unknown502、capacity_exceeded429、budget_exceeded429、capacity_unavailable503、secret_unavailable503、policy_unavailable503。error details只safe字段；公开管理操作HTTP responses完整类型化，内部流错误由Agent terminal接管。

实施验收：创建缺失secret引用失败且不探测非allowlist环境变量；同键配置写重放revision不递增；不支持tools模型绑定工具策略422；并发预留只准总额度内请求，释放/重结算不双扣；日界前后attempt分别归日；模拟上游无usage保守计价有estimated标识；已dispatch进程崩溃最终回收预留；同一价格修订固定重算结果一致；首delta后故障只一个terminal failed；strict schema对required/oneOf/format/additionalProperties与重复键全部拒绝；Redis停机AI failclosed，非AI管理CRUD继续可用。
