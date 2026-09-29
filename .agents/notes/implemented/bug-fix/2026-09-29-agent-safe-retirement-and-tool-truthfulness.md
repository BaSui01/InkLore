# Agent Note: 智能体历史版本退役与工具执行状态真实性

Status: implemented

Task: INK-246
Scope: internal/agent 定义与工作流删除、Admin Agent Tools 治理视图

## Problem

[INK-242 原删除笔记](../bug-fix/2026-09-29-agent-deletion-reference-and-admin-notice.md)通过检查所有外键将已引用定义的物理删除变为409；该策略避免破坏历史，却让用户无法从可选目录删除曾使用过的定义。工具页只有已登记的租户配置，`enabled` 被误认为真实 Agent 工具可执行；运行时 ToolService 和 Scheduler 实际未在组合根装配。参照 [INK-226 配置驱动笔记](../feature/2026-09-24-agent-configuration-driven-integration.md)，不以空配置伪成功。相关契约差异已登记 [ADR-0034（Proposed）](../../../../docs/decisions/0034-agent-tool-execution-status-and-safe-retirement.md)。

## Decision

定义与工作流 DELETE 按租户+expected_revision 将配置头退役，保留所有不可变版本和历史引用。`Scheduler` 实现直接读取冻结定义版本（当前尚未装配到组合根），避免日后接入时已有运行误查隐藏头。种子重跑识别保留键的删除标记并报告跳过，不替用户恢复。Admin 将工具配置与可执行工具分区：现有工具响应 `execution_status=not_bound`，可执行工具目录为空，不把静态工具代码、注册配置或“启用”伪装为真正执行。中文卡片去除机器键/摘要英文小字，机器键仍可在编辑/详情用于诊断。

## Alternatives considered

强制级联物理删除可以释放原标识，但会损坏历史记录与在途运行；继续一律返回409可以维持数据库一致，却无法满足用户清理目录的操作；把编译存在的旧工具当作已可执行可以填充页面，但没有实际 Scheduler/执行器注册与租户授权，会产生假功能。均未采用。

## Consequences

删除后可选目录立即消失，旧资源机器键受唯一索引占用，不能用原键直接重新创建。本篇记录未接执行器阶段的真实性门禁；其后 Worker 与首项真实受控工具已按 [INK-246 接线笔记](../feature/2026-09-29-agent-worker-real-tool-execution.md) 接入，不能继续把本篇“工具均未绑定”的当时观察当成当前状态。新增 ToolView 状态字段及软删除语义属于 R1.x 契约差异，ADR-0034 尚待正式评审；本笔记保留原阶段判断，历史 INK-242 笔记亦保留当时行为与验证结论。

## Verification

该阶段核对 `go build ./...`、Admin 生产构建与语言服务器无诊断；当时未执行真实删除或独立 PG 数据写入，静态构建不等于真实 Admin DELETE、旧 Run 重放与工具调用。其后工具登记及在线列表验收见上文所链的新笔记，完整模型工具调用仍未验收。