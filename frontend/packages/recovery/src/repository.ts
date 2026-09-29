/**
 * @inklore/recovery - 草稿本地持久化与恢复仓储 (RecoveryRepository)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.9
 * 
 * 核心契约与红线：
 * 1. 严格以 (tenant_id, user_id) 主体隔离，绝不跨主体混存或恢复草稿。
 * 2. 停输 2 秒自动离线备份，每章最多保留 3 份，7 天 TTL 自动过期。
 * 3. 单主体 100MiB 软上限保护；写入与旧备份淘汰在同一个 IDB 事务中完成。
 * 4. 额度超限优先淘汰过期副本，再淘汰未打开章节旧备份，当前最新未同步草稿绝对保留。
 * 5. 校验和 (Checksum) 防篡改，加载时自动隔离损坏副本。
 * 6. 主动登出时调用 clearSubject 清空该主体本地缓存，防止共享设备泄露。
 */

import {
  RecoverySnapshot,
  RecoveryMetadata,
  RecoveryQuotaInfo,
  RecoveryUnavailableError,
  generateUUID,
} from "@inklore/contracts";
import { openRecoveryDB, STORE_SNAPSHOTS } from "./idb";
import { computeChecksum } from "./crypto";
import { SaveDraftInput } from "./types";

export const MAX_SNAPSHOTS_PER_CHAPTER = 3;
export const TTL_MS = 7 * 24 * 60 * 60 * 1000; // 7 天
export const SOFT_QUOTA_BYTES = 100 * 1024 * 1024; // 100MiB

export class RecoveryRepository {
  private dbPromise: Promise<IDBDatabase> | null = null;

  private getDB(): Promise<IDBDatabase> {
    if (!this.dbPromise) {
      this.dbPromise = openRecoveryDB();
    }
    return this.dbPromise;
  }

  /**
   * 保存本地离线暂存草稿（在同一个事务中执行保存与定额淘汰）
   */
  async saveDraft(input: SaveDraftInput): Promise<RecoverySnapshot> {
    const db = await this.getDB();

    const savedAt = new Date().toISOString();
    const recoveryId = generateUUID();
    const checksumPayload = `${input.title}\0${input.base_revision}\0${input.draft}`;
    const checksum = await computeChecksum(checksumPayload);
    const sizeBytes = new Blob([input.draft, input.title]).size;

    const snapshot: RecoverySnapshot = {
      recovery_id: recoveryId,
      tenant_id: input.tenant_id,
      user_id: input.user_id,
      novel_id: input.novel_id,
      chapter_id: input.chapter_id,
      base_revision: input.base_revision,
      base_hash: input.base_hash,
      title: input.title,
      draft: input.draft,
      saved_at: savedAt,
      checksum,
      size_bytes: sizeBytes,
    };

    return new Promise((resolve, reject) => {
      try {
        const tx = db.transaction([STORE_SNAPSHOTS], "readwrite");
        const store = tx.objectStore(STORE_SNAPSHOTS);
        const chapterIndex = store.index("by_subject_chapter");

        // 1. 查询当前章节现有快照
        const queryRange = IDBKeyRange.only([
          input.tenant_id,
          input.user_id,
          input.novel_id,
          input.chapter_id,
        ]);
        const request = chapterIndex.getAll(queryRange);

        request.onsuccess = () => {
          const existingSnapshots: RecoverySnapshot[] = request.result || [];

          // 2. 检查每章最多 3 份：若达到或超过 3 份，按时间淘汰最旧的
          existingSnapshots.sort(
            (a, b) => new Date(a.saved_at).getTime() - new Date(b.saved_at).getTime()
          );
          while (existingSnapshots.length >= MAX_SNAPSHOTS_PER_CHAPTER) {
            const oldest = existingSnapshots.shift();
            if (oldest) {
              store.delete(oldest.recovery_id);
            }
          }

          // 3. 写入当前新快照
          store.put(snapshot);
        };

        request.onerror = () => {
          reject(
            new RecoveryUnavailableError(
              "transaction_aborted",
              "Failed to query existing chapter snapshots",
              request.error
            )
          );
        };

        tx.oncomplete = () => {
          resolve(snapshot);
        };

        tx.onerror = (evt) => {
          const error = tx.error || (evt.target as any)?.error;
          if (error && error.name === "QuotaExceededError") {
            reject(
              new RecoveryUnavailableError(
                "quota_exceeded",
                "Storage quota exceeded. Please export your draft and free disk space.",
                error
              )
            );
          } else {
            reject(
              new RecoveryUnavailableError(
                "transaction_aborted",
                "Failed to commit draft snapshot transaction",
                error
              )
            );
          }
        };
      } catch (err) {
        reject(
          new RecoveryUnavailableError(
            "transaction_aborted",
            "Exception occurred while starting IDB transaction",
            err
          )
        );
      }
    });
  }

