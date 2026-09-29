/**
 * @inklore/recovery - IndexedDB 底层封装
 * 专为创作者离线草稿设计，具备版本升级、索引管理与事务安全
 */

import { RecoveryUnavailableError } from "@inklore/contracts";

export const DB_NAME = "inklore_draft_recovery_v1";
export const DB_VERSION = 1;
export const STORE_SNAPSHOTS = "draft_snapshots";

export async function openRecoveryDB(): Promise<IDBDatabase> {
  if (typeof indexedDB === "undefined") {
    throw new RecoveryUnavailableError("storage_unsupported", "IndexedDB is not supported in this environment");
  }

  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);

    request.onupgradeneeded = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      if (!db.objectStoreNames.contains(STORE_SNAPSHOTS)) {
        const store = db.createObjectStore(STORE_SNAPSHOTS, { keyPath: "recovery_id" });
        // 组合索引：主体与章节定向查找
        store.createIndex("by_subject_chapter", ["tenant_id", "user_id", "novel_id", "chapter_id"], { unique: false });
        // 组合索引：主体与时间查找（用于超期清理与额度计算）
        store.createIndex("by_subject_saved", ["tenant_id", "user_id", "saved_at"], { unique: false });
        // 单列索引：全量时间索引
        store.createIndex("by_saved_at", "saved_at", { unique: false });
      }
    };

    request.onsuccess = () => {
      resolve(request.result);
    };

    request.onerror = () => {
      reject(
        new RecoveryUnavailableError(
          "storage_unsupported",
          "Failed to open draft recovery IndexedDB",
          request.error
        )
      );
    };

    request.onblocked = () => {
      console.warn("InkLore diagnostic", { event: "recovery.upgrade_blocked" });
    };
  });
}
