> 历史记录：本文件保留迁移前的设计或实施证据，不是当前工程规范，也不证明当前功能已验收。当前规则见 [AGENTS](../../../../AGENTS.md)；实现以源码和本轮验证为准。

# LLM Gateway R1 冻结设计

状态：INK-141 冻结目标，已完成多协议接入升级。遵守 [基线](00-CONTRACT-BASELINE.md)。各主流协议官方规范见 [LLM供应商官方协议规范](../../../../docs/llm-provider-specifications.md)，细则见 [详细设计](domains/09-DOMAIN-LLM-GATEWAY-DETAIL-SPEC.md)，DDL见 [llm.sql](../../../../docs/system-design/data/llm.sql)，接口见 [配置](../../../../docs/system-design/api/llm-admin.json)、[策略与预算](../../../../docs/system-design/api/llm-policy.json)、[类型](../../../../docs/system-design/api/llm-schemas.json)。

## 1. 唯一职责与公开边界

LLM是Provider/Model/Channel/Key元数据、任务策略版本、调度、预算、定价与用量审计的唯一Owner；不依赖任何业务域，不认识小说/章节。agent、inspiration、compliance各自定义GatewayPort，通过app/adapters调用本域GatewayService。公共UI生成统一创建AgentRun，不公开/v1/llm/generate/chat/completion等重复接口；operator诊断也创建受控Agent diagnostic run。admin operator管理配置；admin auditor/operator读配置元数据和脱敏usage；creator无LLM管理权限。

| 用例 | 输入和方法 | 输出 |
| --- | --- | --- |
| LM-01 Provider管理 | Create/Put/DeleteProvider；protocol、code、title、enabled | immutable protocol；在用对象禁删 |
| LM-02 模型目录 | Create/Put/DeleteModel；provider、upstream_name、context_window、max_output_tokens、capabilities | 明确text/tools/structured/embedding；embedding固定1536维 |
| LM-03 渠道与Key | Create/Put/DeleteChannel/Key；endpoint、model、priority、weight、RPM、concurrency、SecretRef | 仅元数据；无密钥明文输入输出 |
| LM-04 策略发布 | Create/Put/DeletePolicy；task_key、primary/fallback model、output/timeout/retry、required capabilities | PUT新增不可变版本且原子切头；run冻结version_id |
| LM-05 定价维护 | CreatePricing、ListPricing | immutable价格修订；CNY整数微元/百万token；旧usage不重算 |
| LM-06 预算 | GetBudget/PutBudget | 租户北京时间日额度；revision CAS；不能降至已承诺以下 |
| LM-07 审计 | ListUsage/GetUsage | invocation/attempt、token、费用、估算标记、pricing revision、状态；无prompt |
| LM-08 内部模型调用 | ResolvePolicy/Generate/Embed/Cancel | typed内部事件和唯一final result；Agent持久化为公共SSE |

Provider protocol=openai/deepseek/anthropic/gemini/ollama；schema与实际驱动能力不匹配422 llm.capability_mismatch，不静默去掉tools/JSON约束。上下文≤min(model.window,60000)，输出1..16384且小于窗口。权重1..1000，priority 0..100（小者优先），RPM1..100000，并发1..1000，timeout1..180秒，最多3次技术重试。fallback≤3个无重复模型且都满足任务能力/安全策略；不允许请求方任意覆盖model与SecretRef。

## 2. 调度与失败收敛

先授权和冻结策略/价格；PG事务预留预算；Redis Lua原子选择最低可用priority级的平滑加权轮询候选，同时检查渠道与key的60秒滑动RPM、30秒并发租约和熔断。无可用容量429 llm.capacity_exceeded；Redis不可用503 llm.capacity_unavailable，释放未dispatch预留；严禁内存限流降级。网络外呼不持PG事务。

熔断按key维护closed/open/half_open：连续5次可归因上游失败开路30秒；半开只准一个probe，成功关闭，失败指数冷却至300秒。401立即将Key health置invalid并停用，operator更新SecretRef版本并启用后再probe；429按Retry-After封禁1..300秒，不把认证错误当余额不足。每次重试和fallback都是新attempt并单独预留/计费；已经输出delta不自动切模型拼接文本，上游中断产生唯一failed终态。已发送模型未知结果绝不自动重发；Gateway保留llm.upstream_outcome_unknown，Agent适配为agent.execution_outcome_unknown且retryable=false。Agent将内部错误转成run.failed SSE；绝无先成功后错误、[DONE]双终态、隐藏推理泄漏。

## 3. 预算与审计

金额用CNY微元int64；单价为每百万token的微元，按输入/输出/缓存命中分项ceil计算。调用前按输入保守估算+最大输出预留；不支持可靠tokenizer时按UTF8字节上界，标estimated。同租户同北京时间business_date的spent+reserved不得超过limit。首次预留即固定日桶，跨午夜完成仍结算原桶；重试在其dispatch时间新日桶预留，报告含各attempt的日期。上游已dispatch但最终usage缺失，按该attempt最大预留金额保守计费并标estimated；后续可信对账采用追加adjustment，不篡改原usage。

用量记录按attempt不可变，保存策略版本、模型/渠道/key UUID、pricing_revision、input/output/cache tokens、cost、estimate reason、错误code和trace_id；不保存提示词、响应正文与密钥。无usage的取消/超时仍计已消耗费用，预算不是成功才扣。崩溃回收器由持久reservation dispatch标记区分可释放与保守结算，避免永久占款和免费重试。每attempt独立结算后返回usage receipt；Agent终态事务只引用已结算凭证，不重复扣费。Channel的tenant/provider/model一致性由复合FK保证。日额度是内部治理，不属于支付订阅。

## 4. SecretRef与严格结构化输出

SecretRef.kind仅env/secret_store，locator受部署allowlist约束；operator可选择引用，无权在API提交raw secret。SecretResolver→平台环境/SecretStore适配，仅调用前解引用，日志只key_id和不可逆HMAC fingerprint，响应不显示原密钥尾部。Provider endpoint受SSRF allowlist和TLS校验；Ollama局域地址须显式部署授权，不开放任意代理。

structured模式Schema固定Draft2020-12，≤32KiB、深度≤16、禁止外部$ref，validator执行完整JSON Schema；必须单个完整JSON值，无前后Markdown/解释、重复键、NaN/Infinity、尾随内容；UTF8有效。JSON parse通过不等于schema通过；required/additionalProperties/enum/format/oneOf/数值边界全部验证。供应商native JSON模式只是优化，后端二次验证是权威。不自动用正则修复或剥代码围栏。失败llm.structured_output_invalid，attempt计费，不产生可接受artifact。

## 5. R1验收

并发100请求时渠道/key双层限额仍成立；Redis断开AI请求503但非AI CRUD正常。priority不同不混轮询，相同权重统计选择比例正确；首个delta后故障只失败不fallback。午夜前预留次日完成归前日，跨日retry归新日；PG reservation与usage唯一键确保重放不双扣。Secret值无法从API、SQL、日志、事件或失败details获取。新增价格仅影响后续attempt，旧usage保留版本；结构化输出违背oneOf/format/额外属性时拒绝。上述是实施验收，不是当前实现审计。
