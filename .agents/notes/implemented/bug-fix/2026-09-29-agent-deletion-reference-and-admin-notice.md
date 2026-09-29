# Agent Note: 智能体定义历史引用删除与 Admin 通知层级

Status: implemented

Task: INK-242
Scope: agent definition deletion guard 与 frontend/apps/admin 主要管理页反馈

## Problem

浏览器真实 `DELETE /v1/agents/definitions/:id` 返回500，页面把失败放在滚动内容区，确认弹窗遮住提示。只读PG确认 `novel_architect` 的 Run/Node 引用为0但一条引导创建会话引用其版本；既有 `IsDefinitionInUse` 只查 Run，版本删除触发外键拒绝。参考 [Gin 入口笔记](../simplification/2026-09-20-gin-transport-security.md) 中重放授权与危险操作约束，以及 [传输错误笔记](./2026-09-23-transport-safe-error-propagation.md) 的不吞真实错误原则。

## Decision

定义的每个历史版本同时检查 `agent_runs`、`agent_run_nodes` 和 `agent_novel_creation_sessions` 的直接外键；SQL查询失败向上传播，删除事务内再次核对已引用状态，返回公开的 `agent.resource_in_use` 409 而不级联清理历史。页面级读/操作通知使用 Admin 范围的 `AdminNotice` Portal 到 body，右上角 z=100，高于 ConfirmDialog z=50；读取失败可重新加载，删除失败保留确认框但不会自动再次发 DELETE；字段校验仍留表单，中英提示明确停用替代删除。

## Alternatives considered

强制删除会话/Run或隐式取消运行破坏权威历史；捕获所有PG错误统一映射409会把真正依赖故障伪装成业务冲突；单纯给页面内联条加z-index无法跳出滚动裁切/父级层叠上下文。均不采用。

## Consequences

已有历史引用的配置不可物理删除但可以按既有CAS停用；成功/失败反馈不再挤占管理页面布局。读失败保持已有快照，危险操作需要用户再次确认。预检与事务内复核之间仍可能有并发新增引用，最终FK保护仍在但该极端竞态的公开码未运行验证；浏览器Portal验证使用模拟503，不代表真实删除已通过。

## Verification

1. 真实 PG 与后端实测：目标受保护定义 `novel_architect` 被引导会话引用时，Admin 界面执行删除触发 409 `agent.resource_in_use`，弹窗与右上角通知合规友好提示，不再抛出 500 `platform.internal_error`。
2. 真实无引用自定义智能体 CRUD 全流程端到端验收：在 Admin 管理端手动创建自定义智能体“玄幻大纲推演大师 (V2精炼版)”（配置 3 大合法任务类型与提示词，版本 1 呈现并显示“自定义”徽章）；通过编辑追加“章节起草”任务类型保存成功升为“版本 2”；点击删除并在确认弹窗中执行“确认删除”，成功完成真实物理删除，智能体列表由 4 条平滑刷新至 3 条。
3. 构建与代码护栏验证：修复 `LlmDashboardRoute.tsx` 闭合标签与变量；`go build ./cmd/server`、`go vet ./...`、`pnpm --dir frontend run build:admin`（tsc -b && vite build）、`go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts`、`go run ./cmd/archguard -reachability` 均退出 0；未新建测试代码。看板状态更新为 done。
