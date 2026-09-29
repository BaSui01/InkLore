/**
 * @inklore/contracts - 恢复与持久化契约
 * 严格对齐 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.9
 * 约定：每章最多 3 份、7 天 TTL、主体 100MiB 软上限。
 */

/** 恢复快照记录模型 */
export interface RecoverySnapshot {
  recovery_id: string; // UUIDv4
  tenant_id: string;
  user_id: string;
  novel_id: string;
  chapter_id: string;
  base_revision: number;
  base_hash: string;
  title: string;
  draft: string; // 章节正文
  saved_at: string; // ISO 8601 UTC 时间
  checksum: string; // SHA-256 校验和
  size_bytes?: number;
}

/** 恢复快照元数据摘要（用于列表与比对） */
export interface RecoveryMetadata {
  recovery_id: string;
  tenant_id: string;
  user_id: string;
  novel_id: string;
  chapter_id: string;
  base_revision: number;
  base_hash: string;
  title: string;
  saved_at: string;
  size_bytes: number;
}

/** 主体存储额度监控状态 */
export interface RecoveryQuotaInfo {
  used_bytes: number;
  max_bytes: number; // 软上限 100MiB (104857600 字节)
  snapshot_count: number;
  is_exceeded: boolean;
}

/** 恢复仓储不可用原因 */
export type RecoveryUnavailableReason =
  | "quota_exceeded"
  | "storage_unsupported"
  | "permission_denied"
  | "database_corrupted"
  | "transaction_aborted";

/** 恢复仓储异常类 */
export class RecoveryUnavailableError extends Error {
  readonly isRecoveryUnavailable = true;
  readonly reason: RecoveryUnavailableReason;

  constructor(reason: RecoveryUnavailableReason, message: string, readonly cause?: unknown) {
    super(`[RecoveryUnavailable: ${reason}] ${message}`);
    this.name = "RecoveryUnavailableError";
    this.reason = reason;
    Object.setPrototypeOf(this, RecoveryUnavailableError.prototype);
  }
}

export function isRecoveryUnavailable(err: unknown): err is RecoveryUnavailableError {
  return err instanceof RecoveryUnavailableError;
}
