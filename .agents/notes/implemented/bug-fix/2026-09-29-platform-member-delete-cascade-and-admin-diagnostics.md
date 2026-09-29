# Agent Note: 租户成员级联删除外键修复与 Admin 确认弹窗错误反馈

Status: implemented

Task: INK-243
Scope: platform membership delete cascade、i18n 词条去重与 Admin ConfirmDialog 失败展示

## Problem

开发环境控制台报出多项异常：
1. Vite 500 导致 `ChannelsRoute.tsx` 与 `App.tsx` 无法热重载：`packages/i18n/src/locales/admin.ts` 存在重复的对象字面量键名（`admin.protocols.anthropic`、`admin.protocols.gemini`、`admin.protocols.ollama`），导致 TypeScript 在 strict 模式下报 `TS1117` 致命语法错误，阻断了整个前端的编译。
2. 真实 `DELETE /v1/platform/members/:member_id` 报 500 内部错误：PG 真实查询显示目标成员具有 14 个 `platform_sessions` 与 33 个 `platform_refresh_tokens`；既有 `membership.Service.Delete` 级联删除 `platform_sessions` 时，因未先清理引用 session 的 `platform_refresh_tokens`（存在自引用 `replaced_by`）与 `platform_ws_tickets`，触发外键约束违规拒绝并抛出 500。
3. 真实 `DELETE /v1/agents/definitions/:id?expected_revision=1` 报 409 Conflict：PG 查证 `novel_architect` 的首版正在被一条处于 `processing` 状态的 `agent_novel_creation_sessions` 引用，后端按 INK-242 契约正确拒绝物理删除并返回 `agent.resource_in_use`；但 `AgentGovernanceRoute.tsx` 异常分支未在 `ConfirmDialog` 中渲染具体错误，导致弹窗不关闭且遮蔽顶层通知，给用户造成“卡死”体感。

## Decision

1. **后端成员级联删除闭环**：在 `internal/platform/membership/write.go` 的 `Service.Delete` 事务中，删除会话前先解除 `platform_refresh_tokens` 的 `replaced_by` 自依赖，并级联清理 `platform_refresh_tokens` 与 `platform_ws_tickets`，再清理 `platform_sessions` 与 `platform_memberships`。
2. **国际化重复键消除**：清理 `packages/i18n/src/locales/admin.ts` 中中英文词典内的重复键声明，确保 TS 类型检查与 Vite 编译 0 错误。
3. **管理端删除失败人机交互闭环**：在 `AgentGovernanceRoute.tsx` 与 `UserManagementRoute.tsx` 的 `ConfirmDialog` 中集成 `deleteError` 渲染和 `safeErrorText` 契约转换，当遇到 409 `agent.resource_in_use` 时，在弹窗内直接清晰提示“该智能体配置已有运行、引导会话或历史节点引用，不能删除；可停用以阻止新任务使用”，杜绝无反馈卡顿。

## Alternatives considered

- 仅靠前端禁用删除按钮：无法防范直接经由 API 或自动化脚本发起的非法物理删除，必须由数据库事务与外键级联共同约束。
- 静默关闭 ConfirmDialog 仅显示全局通知：会因为操作员视线聚焦在弹窗内部而漏看顶层轻量 Toast，弹窗内就地展示错误指引是最佳人机交互。

## Consequences

租户成员可正常安全删除其名下会话与令牌历史（仅限非本人、非最后一个 operator）；前端 Vite 彻底恢复稳定热更；对于存在引用保护的智能体定义，系统既坚守了不可物理破坏运行事实的业务硬边界，又给予了操作员精准的业务指引。

## Verification

- `frontend` 运行 `pnpm run build`（涵盖 `build:creator` 与 `build:admin`），全部 0 错误一次性成功生成 production bundles。
- 后端运行 `go build ./...`，编译全部通过。
- 运行 `go run ./cmd/i18ncheck`，中英 905 条基准 100% 一致。
- 运行 `go run ./cmd/archguard -reachability`，5 项架构红线 100% 通过。
