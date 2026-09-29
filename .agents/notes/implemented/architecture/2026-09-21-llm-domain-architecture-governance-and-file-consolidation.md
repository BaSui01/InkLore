# Agent Note: LLM 域架构治理与碎片小文件合并

Status: implemented

Task: INK-185
Scope: internal/llm

## Problem

internal/llm 包含 73 个 Go 源文件，存在严重的文件碎片化与架构不规范：
1. 缺少 AGENTS.md §1.2 强制要求的 internal/llm/doc.go 职责与边界声明；
2. 顶层遗留孤立的 internal/llm/ports 包（仅 8 行代码），违背 4 层（domain/application/infrastructure/transport）标准架构；
3. infrastructure 下遗留未被使用的 persistence 子包，被 archguard 标记为不可达运行路径；
4. 存在大量极端碎片化小文件（如 think_stream_filter.go 仅 9 行，metrics.go 24 行，embeddings.go 38 行，scheduling_weight.go 31 行，concurrency_limit_error.go 34 行等）；
5. routing 内部 22 个文件极度分散，同类逻辑（如指标/监控、图片策略、规划与调度、熔断与策略注册）被割裂；
6. 存在环境变量渠道注册表 env_channel_registry.go（从 LLM_CHANNELS 解析内存渠道池）以及 MemoryProviderRegistryRepository、MemoryTaskPolicyRepository 内存仓储和硬编码种子数据，违背"模型与渠道唯一由管理员通过数据库与管理端配置"的系统设计红线。

## Decision

1. **补齐边界声明与标准 4 层结构**：
   - 补齐 internal/llm/doc.go，明确模型/渠道/Key/策略/预算唯一 Owner 及 R1 DAG 无依赖边界；
   - 移除孤立的 internal/llm/ports 包，将 HTTPStatusError 和 StreamHandler 移至 domain 与 routing 内部；
   - 消除 infrastructure/persistence 子包，将其实现合并回 infrastructure 根层，消除 archguard 不可达警告。
2. **彻底移除环境变量渠道注册表与内存仓储（仅限管理员数据库配置）**：
   - 彻底删除 internal/llm/infrastructure/env_channel_registry.go，移除 LLM_CHANNELS 环境变量解析与内存渠道构建；
   - 彻底移除 MemoryProviderRegistryRepository 与 MemoryTaskPolicyRepository 及其种子数据，杜绝一切内存旁路；
   - ProviderRouter 严格以管理员在数据库配置的渠道（SetDBResolver）为一等公民与核心权威，配合 SetPool 与环境变量直连兜底。
3. **领域与应用层高内聚整合**：
   - domain/types.go：整合生图、向量化、指标、用量审计契约、思考流分流器及 structured.go 结构化输出规范与提取；
   - domain/budget.go：整合核算、日桶预算与原子准入状态实体；
   - domain/catalog.go：整合 Provider/Model/Channel/Key/Policy/Pricing 实体与仓储 Port；
   - application/validation.go：整合输入 Schema 严格验证与输出结构化严格验证；
   - application/gateway_service.go：整合模型调度、准入准备与思考流分流；
   - application/provider_admin_service.go：整合 Provider/Model/Channel 管理服务与 task_policy_service.go 任务策略服务。
4. **调度路由层收敛**：
   - routing 收敛为 8 个高内聚模块（balancer, circuit_breaker, generation_plan, limiter, metrics_monitor, pool, route_profile 包含合并后的 image_policy, types）。
5. **基础设施与接入层收敛**：
   - infrastructure/catalog.go：整合 PostgreSQL 目录 Store 与合并后的 secrets.go SecretRef 安全绑定与校验；
   - infrastructure/provider_registry_repository.go：整合 SQLProviderRegistryRepository、SQLTaskPolicyRepository 与 UsageRecorder，单文件完整覆盖所有持久化 SQL 仓储；
   - infrastructure/budget.go：整合预留与结算原子事务；
   - infrastructure/chat.go：整合对话请求与流式解帧；
   - infrastructure/provider_router.go：整合上游目标、用量记录与嵌入外呼，移除 LLM_CHANNELS 旁路；
   - transport/http/handler.go：整合 32 个 REST 端点注册、游标列表与输入验证。

## Alternatives considered

- 保持 73 个小文件不动：每个结构体/错误类型单独建文件导致理解成本高、查找跨度大，且违反 4 层架构红线（存在 ports 与 persistence 额外层级）；
- 保留 LLM_CHANNELS 环境变量注册表：破坏了系统管理端对渠道、密钥与模型的统一数据库纳管设计，导致配置多头来源与状态漂移；
- 保留内存仓储（MemoryRepository）：内存数据随进程重启丢失，且存在非权威的硬编码种子数据，不符合生产级架构。
重构后 29 个 Go 源文件体积均在 209B~37KB 之间，内聚性大幅提升且严格满足 50KB 单文件红线。

## Consequences

- Go 源文件从 73 个收敛至 29 个，消除 44 个碎片小文件；
- 单文件体积分布在 209B - 37KB，完全遵守 <= 50KB 铁律；
- archguard 架构红线全绿，不可达包数量由 16 降至 15；
- 消除孤立 ports 包与 persistence 子包，严格符合 domain/application/infrastructure/transport 标准 4 层分工；
- 渠道、模型、密钥与任务策略完全收敛至数据库持久化与管理员统一配置，消除内存注册表与环境变量渠道旁路；
- 管理端完成开源模型配置源（基于 LiteLLM 标准）一键同步与预设填充组件，模型能力严格对齐 text/tools/structured/embedding DDL 契约；
- 沉淀后续迭代从单一读取到换入写入与阶梯费率规则（tiered context & cache creation）的平滑演进方案；
- 完成双层货币解耦机制落地：在 domain/credit_estimator.go 实现小说字数到微元成本与墨水币积分精确折算引擎，在管理端 LlmDashboardRoute 集成小说商业化与千字算力测算引擎（NovelBillingCalculatorCard），实现千字感知单价、底层云成本、平台毛利与字数包定价建议动态联动。

## Verification

1. go build ./... 与 go vet ./... 零报错；
2. go run ./cmd/archguard -reachability 架构红线全部通过；
3. git diff --check internal/llm 干净无格式错误；
4. node scripts/check-project-skills.mjs 验证通过；
5. node scripts/agent-notes.mjs check 验证通过。
