# Agent Note: Agent Worker 与受控工具执行器真实接线

Status: implemented

Task: INK-246
Scope: internal/app Agent Worker 装配、internal/agent 工具冻结版本执行及 Admin 工具状态

## Problem

[前一阶段的真实性笔记](../bug-fix/2026-09-29-agent-safe-retirement-and-tool-truthfulness.md)确认，单纯登记工具并不等于项目有可调用的 Agent Tools：组合根既未启动 Scheduler，也没有向执行端口注册处理器，Admin 只能如实显示不可用。旧 `tool/infrastructure` 包含会臆造小说实体、合规结论或网络检索结果的工具，不能因为代码存在就标成可用。用户要求实际接通，而非保留“以后再做”的空态。

## Decision

组合根现向 Scheduler 和 Admin 工具状态投影注入同一 `ToolExecutor`，在路由与依赖校验成功后启动独立后台 Worker；关闭 App 时先停止 Worker。PG 作业按 lease owner/epoch 领取、心跳与终结；后台终态与平台 job 在事务内一致提交。Scheduler 在 Think 时只暴露定义冻结版本里受控且可执行的工具，模型返回的 key/version 必须精确命中；Act 按租户与冻结版本重新验证 Schema、启用状态、副作用与执行器，Observe 将真实结果作为不可信数据加入下一轮模型上下文，不以“已登记”冒充成功。工具结果不进入 SSE 正文。

当前只登记一项真实无外部写入的只读能力 `platform_utc_now`，处理器读取当前 UTC 时间并按版本输入/输出 Schema 验证；MCP、网络搜索和旧启发式小说分析均未接入。`admin seed-tool -tenant-id` 可独立在 GORM 事务中只为指定租户登记该能力，已软删/禁用或 Schema 不兼容则拒绝复活。管理页只在当前运行进程已装配 Worker 且租户当前版本匹配时标为 `available`，否则 `not_bound`。该冻结响应增量仍见 [ADR-0034（Proposed）](../../../../docs/decisions/0034-agent-tool-execution-status-and-safe-retirement.md)。

## Alternatives considered

把旧工具文件无条件扫出来虽然很快形成丰富列表，但包括返回凭空人物关系、合规通过和假网络兜底等不可信结果；仅将 `execution_status` 常量改为 `available` 不会创建运行处理器；开放管理员直接执行任意工具的 REST 端点还会绕过作者任务资源授权与审批。因此先接最小受控的真实只读能力，由 Worker 的既有 Run 通道触发，其他工具保留不可用状态。

## Consequences

当前工作台和开发租户已各有一条工具登记配置，现运行后端返回真实 `available`；原有定义不会因播种被静默升版或绑定，必须在管理页明确发布带工具版本的新定义，旧 Run 仍保持冻结快照。新 Worker 将领取数据库中该域排队任务，未知结果恢复与审批竞态需要后续真实故障注入。`available` 仅证明该租户配置具有处理器和符合条件，不证明模型供应商实际选择了工具，也不意味着任意登记工具或 MCP 已支持。

## Verification

`go build ./...`、Admin 生产构建、请求 Schema、i18n、架构护栏与变更 Go/TS 文件 LSP 均通过；`inklore_r1` 两个指定租户经 GORM CLI 分别登记工具，DBX 只读确认开发租户版本为 1、启用、只读且不需审批。独立 39080 后端 `/readyz` 返回 200；现有管理员会话查询 `GET /v1/agents/tools` 返回 200，当前租户工具的 `execution_status=available`；新进程接管工作台 29080 后浏览器实际显示一个可执行工具卡片。独立验收进程已关闭。尚未取得模型实际选中工具、后台调用处理器与 SSE/产物的端到端证据；不得用上述目录证据冒充完整 Agent Run 成功。