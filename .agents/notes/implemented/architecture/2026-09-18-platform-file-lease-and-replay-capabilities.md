# Agent Note: 平台文件、租约、outbox及通知的已实现组件

Status: implemented

Task: INK-162
Scope: 平台文件、租约、outbox及通知的已实现组件

## Problem

文件或网络解析不能占用业务UoW；裸epoch或HTTP字段不能代表worker执行权；只写inbox再异步投影会丢失原子性。 已检索既有平台原子边界与身份锁序笔记，保留其默认行为，本篇补充领域落地约束。

## Decision

Prepared私有bytes在事务外校验后原子写bytea；引用经消费Port且绑定在引用前加锁。私有Lease和transaction-bound命令权分开，checkpoint起止验证PG租约，Recover独立于Execute。Outbox与inbox本地回调同事务；App重放先锁当前admin/operator身份并提供私有cap，系统dead-letter审计nil actor。WS轮询已提交PG安全事件并逐条复核权限，只发送5字段。

## Alternatives considered

单进程Hub无法覆盖多实例；任意caller提供epoch不构成授权；把资源Owner当worker actor不真实，因此不用这些方案。

## Consequences

组件已有真实验收，ADR-0019已补齐并通过304机读门禁；162的平台任务已完成。领域作业producer与worker调度、生产指标采样及维护调度分别留给171/172/176，不能将技术验收adapter说成已上线业务作业。

## Verification

主独立go run ./cmd/acceptance/ink162 all与conditional_download均退出0，六组含真实WS、实际App重放权限/审计及304空body条件下载；通用jobs机制使用真实PG/scope验收adapter，领域消费者边界明确留给171/172。

详见 [任务验收](../testing/2026-09-18-acceptance-ink-162.md)。
