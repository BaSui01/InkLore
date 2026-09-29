import React, { useState, useEffect } from "react";
import { Modal, Button, Badge, ErrorPresenter } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { getApiSDK, reportSafeDiagnostic, safeErrorText } from "@inklore/transport";
import { Chapter, ConfirmChapterResponse } from "@inklore/contracts";
import { CheckCircle2, ShieldCheck, AlertCircle } from "lucide-react";

export interface ConfirmChapterModalProps {
  open: boolean;
  novelId: string;
  chapter: Chapter;
  onClose: () => void;
  onSuccess: (confirmation: ConfirmChapterResponse) => void;
}

export function ConfirmChapterModal({
  open,
  novelId,
  chapter,
  onClose,
  onSuccess,
}: ConfirmChapterModalProps) {
  const { t } = useTranslation();
  const api = getApiSDK();

  const [loading, setLoading] = useState(false);
  const [worldRevision, setWorldRevision] = useState<number | null>(null);
  const [fetchingRevision, setFetchingRevision] = useState(false);
  const [confirmError, setConfirmError] = useState<unknown>(null);

  // 打开弹窗时获取当前世界观版本号
  useEffect(() => {
    if (!open || !novelId) {
      setWorldRevision(null);
      setConfirmError(null);
      return;
    }
    let active = true;
    setFetchingRevision(true);
    setConfirmError(null);

    api.worldview
      .getWorldRevision(novelId)
      .then((res) => {
        if (!active) return;
        setWorldRevision(res.revision);
      })
      .catch((err) => {
        if (!active) return;
        reportSafeDiagnostic("creator.confirm_fetch_world_rev_failed", err);
        setConfirmError(err);
      })
      .finally(() => {
        if (active) setFetchingRevision(false);
      });

    return () => {
      active = false;
    };
  }, [open, novelId]);

  const handleConfirm = async () => {
    if (!chapter || worldRevision === null) return;
    setLoading(true);
    setConfirmError(null);
    try {
      const res = await api.content.confirmChapter(chapter.id, {
        expected_revision: chapter.revision,
        expected_world_revision: worldRevision,
        fact_proposals: [], // 确认当前正文为正式版，若无冲突事实则空提议推进世界观修订
      });
      onSuccess(res);
      onClose();
    } catch (err) {
      reportSafeDiagnostic("creator.confirm_chapter_failed", err);
      setConfirmError(err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="确认本章正文 (Chapter Confirmation)"
      footer={
        <div className="flex items-center justify-between w-full">
          <Button variant="ghost" onClick={onClose} disabled={loading}>
            取消
          </Button>
          <Button
            variant="primary"
            loading={loading}
            disabled={fetchingRevision || worldRevision === null}
            onClick={handleConfirm}
            icon={<ShieldCheck className="w-3.5 h-3.5" />}
          >
            确认入账并定稿
          </Button>
        </div>
      }
    >
      <div className="space-y-4 max-h-[60vh] overflow-y-auto p-1 text-xs">
        <div className="p-3.5 rounded-xl bg-amber-50/60 dark:bg-amber-950/20 border border-amber-200/80 dark:border-amber-800/40 space-y-2">
          <div className="flex items-center gap-2 font-semibold text-amber-900 dark:text-amber-200">
            <ShieldCheck className="w-4 h-4 text-amber-600" />
            <span>跨域事务：确认正文与事实落账</span>
          </div>
          <p className="text-neutral-600 dark:text-neutral-400 leading-relaxed">
            确认操作将在同一原子事务中锁定章节正文快照，将状态从 <code>draft</code> 迁移至 <code>confirmed</code>，并将本章确立的设定与事实永久写入世界观事实账本。
          </p>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between p-2.5 rounded-lg border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900">
            <span className="text-neutral-500">章节标题</span>
            <span className="font-semibold text-neutral-800 dark:text-neutral-200">{chapter.title}</span>
          </div>
          <div className="flex items-center justify-between p-2.5 rounded-lg border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900">
            <span className="text-neutral-500">期望章节修订号 (expected_revision)</span>
            <span className="font-mono font-semibold text-neutral-800 dark:text-neutral-200">rev.{chapter.revision}</span>
          </div>
          <div className="flex items-center justify-between p-2.5 rounded-lg border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900">
            <span className="text-neutral-500">世界观账本版本 (expected_world_revision)</span>
            <span className="font-mono font-semibold text-neutral-800 dark:text-neutral-200">
              {fetchingRevision ? "读取中..." : worldRevision !== null ? `rev.${worldRevision}` : "不可用"}
            </span>
          </div>
        </div>

        {confirmError != null && (
          <div className="pt-2">
            <ErrorPresenter
              error={confirmError}
              context={{ operationLabel: "章节确认事务" }}
              onRetry={worldRevision !== null ? handleConfirm : undefined}
            />
          </div>
        )}
      </div>
    </Modal>
  );
}
