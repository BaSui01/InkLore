# Agent Note: Agent 逐轮引导创建小说

Status: proposed

Task: INK-236
Scope: internal/agent/application/novel_creation.go、Agent guide HTTP/API、Creator 首页开书对话框

## Problem

旧开书对话框一次性直接调用 Content 创建作品，没有真实 Agent 访谈、作者选择建议或跨刷新恢复。通用 AgentRun 依赖既有资源，不能用于尚未创建的 Novel；应由 ADR-0032 的独立 Agent Guide 聚合承载预创建对话。

## Proposal

用独立持久化 session 在 agent 域调用当前租户已发布的 `novel_architect` 定义与 task policy。模型只能请求固定 `ask_user` 或 `propose_novel` 工具，作者答案作为不可信输入保存。提案可编辑，但作品只在显式确认后交给 Content 创建。Creator 按 tenant/user/audience 隔离暂存 session id；只有启动请求尚未返回时，额外暂存灵感文本与 UUID 幂等键，以便刷新后安全重取该请求，之后所有恢复仍通过 owner API。

## Alternatives considered

- 静态表单：没有满足由 Agent 主动提问并提供建议的需求。
- 伪造未创建的 Novel 资源并复用通用 Run：破坏 Run 的资源与授权快照约束。
- 模型直接写入作品：绕过 Content owner 和作者确认边界。

## Acceptance criteria

- 已落地 Agent guide API、固定工具参数白名单、Creator 多轮问答/建议选择、提案编辑及显式 Content 确认。
- 必须在隔离 PostgreSQL 应用迁移后验证会话持久化、刷新恢复、租户与 owner 隔离、不同答案幂等键 CAS、模型失败和最多八轮。
- 使用真实启用的模型配置验证 Agent 返回工具调用；确认只创建一部作品，未确认不创建。

## Risks

尚未应用迁移或验证真实 Provider。策略版本与定义提示词需在对话间保持冻结；上游请求失败时不得生成静态替代问题。LocalStorage 只保存不可直接授权的 session id，所有恢复仍需服务端身份校验。
