import React, { useState, useEffect } from "react";
import { Modal, Button, DiffView } from "@inklore/ui";
import { useI18n } from "@inklore/i18n";
import { getApiSDK, reportSafeDiagnostic } from "@inklore/transport";

export interface ConflictResolutionModalProps {
  open: boolean;
  chapterId: string;
  localBody: string;
  onClose: () => void;
  onUseServer: (serverBody: string, serverRevision: number) => void;
  onOverwriteWithLocal: (serverRevision: number) => void;
}

export function ConflictResolutionModal({
  open,
  chapterId,
  localBody,
  onClose,
  onUseServer,
  onOverwriteWithLocal,
}: ConflictResolutionModalProps) {
  const { t } = useI18n();
  const api = getApiSDK();

  const [loading, setLoading] = useState(false);
  const [remoteBody, setRemoteBody] = useState<string | null>(null);
  const [remoteRevision, setRemoteRevision] = useState<number | null>(null);
  const [loadError, setLoadError] = useState<unknown>(null);

  useEffect(() => {
    if (!open || !chapterId) {
      setRemoteBody(null);
      setRemoteRevision(null);
      setLoadError(null);
      return;
    }

    let active = true;
    setLoading(true);
    setLoadError(null);

    api.content
      .getChapterBody(chapterId)
      .then((res) => {
        if (!active) return;
        setRemoteBody(res.body);
        setRemoteRevision(res.chapter.revision);
      })
      .catch((err) => {
        if (!active) return;
        reportSafeDiagnostic("creator.conflict_fetch_failed", err);
        setLoadError(err);
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
    };
  }, [open, chapterId]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("editor.conflictTitle")}
      footer={
        <div className="flex items-center justify-between w-full">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              disabled={loading || remoteBody === null || remoteRevision === null}
              onClick={() => {
                if (remoteBody !== null && remoteRevision !== null) {
                  onUseServer(remoteBody, remoteRevision);
                }
              }}
            >
              {t("editor.conflictUseServer")}
            </Button>
            <Button
              variant="primary"
              disabled={loading || remoteRevision === null}
              onClick={() => {
                if (remoteRevision !== null) {
                  onOverwriteWithLocal(remoteRevision);
                }
              }}
            >
              {t("editor.conflictKeepLocal")}
            </Button>
          </div>
        </div>
      }
    >
      <div className="space-y-4 max-h-[60vh] overflow-y-auto p-1">
        {loading && (
          <div className="p-8 text-center text-stone-500 text-sm">
            {t("editor.conflictResolving")}
          </div>
        )}

        {loadError != null && (
          <div className="p-4 rounded bg-red-50 dark:bg-red-950/30 text-red-700 dark:text-red-300 text-xs">
            {t("editor.loadFailed")}
          </div>
        )}

        {!loading && remoteBody !== null && (
          <DiffView
            left={{
              resourceId: chapterId,
              revision: remoteRevision ?? 0,
              contentHash: "",
              text: remoteBody,
              status: "loaded",
            }}
            right={{
              resourceId: chapterId,
              revision: 0,
              contentHash: "",
              text: localBody,
              status: "loaded",
            }}
            leftLabel={t("editor.conflictServerVersion")}
            rightLabel={t("editor.conflictLocalDraft")}
            readOnly
          />
        )}
      </div>
    </Modal>
  );
}
