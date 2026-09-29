/**
 * @inklore/transport - 幂等操作注册表 (OperationRegistry)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §5
 * 
 * 核心铁律：
 * 1. 纯内存维护操作状态：prepared -> inflight -> succeeded | failed | unknown。
 * 2. 幂等 Key 为 UUIDv4，每次用户新意图生成一次；网络重试与 401 刷新重放严格保持同 Key。
 * 3. 严格禁止在注册表中保存密码、密钥或大篇幅正文载荷（仅保留轻量哈希指纹）。
 * 4. 具备 24 小时过期滑动窗口自动清理机制。
 */

import { ScopeKey, UUID, generateUUID } from "@inklore/contracts";

export type OperationState = "prepared" | "inflight" | "succeeded" | "failed" | "unknown";

export interface OperationRecord {
  operationId: string;
  key: UUID;
  canonicalMethodPath: string;
  payloadHash: string;
  scope: ScopeKey;
  createdAt: number;
  state: OperationState;
  resultRef?: unknown;
}

export interface RegisterOperationInput {
  operationId: string;
  key?: UUID;
  canonicalMethodPath: string;
  payloadHash?: string;
  scope: ScopeKey;
}

const CLEANUP_WINDOW_MS = 24 * 60 * 60 * 1000; // 24小时

export class OperationRegistry {
  private readonly records = new Map<string, OperationRecord>();

  /** 注册或获取现有幂等操作 */
  register(input: RegisterOperationInput): OperationRecord {
    this.pruneExpired();

    const key = input.key || generateUUID();
    const existing = this.records.get(key);
    if (existing) {
      return existing;
    }

    const record: OperationRecord = {
      operationId: input.operationId,
      key,
      canonicalMethodPath: input.canonicalMethodPath,
      payloadHash: input.payloadHash || "",
      scope: { ...input.scope },
      createdAt: Date.now(),
      state: "prepared",
    };

    this.records.set(key, record);
    return record;
  }

  /** 获取指定 Key 的记录 */
  get(key: string): OperationRecord | undefined {
    return this.records.get(key);
  }

  /** 标记操作进入 Inflight */
  markInflight(key: string): void {
    const record = this.records.get(key);
    if (record) {
      record.state = "inflight";
    }
  }

  /** 标记操作成功 */
  markSucceeded(key: string, resultRef?: unknown): void {
    const record = this.records.get(key);
    if (record) {
      record.state = "succeeded";
      record.resultRef = resultRef;
    }
  }

  /** 标记操作失败 */
  markFailed(key: string): void {
    const record = this.records.get(key);
    if (record) {
      record.state = "failed";
    }
  }

  /** 标记操作状态未知（如超时或网络断开） */
  markUnknown(key: string): void {
    const record = this.records.get(key);
    if (record) {
      record.state = "unknown";
    }
  }

  /** 主体切换或登出时全量清理 */
  clear(): void {
    this.records.clear();
  }

  /** 自动清理超过 24 小时的旧记录 */
  private pruneExpired(): void {
    const cutoff = Date.now() - CLEANUP_WINDOW_MS;
    for (const [key, record] of this.records.entries()) {
      if (record.createdAt < cutoff) {
        this.records.delete(key);
      }
    }
  }
}
