# Agent Note: Agent 内置工具不可由管理员配置

Status: implemented

Task: INK-247
Scope: internal/agent 工具只读目录、对外写路由及 Admin 智能体工具页面

## Problem

[INK-246 的 Worker 接线笔记](./2026-09-29-agent-worker-real-tool-execution.md)实现真实执行器和租户版本安全检查，但管理工具页仍暴露“已登记配置”及创建、修改、停用和删除操作，使代码中固化的工具被误解为租户可任意定义。仅隐藏按钮仍留 POST/PUT/DELETE API 可绕过。用户要求展示的只能是项目代码里实际装配的内置 Agent Tools，不能配置工具。冻结 R1 的 A10 原本包含 Tool CRUD，差异另见 [ADR-0035（Proposed）](../../../../docs/decisions/0035-agent-builtin-readonly-tool-catalog.md)。

## Decision

`ToolExecutor` 自身管理不可被前端任意声明的 handler 元数据；Admin `GET /v1/agents/builtin-tools` 经鉴权读取该注册表，按当前租户内部冻结版本与处理器合同得出 `available` 或 `not_bound`，工具卡片只展示名称、描述、状态，无英文机器码小字。移除前端工具配置表单、配置分区及新建/编辑/启停/删除按钮；同时物理移除对外三条工具写路由和机读 API 操作，已知路径的写方法走统一405。保留旧 GET 供已冻结版本查询；内部 GORM seed-tool 只注册代码允许的能力，不能从 API 上传任意工具。智能体定义与工作流治理照旧，历史绑定版本编辑保留原值。

## Alternatives considered

单改 UI 隐藏工具控件最省工作量，但 API 依旧能写新配置；把数据库工具列表标记为“只读”没有保证它们确实有代码处理器；删除历史工具版本又会破坏已有 Run 和定义绑定。故选择独立编译期目录加写路由关闭。

## Consequences

新增工具必须先新增可信 Go handler，再由部署侧建立不可变内部版本；有代码实现但未在当前租户播种时仍只读展示，状态必须是未接通，不伪称可执行。旧 GET /tools 暂保留为历史读取 API；任何带任意 Schema 的公网写命令都不再可达。原 R1 A10 工具 CRUD 为历史合同，需按 ADR-0035 走正式冻结差异评审。内置 `platform_utc_now` 是当前唯一被实际装配的 handler，不得把未接通旧启发式工具或 MCP 冒称“已内置”。

## Verification

Go 与 Admin 构建、变更文件 LSP、生成的请求 Schema 和国际化检查通过；新后端 `/readyz` 200，匿名内置目录 401，对工具 POST/PUT/DELETE 的真实 HTTP 均为 405 并返回 Allow: GET, OPTIONS。管理端现有会话刷新后工具页只显示代码内置“UTC 时间”的名称、描述及可执行状态，未出现配置入口或机器码。跨租户未播种状态、模型实际调用工具及冻结旧 Run 仍未做端到端验收。此前租户数据里的工具头和历史版本未因本次 UI/API 收敛而删除。