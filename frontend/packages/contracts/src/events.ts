/**
 * @inklore/contracts - 实时流事件契约定义
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.4
 * SSE 独占请求级生成流，WS 独占系统级低频广播。
 * 纯净排版与零 Emoji 铁律：杜绝隐藏思考链外泄，只展示 reasoning.summary。
 */

import { Seq } from "./types";

/** 智能体生成流事件类型联合 */
export type RunEventType =
  | "run.started"
  | "text.delta"
  | "reasoning.summary"
  | "tool.started"
  | "tool.completed"
  | "approval.required"
  | "node.updated"
  | "usage.updated"
  | "run.succeeded"
  | "run.failed"
  | "run.cancelled";

/** 运行启动载荷 */
export interface RunStartedPayload {
  task_kind: string;
  model?: string;
  inputs?: Record<string, unknown>;
}

/** 逐字推流增量载荷 */
export interface TextDeltaPayload {
  delta: string;
  index?: number;
}

/** 推理摘要载荷 (仅公开安全摘要，严禁隐藏思考过程) */
export interface ReasoningSummaryPayload {
  summary: string;
}

/** 工具调用开始载荷 */
export interface ToolStartedPayload {
  tool_call_id: string;
  tool_name: string;
  arguments_summary?: Record<string, unknown>;
}

/** 工具调用完成载荷 */
export interface ToolCompletedPayload {
  tool_call_id: string;
  tool_name: string;
  outcome: "success" | "error";
  summary?: string;
}

/** 人工安全审批请求载荷 */
export interface ApprovalRequiredPayload {
  approval_id: string;
  node_id: string;
  tool_name: string;
  risk_level: "low" | "medium" | "high" | "critical";
  expires_at: string;
  arguments_summary?: Record<string, unknown>;
}

/** DAG 节点状态更新载荷 */
export interface NodeUpdatedPayload {
  node_id: string;
  status: "pending" | "running" | "completed" | "failed" | "skipped";
  node_type: string;
  duration_ms?: number;
}

/** Token 用量与成本更新载荷 */
export interface UsageUpdatedPayload {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_micros?: number;
}

/** 智能体运行成功终态载荷 */
export interface RunSucceededPayload {
  final_output_summary?: string;
  duration_ms: number;
}

/** 智能体运行失败终态载荷 */
export interface RunFailedPayload {
  error_code: string;
  error_message?: string;
  duration_ms: number;
}

/** 智能体运行取消终态载荷 */
export interface RunCancelledPayload {
  reason?: string;
  cancelled_by?: string;
  duration_ms: number;
}

/** 事件载荷判别映射表 */
export interface RunEventPayloadMap {
  "run.started": RunStartedPayload;
  "text.delta": TextDeltaPayload;
  "reasoning.summary": ReasoningSummaryPayload;
  "tool.started": ToolStartedPayload;
  "tool.completed": ToolCompletedPayload;
  "approval.required": ApprovalRequiredPayload;
  "node.updated": NodeUpdatedPayload;
  "usage.updated": UsageUpdatedPayload;
  "run.succeeded": RunSucceededPayload;
  "run.failed": RunFailedPayload;
  "run.cancelled": RunCancelledPayload;
}

/** 强类型 SSE 事件封套 */
export interface RunEventEnvelope<TType extends RunEventType = RunEventType> {
  run_id: string;
  seq: Seq;
  occurred_at: string;
  event: TType;
  payload: RunEventPayloadMap[TType];
}

/** 终态事件类型判断 */
export function isTerminalEvent(event: RunEventType): boolean {
  return event === "run.succeeded" || event === "run.failed" || event === "run.cancelled";
}

/** WebSocket 系统级低频广播事件消息 (无正文) */
export interface PlatformNotificationMessage {
  event_id: string;
  topic: string;
  resource_id: string;
  revision: number;
  occurred_at: string;
}
