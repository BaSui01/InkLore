# Agent Note: Creator 任务策略目录缺口必须先走 R1.x 裁决

Status: proposed

Task: INK-226
Scope: docs/system-design/API冻结基线与 Creator AgentWorkbench 的 task_key / policy_version_id 选择契约

## Problem

INK-226 的 Creator 工作台已经通过 Creator 安全目录读取 Definition、Workflow 和动态模型目录，但 `task_key` 仍由作者自由输入。R1 Agent catalog 只返回定义的 `task_kinds` 与标题/版本、工作流标题/版本；Admin LLM policy 列表 audience 不允许 Creator 读取。LLM policy 有租户级 enabled/current_version_id 与不可变版本，而 Agent CreateRun 同时要求 `task_kind` 和自由格式 `task_key`；已有 schema 没有 `task_kind→policy` 绑定关系。现有 `agent_runs.policy_version_id` 也不意味着 worker 已按该版本执行，必须检查实际传参链。

已检索 ADR-0013、ADR-0016、ADR-0024 与 INK-226 已有 implemented note。ADR-0013 的固定七个历史 key 与 R1 冻结数据/API不能视作 Creator 安全动态目录；ADR-0016 的冻结规则要求缺陷按 R1.x 正式变更；INK-226 note 已将 task catalog 和事件外动态澄清列为未提供能力，本记录补充具体可审议接口和迁移范围，不更改现有 note 的事实结论。

## Proposal

仅提案：新增 Agent creator-safe `GET /v1/agents/catalog/task-policies` 返回分页安全投影 `policy_id`、`policy_version_id`、`task_key`、`title`、`version`；仅当前 tenant 已启用且发布的 policy。Agent 通过消费方 Port / app adapter 获取 LLM 目录，不能直读 LLM 私表，也不能复用 Admin API。Creator 分别选择定义支持的 `task_kind` 与策略，instruction 保持原文，不根据角色名或指令文本硬编码策略映射。

CreateRun 创作请求提议新增必填 `task_policy_version_id`，服务端从其解析 task_key、校验 tenant/启用/当前版本并持久化现有 `agent_runs.task_key` 与 `policy_version_id`；旧 `task_key` 请求只在明确定义的过渡兼容期保留，服务端仍须解析到真实已启用 policy。执行时使用冻结 policy version。提案详见 [ADR-0027](../../../../docs/decisions/0027-proposed-creator-task-policy-catalog.md)。该契约尚待用户批准，未实施。

Agent 自然语言澄清是可选且独立的 R1.x 范围：需完整设计运行暂停/继续、用户响应幂等、恢复、超时、撤权与 SSE；不得新增冻结事件名 `ask_question`，不得滥用 `approval.required`。

## Alternatives considered

- Creator 直接读取 Admin 的 task-policy API 看似复用现成目录，但其权限与返回投影均不安全，不能由前端删字段取代服务端授权。
- 静态 task-key/角色映射开发快，但不能响应租户动态发布，且是零硬编码红线禁止的假目录。
- 只增加只读目录、继续自由提交 key 可减轻输入错误，却没有消除作者手输和策略版本绑定缺口。
- 让任务指令文本自动推断 task_kind/policy 对作者最省操作，但现有数据 schema/事件/运行状态不表达确认交互，不能由前端假设或新增 SSE 事件实现。

## Acceptance criteria

- 用户明确裁决是否批准新增 Creator 安全策略目录及其响应字段、creator/tenant 权限投影。
- 用户明确裁决 CreateRun 的 `task_policy_version_id`、服务端派生 task_key、兼容旧 key 的迁移窗口。
- 批准后再更改冻结 API 源契约、生成文件、DDL、错误枚举、SDK、UI 与 R1.x 实施计划；Proposed 状态不等于获批、任务完成或运行验收。
- 将自然语言澄清明确分为独立可选范围，若纳入另行提交状态机、事件、幂等、恢复/超时/撤权方案。

## Risks

如果未经裁决先改契约、生成文件、DDL 或 UI，会绕开 R1 冻结流程；静态映射会掩盖运营目录未接通；暴露 Admin policy 响应会泄露路由治理元数据。过渡兼容期若不设 sunset 会长期保留自由 key 提交。仅校验 CreateRun 时的 policy 版本而执行阶段仍按当前 task_key 查策略，会造成 run 声称冻结版本但实际模型调用漂移。该节点不更改唯一 backlog `docs/backlog.md`，INK-226 风险与状态留待最终集成节点处理。