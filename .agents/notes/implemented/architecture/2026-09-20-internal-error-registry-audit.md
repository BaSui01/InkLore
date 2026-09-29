# Agent Note: 内部维护与未配置模型错误归属核对

Status: implemented

Task: INK-180
Scope: docs/system-design/api/errors, internal/llm, internal/worldview

## Problem

设计文档 13 与 05 分册中提及了 llm.provider_not_configured 与 worldview.projection_corrupt。但在冻结的公开错误目录（docs/system-design/api/errors/）中并未包含这两个错误码。需核对是否需要增补为公开错误码，或明确其作为内部错误/故障原因的边界。

## Decision

1. **llm.provider_not_configured**：
   - 保持为 LLM 网关内部调度与路由层面的具体失败原因（Failure Reason），不对外暴露为公开顶级契约错误码；
   - 当所有 Provider 均未配置或不可用时，对外统一映射为已冻结的公开错误码 `llm.capacity_unavailable` (HTTP 503)，客户端提示联系管理员配置渠道或稍后重试。
2. **worldview.projection_corrupt**：
   - 保持为世界观投影重构作业（ReplayHeads）检测到账本事件链断裂或 payload 丢失时的内部防御性停机告警错误（HTTP 500）；
   - 该错误属于运维审计级内部故障，不对外作为业务可恢复错误开放；对外统一映射为平台通用 500 契约。
3. **保持公开契约稳定性**：
   - 不修改已冻结的 OpenAPI 契约与 freeze-manifest，保持现存 98 个公开错误定义的绝对稳定性。

## Alternatives considered

将两项补充登记到 docs/system-design/api/errors/*.json：这将改变已冻结的契约 baseline，需要重新生成 freeze-manifest 并修改各端错误处理映射。鉴于二者均为不可自愈的系统级/配置级故障，对外暴露细分业务码没有客户端恢复收益，反而增加前端适配复杂性。

## Consequences

- 契约目录保持绝对稳定，设计检查（check-design.cjs）无任何漂移；
- 内部调度和维护作业保留了精确的排障标识，满足可观测性与运维定位要求。

## Verification

1. 检查 docs/system-design/tools/check-design.cjs 验证通过；
2. 全仓构建与静态检查通过。