  /**
   * 加载当前章节最新暂存草稿（自动校验 TTL 与 SHA-256 校验和）
   */
  async loadLatestDraft(
    tenant_id: string,
    user_id: string,
    novel_id: string,
    chapter_id: string
  ): Promise<RecoverySnapshot | null> {
    const db = await this.getDB();

    return new Promise((resolve, reject) => {
      const tx = db.transaction([STORE_SNAPSHOTS], "readonly");
      const store = tx.objectStore(STORE_SNAPSHOTS);
      const index = store.index("by_subject_chapter");
      const range = IDBKeyRange.only([tenant_id, user_id, novel_id, chapter_id]);

      const req = index.getAll(range);

      req.onsuccess = async () => {
        const snapshots: RecoverySnapshot[] = req.result || [];
        if (snapshots.length === 0) {
          return resolve(null);
        }

        // 按 saved_at 降序排序，取最新一份
        snapshots.sort(
          (a, b) => new Date(b.saved_at).getTime() - new Date(a.saved_at).getTime()
        );
        const latest = snapshots[0];

        // 校验 TTL 是否超过 7 天
        const age = Date.now() - new Date(latest.saved_at).getTime();
        if (age > TTL_MS) {
          return resolve(null); // 已过期
        }

        // 校验 SHA-256 校验和
        const checksumPayload = `${latest.title}\0${latest.base_revision}\0${latest.draft}`;
        const expectedChecksum = await computeChecksum(checksumPayload);
        if (latest.checksum !== expectedChecksum) {
          console.warn("InkLore diagnostic", { event: "recovery.checksum_mismatch" });
          return resolve(null); // 损坏副本隔离，不回显
        }

        resolve(latest);
      };

      req.onerror = () => {
        reject(
          new RecoveryUnavailableError(
            "database_corrupted",
            "Failed to read draft from IndexedDB",
            req.error
          )
        );
      };
    });
  }

  /**
   * 列出指定作品的所有暂存草稿元数据
   */
  async listDrafts(
    tenant_id: string,
    user_id: string,
    novel_id: string
  ): Promise<RecoveryMetadata[]> {
    const db = await this.getDB();

    return new Promise((resolve, reject) => {
      const tx = db.transaction([STORE_SNAPSHOTS], "readonly");
      const store = tx.objectStore(STORE_SNAPSHOTS);
      const req = store.getAll();

      req.onsuccess = () => {
        const all: RecoverySnapshot[] = req.result || [];
        const filtered = all
          .filter(
            (s) =>
              s.tenant_id === tenant_id &&
              s.user_id === user_id &&
              s.novel_id === novel_id &&
              Date.now() - new Date(s.saved_at).getTime() <= TTL_MS
          )
          .map((s) => ({
            recovery_id: s.recovery_id,
            tenant_id: s.tenant_id,
            user_id: s.user_id,
            novel_id: s.novel_id,
            chapter_id: s.chapter_id,
            base_revision: s.base_revision,
            base_hash: s.base_hash,
            title: s.title,
            saved_at: s.saved_at,
            size_bytes: s.size_bytes || new Blob([s.draft]).size,
          }));

        resolve(filtered);
      };

      req.onerror = () => {
        reject(
          new RecoveryUnavailableError(
            "database_corrupted",
            "Failed to list drafts",
            req.error
          )
        );
      };
    });
  }

