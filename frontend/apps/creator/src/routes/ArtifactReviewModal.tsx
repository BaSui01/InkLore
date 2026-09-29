import React from "react";
import { Modal, Button, DiffView } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { Check, X } from "lucide-react";

export interface ArtifactReviewModalProps {
  open: boolean;
  originalText: string;
  suggestedText: string;
  actionTitle?: string;
  sourceRevision?: number;
  sourceHash?: string;
  artifactHash?: string;
  partial?: boolean;
  onClose: () => void;
  onAdopt: (adoptedText: string) => void;
}

export function ArtifactReviewModal({
  open, originalText, suggestedText, actionTitle, sourceRevision, sourceHash, artifactHash, partial = false, onClose, onAdopt,
}: ArtifactReviewModalProps) {
  const { t } = useTranslation();
  return (
    <Modal open={open} onClose={onClose} title={actionTitle || t("agentWorkbench.artifactReview")} footer={
      <div className="flex w-full items-center justify-between">
        <Button variant="ghost" onClick={onClose} icon={<X className="h-3.5 w-3.5" />}>{t("agentWorkbench.discard")}</Button>
        <Button variant="primary" disabled={partial} onClick={() => onAdopt(suggestedText)} icon={<Check className="h-3.5 w-3.5" />}>{t("agentWorkbench.confirmAdopt")}</Button>
      </div>
    }>
      <div className="max-h-[60vh] space-y-4 overflow-y-auto p-1">
        <div className="rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-subtle)] p-3 text-xs text-[var(--text-secondary)]">
          <p>{partial ? t("agentWorkbench.partialWarning") : t("agentWorkbench.reviewNotice")}</p>
          {sourceRevision !== undefined && <p className="mt-2">{t("agentWorkbench.sourceRevision", sourceRevision)}</p>}
          {sourceHash && <p className="break-all">{t("agentWorkbench.sourceHash", sourceHash)}</p>}
          {artifactHash && <p className="break-all">{t("agentWorkbench.artifactHash", artifactHash)}</p>}
        </div>
        <DiffView
          left={{ resourceId: "source", revision: sourceRevision || 0, contentHash: sourceHash || "", text: originalText, status: "loaded" }}
          right={{ resourceId: "artifact", revision: sourceRevision || 0, contentHash: artifactHash || "", text: suggestedText, status: "loaded" }}
          leftLabel={t("agentWorkbench.sourceText")}
          rightLabel={t("agentWorkbench.proposal")}
          readOnly
        />
      </div>
    </Modal>
  );
}

