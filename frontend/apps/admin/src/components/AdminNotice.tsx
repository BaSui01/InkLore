import { useEffect } from "react";
import { createPortal } from "react-dom";
import { AlertCircle, CheckCircle2, RefreshCw, X } from "lucide-react";
import { isApiError } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";
import { safeErrorText } from "@inklore/transport";

interface AdminNoticeProps {
  error?: unknown;
  success?: string | null;
  onRetry?: () => void;
  onDismiss: () => void;
}

/** One persistent operational error (or short success) above every Admin dialog. */
export function AdminNotice({ error, success, onRetry, onDismiss }: AdminNoticeProps) {
  const { t } = useTranslation();
  useEffect(() => {
    if (!error && success) {
      const timer = window.setTimeout(onDismiss, 3500);
      return () => window.clearTimeout(timer);
    }
  }, [error, success, onDismiss]);

  if (!error && !success) return null;
  const failed = error != null;
  const apiError = failed && isApiError(error) ? error : null;
  const safeCode = apiError && /^[a-z][a-z0-9_.]{0,79}$/.test(apiError.code) ? apiError.code : null;
  const requestID = apiError && /^[a-f0-9]{8}-(?:[a-f0-9]{4}-){3}[a-f0-9]{12}$/.test(apiError.request_id) ? apiError.request_id : null;

  return createPortal(
    <div className="fixed right-4 top-5 z-[100] w-[min(25rem,calc(100vw-2rem))] pointer-events-none" role="status" aria-live={failed ? "assertive" : "polite"}>
      <div className={`pointer-events-auto rounded-xl border bg-[var(--bg-surface)] p-4 shadow-2xl ${failed ? "border-[var(--status-danger)]" : "border-[var(--status-success)]"}`}>
        <div className="flex items-start gap-3">
          {failed ? <AlertCircle className="h-5 w-5 shrink-0 text-[var(--status-danger)]" /> : <CheckCircle2 className="h-5 w-5 shrink-0 text-[var(--status-success)]" />}
          <div className="min-w-0 flex-1">
            <p className="text-sm font-semibold text-[var(--text-primary)]">{failed ? t("common.error") : t("common.success")}</p>
            <p className="mt-1 text-sm text-[var(--text-secondary)] break-words">{failed ? safeErrorText(error, t) : success}</p>
            {safeCode && <p className="mt-2 text-xs font-mono text-[var(--text-muted)] break-all">{safeCode}{requestID ? ` · ${requestID}` : ""}</p>}
            {failed && onRetry && <button type="button" onClick={onRetry} className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-[var(--action-primary)] cursor-pointer"><RefreshCw className="h-4 w-4" />{t("common.refresh")}</button>}
          </div>
          <button type="button" onClick={onDismiss} aria-label={t("common.close")} className="shrink-0 rounded p-1 text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"><X className="h-4 w-4" /></button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
