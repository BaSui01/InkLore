import { useEffect, useRef, useState } from "react";
import { type AgentApproval, type ApprovalDecisionInput, isApiError, isNetworkError } from "@inklore/contracts";
import { getApiSDK } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { Button, ErrorPresenter } from "@inklore/ui";

export function ApprovalCard({ approval, disabled, sourceRevision, onRefresh, onDenied }: {
  approval: AgentApproval; disabled: boolean; sourceRevision: number | null;
  onRefresh: () => Promise<unknown>; onDenied: (cause: unknown) => void;
}) {
  const { t } = useTranslation();
  const api = getApiSDK();
  const [current, setCurrent] = useState(approval);
  const [busy, setBusy] = useState(false);
  const [unknown, setUnknown] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [now, setNow] = useState(Date.now);
  const lock = useRef(false);
  const intent = useRef<{ key: string; decision: "approve" | "reject"; body: ApprovalDecisionInput } | null>(null);
  useEffect(() => { setCurrent(value => approval.revision > value.revision ? approval : value); }, [approval]);
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 15_000); return () => clearInterval(timer); }, []);
  const owner = api.getSession()?.getScope()?.userId === current.owner_user_id;
  const sourceChanged = current.source_revision !== sourceRevision;
  const closed = current.status !== "pending" || Date.parse(current.expires_at) <= now;
  const refresh = async () => {
    const fresh = await api.agent.getApproval(current.id);
    setCurrent(fresh);
    if (fresh.status !== "pending") { intent.current = null; setUnknown(false); }
    await onRefresh();
  };
  const decide = async (decision: "approve" | "reject") => {
    if (lock.current || disabled || !owner || closed || sourceChanged) return;
    lock.current = true; setBusy(true); setError(null);
    if (!intent.current) intent.current = { key: crypto.randomUUID(), decision, body: { expected_revision: current.revision, arguments_hash: current.arguments_hash, source_revision: current.source_revision } };
    try {
      const value = await api.agent.decideApproval(current.id, intent.current.decision, intent.current.body, intent.current.key);
      setCurrent(value); intent.current = null; setUnknown(false); await onRefresh();
    } catch (cause) {
      setError(cause);
      if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) { onDenied(cause); return; }
      const uncertain = isNetworkError(cause) || (isApiError(cause) && (cause.httpStatus >= 500 || cause.code === "platform.operation_in_progress"));
      setUnknown(uncertain && !!intent.current);
      if (!uncertain) intent.current = null;
      if (isApiError(cause) && cause.httpStatus === 409 && !uncertain) {
        try { await refresh(); } catch (freshError) { setError(freshError); if (isApiError(freshError) && [401, 403, 404].includes(freshError.httpStatus)) onDenied(freshError); }
      }
    } finally { lock.current = false; setBusy(false); }
  };
  return <section aria-label={t("agentApproval.title")} className="space-y-3 rounded-xl border border-[var(--status-warning)] p-4 break-words">
    <h3 className="font-semibold">{t("agentApproval.title")}</h3>
    <dl className="space-y-1 text-sm">
      <div><dt className="inline font-medium">{t("agentApproval.tool")}:</dt> <dd className="inline">{current.tool_version_id}</dd></div>
      <div><dt className="inline font-medium">{t("agentApproval.node")}:</dt> <dd className="inline">{current.node_id}</dd></div>
      <div><dt className="inline font-medium">{t("agentApproval.expires")}:</dt> <dd className="inline">{current.expires_at}</dd></div>
    </dl>
    <p className="text-sm">{t("agentApproval.review")}</p>
    <pre className="max-h-60 overflow-auto whitespace-pre-wrap rounded bg-[var(--bg-subtle)] p-3 text-xs">{current.arguments_json}</pre>
    {sourceChanged && <p role="alert">{t("agentQuestion.sourceChanged")}</p>}
    {closed ? <p role="status">{t("agentApproval.closed")}</p> : <div className="flex flex-wrap gap-2">
      {unknown ? <Button disabled={disabled || busy} onClick={() => void decide(intent.current!.decision)}>{t("agentQuestion.retry")}</Button> : <>
        <Button loading={busy} disabled={disabled || !owner || sourceChanged} onClick={() => void decide("approve")}>{t("agentApproval.approve")}</Button>
        <Button variant="outline" disabled={disabled || busy || !owner || sourceChanged} onClick={() => void decide("reject")}>{t("agentApproval.reject")}</Button>
      </>}
    </div>}
    {unknown && <p role="status">{t("agentQuestion.unknown")}</p>}
    {error != null && <ErrorPresenter error={error} onRetry={() => void refresh().catch(cause => { setError(cause); if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) onDenied(cause); })} />}
  </section>;
}
