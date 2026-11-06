import { useCallback, useEffect, useRef, useState } from "react";
import { type AgentAuditRun, isApiError } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";
import { Button, ErrorPresenter } from "@inklore/ui";
import { RunInspector } from "@inklore/run-ui";
import { adminSession } from "../api/adminApi";
import { getAgentAuditRun, listAgentAuditRuns } from "../api/agentAudit";

export function AgentRunAudit() {
  const { t } = useTranslation();
  const [items, setItems] = useState<AgentAuditRun[]>([]);
  const [selected, setSelected] = useState<AgentAuditRun | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(false);
  const [denied, setDenied] = useState(false);
  const version = useRef(0);
  const detailVersion = useRef(0);
  const clear = useCallback(() => { version.current++; detailVersion.current++; setItems([]); setSelected(null); setCursor(null); setDenied(true); setLoading(false); }, []);
  const failure = useCallback((cause: unknown) => {
    if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) clear();
    setError(cause);
  }, [clear]);
  const load = useCallback(async (after?: string) => {
    // StrictMode replays effects: the newer request supersedes the old one.
    const current = ++version.current; setLoading(true); setError(null);
    try {
      const page = await listAgentAuditRuns(after);
      if (current !== version.current) return;
      setItems(previous => after ? [...previous, ...page.items.filter(item => !previous.some(old => old.id === item.id))] : page.items);
      setCursor(page.has_more ? page.next_cursor ?? null : null); setDenied(false);
    } catch (cause) { if (current === version.current) failure(cause); }
    finally { if (current === version.current) setLoading(false); }
  }, [failure]);
  useEffect(() => {
    const epoch = adminSession.getSessionEpoch();
    const unsubscribe = adminSession.subscribe(value => { if (value.status !== "authenticated" || value.sessionEpoch !== epoch) clear(); });
    void load();
    return () => { unsubscribe(); version.current++; detailVersion.current++; };
  }, [load, clear]);
  const inspect = async (id: string) => {
    const current = version.current; const detail = ++detailVersion.current;
    try { const value = await getAgentAuditRun(id); if (current === version.current && detail === detailVersion.current) { setSelected(value); setError(null); } }
    catch (cause) { if (current === version.current && detail === detailVersion.current) failure(cause); }
  };
  return <section aria-label={t("agentAudit.title")} className="flex-1 min-h-0 overflow-auto p-4 space-y-4">
    <div className="flex flex-wrap justify-between gap-3"><h2 className="font-semibold">{t("agentAudit.title")}</h2><Button variant="outline" disabled={loading || denied} onClick={() => void load()}>{t("common.refresh")}</Button></div>
    <p className="text-sm text-[var(--text-secondary)]">{t("agentAudit.notice")}</p>
    {error != null && <ErrorPresenter error={error} onRetry={denied ? undefined : () => void load()} />}
    {denied && <p role="alert">{t("agentQuestion.denied")}</p>}
    {loading && <p role="status">{t("common.loading")}</p>}
    {!loading && error == null && items.length === 0 && <p>{t("agentAudit.empty")}</p>}
    <div className="space-y-2">{items.map(item => <button key={item.id} className="block w-full rounded border border-[var(--border-decorative)] p-3 text-left text-sm break-words hover:bg-[var(--bg-subtle)]" onClick={() => void inspect(item.id)}>
      <span className="block font-medium">{item.task_kind} · {t(`agentState.${item.status}`)}</span><span className="text-xs">{item.id}</span>
    </button>)}</div>
    {cursor && <Button variant="outline" disabled={loading} onClick={() => void load(cursor)}>{t("agentAudit.more")}</Button>}
    {selected && <RunInspector mode="redacted-summary" runId={selected.id} taskKind={selected.task_kind} status={selected.status} nodeCount={selected.node_count} durationMs={selected.duration_ms} costMicros={selected.cost_micros} errorCode={selected.error_code ?? undefined} />}
  </section>;
}
