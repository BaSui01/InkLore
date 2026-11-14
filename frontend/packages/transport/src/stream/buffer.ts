/**
 * @inklore/transport - 运行流事件缓冲区 (RunEventBuffer)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.4
 * 
 * 核心铁律：
 * 1. 缓冲最多保留最近 2000 个事件或 2MiB 展示片段，超过上限截断旧展示段并标明权威产物读取。
 * 2. 严格按 seq 单调递增去重与防抖，不按文本内容进行不可靠文本去重。
 * 3. 仅提取 reasoning.summary，绝不向视图外泄任何隐藏思考过程。
 * 4. 实时聚合流状态、打字机正文、节点进展与 Token 用量。
 */

import {
  RunEventEnvelope,
  RunEventType,
  Seq,
  UsageUpdatedPayload,
  NodeUpdatedPayload,
  ToolCompletedPayload,
  ApprovalRequiredPayload,
  ToolStartedPayload,
} from "@inklore/contracts";

export const MAX_BUFFER_EVENTS = 2000;
export const MAX_BUFFER_BYTES = 2 * 1024 * 1024; // 2MiB

export interface AggregatedRunState {
  runId: string;
  lastSeq: Seq | 0;
  textBuffer: string;
  reasoningSummary: string;
  activeTools: Map<string, { toolName: string; startedAt: number; summary?: string }>;
  completedTools: ToolCompletedPayload[];
  activeApproval: ApprovalRequiredPayload | null;
  pendingQuestionId: string | null;
  nodeStates: Map<string, NodeUpdatedPayload>;
  usage: UsageUpdatedPayload | null;
  isCompleted: boolean;
  terminalEvent: RunEventType | null;
  isTruncated: boolean;
}

export class RunEventBuffer {
  readonly runId: string;
  private readonly events: RunEventEnvelope[] = [];
  private lastSeq: Seq | 0 = 0;
  private currentBytes = 0;
  private isTruncated = false;

  private textAccumulator = "";
  private reasoningSummary = "";
  private activeTools = new Map<string, { toolName: string; startedAt: number; summary?: string }>();
  private completedTools: ToolCompletedPayload[] = [];
  private activeApproval: ApprovalRequiredPayload | null = null;
  private pendingQuestionId: string | null = null;
  private nodeStates = new Map<string, NodeUpdatedPayload>();
  private usage: UsageUpdatedPayload | null = null;
  private isCompleted = false;
  private terminalEvent: RunEventType | null = null;

  constructor(runId: string) {
    this.runId = runId;
  }

  /**
   * 推入新事件
   * 自动校验 seq 递增，剔除重复 seq，管理 2000 事件/2MiB 阈值
   */
  push(event: RunEventEnvelope): boolean {
    if (event.run_id !== this.runId) {
      return false;
    }

    // 严格单调递增：丢弃重复或逆序 seq
    if (this.lastSeq !== 0 && event.seq <= this.lastSeq) {
      return false;
    }

    const eventJson = JSON.stringify(event);
    const eventSize = new TextEncoder().encode(eventJson).byteLength;

    this.events.push(event);
    this.lastSeq = event.seq;
    this.currentBytes += eventSize;

    // 聚合处理
    this.aggregate(event);

    // 超限截断控制 (2000 events / 2MiB)
    this.enforceLimits();

    return true;
  }