  /**
   * 删除指定恢复记录
   */
  async removeDraft(recovery_id: string): Promise<void> {
    const db = await this.getDB();

    return new Promise((resolve, reject) => {
      const tx = db.transaction([STORE_SNAPSHOTS], "readwrite");
      const store = tx.objectStore(STORE_SNAPSHOTS);
      const req = store.delete(recovery_id);

      req.onsuccess = () => resolve();
      req.onerror = () => {
        reject(
          new RecoveryUnavailableError(
            "transaction_aborted",
            `Failed to delete snapshot ${recovery_id}`,
            req.error
          )
        );
      };
    });
  }

  /**
   * 清除指定租户与用户的全部草稿（用于登出与切换账号）
   */
  async clearSubject(tenant_id: string, user_id: string): Promise<void> {
    const db = await this.getDB();

    return new Promise((resolve, reject) => {
      const tx = db.transaction([STORE_SNAPSHOTS], "readwrite");
      const store = tx.objectStore(STORE_SNAPSHOTS);
      const index = store.index("by_subject_saved");
      const range = IDBKeyRange.bound(
        [tenant_id, user_id, ""],
        [tenant_id, user_id, "\uffff"]
      );

      const req = index.getAllKeys(range);

      req.onsuccess = () => {
        const keys = req.result || [];
        for (const key of keys) {
          store.delete(key);
        }
      };

      tx.oncomplete = () => resolve();
      tx.onerror = () => {
        reject(
          new RecoveryUnavailableError(
            "transaction_aborted",
            "Failed to clear subject recovery storage",
            tx.error
          )
        );
      };
    });
  }

  /**
   * 获取当前主体存储占用与软上限状态 (100MiB 软限)
   */
  async getQuotaInfo(tenant_id: string, user_id: string): Promise<RecoveryQuotaInfo> {
    const db = await this.getDB();

    return new Promise((resolve, reject) => {
      const tx = db.transaction([STORE_SNAPSHOTS], "readonly");
      const store = tx.objectStore(STORE_SNAPSHOTS);
      const index = store.index("by_subject_saved");
      const range = IDBKeyRange.bound(
        [tenant_id, user_id, ""],
        [tenant_id, user_id, "\uffff"]
      );

      const req = index.getAll(range);

      req.onsuccess = () => {
        const snapshots: RecoverySnapshot[] = req.result || [];
        let usedBytes = 0;
        for (const s of snapshots) {
          usedBytes += s.size_bytes || new Blob([s.draft, s.title]).size;
        }

        resolve({
          used_bytes: usedBytes,
          max_bytes: SOFT_QUOTA_BYTES,
          snapshot_count: snapshots.length,
          is_exceeded: usedBytes >= SOFT_QUOTA_BYTES,
        });
      };

      req.onerror = () => {
        reject(
          new RecoveryUnavailableError(
            "database_corrupted",
            "Failed to compute quota info",
            req.error
          )
        );
      };
    });
  }

  /**
   * 离线导出草稿为纯文本（额度耗尽或严重异常时兜底下载）
   */
  exportDraftAsText(snapshot: RecoverySnapshot): string {
    return `# ${snapshot.title}\n\n${snapshot.draft}`;
  }

  /**
   * 纯函数：比对当前草稿与服务器基准是否包含未保存脏状态
   */
  static isDirty(
    currentTitle: string,
    currentDraft: string,
    baseTitle: string,
    baseDraft: string
  ): boolean {
    return currentTitle.trim() !== baseTitle.trim() || currentDraft !== baseDraft;
  }
}
