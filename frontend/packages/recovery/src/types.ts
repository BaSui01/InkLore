/**
 * @inklore/recovery - 本地持久化与恢复类型
 */

export type {
  RecoverySnapshot,
  RecoveryMetadata,
  RecoveryQuotaInfo,
  RecoveryUnavailableReason,
} from "@inklore/contracts";

export {
  RecoveryUnavailableError,
  isRecoveryUnavailable,
} from "@inklore/contracts";

export interface SaveDraftInput {
  tenant_id: string;
  user_id: string;
  novel_id: string;
  chapter_id: string;
  base_revision: number;
  base_hash: string;
  title: string;
  draft: string;
}

export interface DraftDiffSummary {
  hasTitleChange: boolean;
  hasBodyChange: boolean;
  titleDelta: string;
  wordCountDiff: number;
}