  private aggregate(envelope: RunEventEnvelope): void {
    const { event, payload } = envelope;

    switch (event) {
      case "text.delta": {
        const p = payload as { delta: string };
        if (p?.delta) {
          this.textAccumulator += p.delta;
        }
        break;
      }
      case "reasoning.summary": {
        const p = payload as { summary: string };
        if (p?.summary) {
          this.reasoningSummary = p.summary;
        }
        break;
      }
      case "tool.started": {
        const p = payload as ToolStartedPayload;
        if (p?.invocation_id) {
          this.activeTools.set(p.invocation_id, {
            toolName: p.tool_id,
            startedAt: Date.now(),
          });
        }
        break;
      }
      case "tool.completed": {
        const p = payload as ToolCompletedPayload;
        if (p?.invocation_id) {
          this.activeTools.delete(p.invocation_id);
          this.completedTools.push(p);
        }
        break;
      }
      case "approval.required": {
        this.activeApproval = payload as ApprovalRequiredPayload;
        break;
      }
      case "question.required": {
        this.pendingQuestionId = (payload as { question_id: string }).question_id;
        break;
      }
      case "question.answered": {
        if (this.pendingQuestionId === (payload as { question_id: string }).question_id) this.pendingQuestionId = null;
        break;
      }
      case "node.updated": {
        const p = payload as NodeUpdatedPayload;
        if (p?.node_id) {
          this.nodeStates.set(p.node_id, p);
        }
        break;
      }
      case "usage.updated": {
        const update = payload as UsageUpdatedPayload;
        this.usage = this.usage ? {
          ...update,
          input_tokens: this.usage.input_tokens + update.input_tokens,
          output_tokens: this.usage.output_tokens + update.output_tokens,
          cost_micros: this.usage.cost_micros + update.cost_micros,
          estimated: this.usage.estimated || update.estimated,
        } : { ...update };
        break;
      }
      case "run.succeeded":
      case "run.failed":
      case "run.cancelled": {
        this.isCompleted = true;
        this.terminalEvent = event;
        break;
      }
    }
  }

  private enforceLimits(): void {
    let removed = false;
    while (this.events.length > MAX_BUFFER_EVENTS || this.currentBytes > MAX_BUFFER_BYTES) {
      const oldest = this.events.shift();
      if (!oldest) break;
      this.currentBytes -= new TextEncoder().encode(JSON.stringify(oldest)).byteLength;
      removed = true;
    }
    if (removed) {
      this.isTruncated = true;
      // Rebuild only retained display data. Dropping an event without dropping
      // its aggregate would leave unbounded text/tool memory behind.
      this.textAccumulator = ""; this.reasoningSummary = "";
      this.activeTools.clear(); this.completedTools = []; this.nodeStates.clear();
      this.usage = null; this.activeApproval = null; this.pendingQuestionId = null;
      for (const event of this.events) this.aggregate(event);
    }
  }

  markTruncated(): void { this.isTruncated = true; }

  /** 获取最新游标 seq */
  getLastSeq(): Seq | 0 {
    return this.lastSeq;
  }

  /** 获取当前事件总数 */
  getEventCount(): number {
    return this.events.length;
  }

  /** 获取只读事件列表快照 */
  getEvents(): readonly RunEventEnvelope[] {
    return this.events;
  }

  /** 获取当前聚合渲染状态 */
  getAggregatedState(): AggregatedRunState {
    return {
      runId: this.runId,
      lastSeq: this.lastSeq,
      textBuffer: this.textAccumulator,
      reasoningSummary: this.reasoningSummary,
      activeTools: new Map(this.activeTools),
      completedTools: [...this.completedTools],
      activeApproval: this.activeApproval ? { ...this.activeApproval } : null,
      pendingQuestionId: this.pendingQuestionId,
      nodeStates: new Map(this.nodeStates),
      usage: this.usage ? { ...this.usage } : null,
      isCompleted: this.isCompleted,
      terminalEvent: this.terminalEvent,
      isTruncated: this.isTruncated,
    };
  }

  /** 清空缓冲区 */
  clear(): void {
    this.events.length = 0;
    this.currentBytes = 0;
    this.lastSeq = 0;
    this.textAccumulator = "";
    this.reasoningSummary = "";
    this.activeTools.clear();
    this.completedTools = [];
    this.activeApproval = null;
    this.pendingQuestionId = null;
    this.nodeStates.clear();
    this.usage = null;
    this.isCompleted = false;
    this.terminalEvent = null;
    this.isTruncated = false;
  }
}
