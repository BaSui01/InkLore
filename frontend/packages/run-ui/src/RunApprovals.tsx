import { useEffect, useState } from "react";
import { type AgentApproval, isApiError } from "@inklore/contracts";
import { getApiSDK } from "@inklore/transport";
import { ErrorPresenter } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { ApprovalCard } from "./ApprovalCard";

export function RunApprovals({ runId, runRevision, sourceRevision, disabled, onRefresh, onDenied }: {
  runId: string; runRevision: number; sourceRevision: number | null; disabled: boolean;
  onRefresh: () => Promise<unknown>; onDenied: (cause: unknown) => void;
}) {
  const { t } = useTranslation();
  const [items, setItems] = useState<AgentApproval[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let alive = true; setLoading(true);
    void getApiSDK().agent.listApprovals().then(values => { if (alive) { setItems(values.filter(item => item.run_id === runId)); setError(null); } })
      .catch(cause => { if (!alive) return; setError(cause); if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) { setItems([]); onDenied(cause); } })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, [runId, runRevision, attempt, onDenied]);
  return <div className="space-y-3">
    {loading && <p role="status">{t("agentApproval.loading")}</p>}
    {error != null && <ErrorPresenter error={error} onRetry={() => setAttempt(value => value + 1)} />}
    {!loading && error == null && items.length === 0 && <p>{t("agentApproval.empty")}</p>}
    {items.map(item => <ApprovalCard key={item.id} approval={item} disabled={disabled || loading} sourceRevision={sourceRevision} onRefresh={onRefresh} onDenied={onDenied} />)}
  </div>;
}
