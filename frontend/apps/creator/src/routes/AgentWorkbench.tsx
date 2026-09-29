import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, Bot, RefreshCw, X } from "lucide-react";
import type { AgentRun, ArtifactView, CatalogDefinition, CatalogWorkflow, LlmModel, RunEventEnvelope } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";
import { Button, ErrorPresenter } from "@inklore/ui";
import { getApiSDK, StreamClient, type AggregatedRunState, type ConnectionState } from "@inklore/transport";
import { RunInspector } from "@inklore/run-ui";
import { ArtifactReviewModal } from "./ArtifactReviewModal";

export interface AgentWorkbenchProps {
  open: boolean;
  novelId: string;
  chapterId: string;
  chapterRevision: number | null;
  chapterText: string;
  models: LlmModel[];
  selectedModelId: string;
  initialSelection?: { text: string; startOffset: number; endOffset: number } | null;
  onClose: () => void;
  onAdopt: (artifact: ArtifactView, selection: AgentWorkbenchProps["initialSelection"], sourceSnapshot: string) => void;
}

type CatalogState = { definitions: CatalogDefinition[]; workflows: CatalogWorkflow[] };
const terminal = (status: AgentRun["status"]) => ["succeeded", "completed", "failed", "cancelled"].includes(status);
const hashText = async (value: string) => {
  const bytes = new TextEncoder().encode(value.replace(/\r\n?/g, "\n"));
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
};

export function AgentWorkbench(props: AgentWorkbenchProps) {
  const { t } = useTranslation();
  const api = getApiSDK();
  const [catalog, setCatalog] = useState<CatalogState>({ definitions: [], workflows: [] });
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogError, setCatalogError] = useState<unknown>(null);
  const [catalogReady, setCatalogReady] = useState(false);
  const [definitionId, setDefinitionId] = useState("");
  const [workflowId, setWorkflowId] = useState("");
  const [taskKind, setTaskKind] = useState("");
  const [taskKey, setTaskKey] = useState("");
  const [instruction, setInstruction] = useState("");
  const [modelId, setModelId] = useState("");
  const [creating, setCreating] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [run, setRun] = useState<AgentRun | null>(null);
  const [runStatus, setRunStatus] = useState<AgentRun["status"] | null>(null);
  const [streamState, setStreamState] = useState<ConnectionState>("idle");
  const [streamSnapshot, setStreamSnapshot] = useState<AggregatedRunState | null>(null);
  const [runError, setRunError] = useState<unknown>(null);
  const [artifacts, setArtifacts] = useState<ArtifactView[]>([]);
  const [review, setReview] = useState<ArtifactView | null>(null);
  const [sourceSnapshot, setSourceSnapshot] = useState("");
  const [sourceHash, setSourceHash] = useState("");
  const [staleRun, setStaleRun] = useState(false);
  const [offline, setOffline] = useState(typeof navigator !== "undefined" && !navigator.onLine);
  const subscription = useRef<{ unsubscribe: () => void; getBuffer: () => { getAggregatedState: () => AggregatedRunState; getLastSeq: () => number | 0 } } | null>(null);
  const cursor = useRef(0);

  const loadCatalog = useCallback(async () => {
    setCatalogLoading(true);
    setCatalogError(null);
    setCatalogReady(false);
    const [defs, wfs] = await Promise.allSettled([api.agent.listCatalogDefinitions(), api.agent.listCatalogWorkflows()]);
    setCatalog((current) => ({
      definitions: defs.status === "fulfilled" ? defs.value : current.definitions,
      workflows: wfs.status === "fulfilled" ? wfs.value : current.workflows,
    }));
    if (defs.status === "fulfilled") {
      setDefinitionId((current) => defs.value.some((item) => item.id === current) ? current : defs.value[0]?.id || "");
    }
    if (wfs.status === "fulfilled") {
      setWorkflowId((current) => wfs.value.some((item) => item.id === current) ? current : "");
    }
    if (defs.status === "fulfilled" && wfs.status === "fulfilled") setCatalogReady(true);
    else setCatalogError(defs.status === "rejected" ? defs.reason : wfs.status === "rejected" ? wfs.reason : null);
    setCatalogLoading(false);
  }, [api]);

  useEffect(() => {
    const update = () => setOffline(!navigator.onLine);
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);

  useEffect(() => {
    if (props.open) void loadCatalog();
  }, [props.open, loadCatalog]);

  useEffect(() => {
    if (props.open) setModelId(props.selectedModelId);
  }, [props.open, props.selectedModelId]);

  const definition = catalog.definitions.find((item) => item.id === definitionId);
  const legalKinds = useMemo(() => (definition?.task_kinds || []).filter((kind) =>
    ["chapter_draft", "chapter_rewrite"].includes(kind)
  ), [definition]);

  const refreshAuthority = useCallback(async (runId: string) => {
    const current = await api.agent.getRun(runId);
    setRun(current);
    setRunStatus(current.status);
    setStaleRun(false);
    if (terminal(current.status)) {
      if (current.status === "succeeded" || current.status === "completed") {
        const list = await api.agent.listArtifacts(runId);
        setArtifacts(list);
      }
    }
    return current;
  }, [api]);

  useEffect(() => {
    if (!run?.id) return;
    const session = api.getSession();
    if (!session) { setRunError(new Error("Authenticated session unavailable")); return; }
    let alive = true;
    const client = new StreamClient({ session });
    const syncBuffer = () => {
      const active = subscription.current;
      if (!active || !alive) return;
      const buffer = active.getBuffer();
      setStreamSnapshot(buffer.getAggregatedState());
      cursor.current = buffer.getLastSeq();
    };
    const stream = client.subscribe({
      runId: run.id,
      lastSeq: cursor.current || undefined,
      onEvent: (event: RunEventEnvelope) => {
        syncBuffer();
        if (event.event === "run.started") setRunStatus("running");
        if (event.event === "approval.required") setRunStatus("waiting_approval");
      },
      onConnection: (state) => {
        setStreamState(state);
        if (state === "open") { setRunError(null); setStaleRun(false); }
      },
      onError: (error) => { if (alive) setRunError(error); },
      onCursorExpired: () => {
        void refreshAuthority(run.id).catch((error) => { if (alive) setRunError(error); });
      },
      onTerminal: (event) => {
        if (!alive) return;
        const status = event === "run.succeeded" ? "succeeded" : event === "run.failed" ? "failed" : "cancelled";
        setRunStatus(status);
        void refreshAuthority(run.id).catch((error) => { if (alive) setRunError(error); });
      },
    });
    subscription.current = stream;
    return () => {
      alive = false;
      cursor.current = stream.getCursor();
      stream.unsubscribe();
      if (subscription.current === stream) subscription.current = null;
    };
  }, [run?.id, api, refreshAuthority]);

  useEffect(() => {
    if (!props.open || !run?.id || terminal(run.status)) return;
    let active = true;
    const reconcile = async () => {
      try {
        const current = await api.agent.getRun(run.id);
        if (!active) return;
        setRun(current);
        setRunStatus(current.status);
        if (terminal(current.status)) {
          if (current.status === "succeeded" || current.status === "completed") setArtifacts(await api.agent.listArtifacts(run.id));
        }
      } catch (error) {
        if (active) setStaleRun(true);
      }
    };
    const handleVisibility = () => { if (document.visibilityState === "visible") void reconcile(); };
    document.addEventListener("visibilitychange", handleVisibility);
    return () => { active = false; document.removeEventListener("visibilitychange", handleVisibility); };
  }, [props.open, run?.id, run?.status, api]);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!definition || !catalogReady || !props.chapterRevision || !legalKinds.includes(taskKind) || !taskKey.trim() || !instruction.trim()) return;
    setCreating(true); setRunError(null); setArtifacts([]); setStreamSnapshot(null); setRunStatus(null); setStaleRun(false); cursor.current = 0;
    try {
      const source = props.chapterText.replace(/\r\n?/g, "\n");
      const computedHash = await hashText(source);
      setSourceSnapshot(source);
      setSourceHash(computedHash);
      const resource = {
        resource_id: props.chapterId, resource_type: "chapter", source_revision: props.chapterRevision,
        novel_id: props.novelId, source_hash: computedHash,
        ...(props.initialSelection ? { selection: { start_offset: props.initialSelection.startOffset, end_offset: props.initialSelection.endOffset, source_hash: computedHash } } : {}),
      };
      const created = await api.agent.createRun({
        task_kind: taskKind, task_key: taskKey.trim(), resource, instruction: instruction.trim(),
        definition_id: definition.id, workflow_id: workflowId || undefined, model_id: modelId || undefined,
      });
      setRun(created); setRunStatus(created.status);
    } catch (error) {
      setRunError(error);
    } finally {
      setCreating(false);
    }
  };

  const cancelRun = async () => {
    if (!run || terminal(run.status) || cancelling || offline) return;
    setCancelling(true);
    setRunError(null);
    try {
      const current = await api.agent.getRun(run.id);
      const updated = await api.agent.cancelRun(run.id, { expected_revision: current.revision });
      setRun(updated);
      setRunStatus(updated.status);
    } catch (error) {
      setRunError(error);
    } finally {
      setCancelling(false);
    }
  };

  const startNew = () => {
    subscription.current?.unsubscribe();
    subscription.current = null; cursor.current = 0;
    setRun(null); setRunStatus(null); setStreamSnapshot(null); setRunError(null); setArtifacts([]); setStaleRun(false);
  };

  const applyArtifact = (artifact: ArtifactView) => {
    if (artifact.is_partial || !artifact.text_content || artifact.target_resource_id !== props.chapterId || artifact.source_revision !== props.chapterRevision) return;
    setReview(artifact);
  };

  if (!props.open) return null;
  return (
    <div className="fixed inset-0 z-40 flex justify-end bg-black/40" role="presentation">
      <section className="h-full w-full max-w-2xl overflow-y-auto border-l border-[var(--border-decorative)] bg-[var(--bg-surface)] p-5 text-[var(--text-primary)] shadow-2xl" role="dialog" aria-modal="true" aria-labelledby="agent-workbench-title">
        <header className="mb-5 flex items-start justify-between border-b border-[var(--border-decorative)] pb-4">
          <div><h2 id="agent-workbench-title" className="text-lg font-semibold">{t("agentWorkbench.title")}</h2><p className="mt-1 text-sm text-[var(--text-secondary)]">{t("agentWorkbench.description")}</p></div>
          <button type="button" onClick={props.onClose} aria-label={t("common.close")} className="rounded p-2 hover:bg-[var(--bg-subtle)]"><X size={18} /></button>
        </header>
        {offline && <div className="mb-3 rounded border border-[var(--status-warning)] p-3 text-sm" role="status">{t("agentWorkbench.offline")}</div>}
        {catalogError !== null && <><ErrorPresenter error={catalogError} context={{ operationLabel: t("agentWorkbench.catalog") }} onRetry={() => void loadCatalog()} /><p role="status" className="text-xs text-[var(--text-secondary)]">{t(catalog.definitions.length || catalog.workflows.length ? "agentWorkbench.catalogPartial" : "agentWorkbench.catalogUnavailable")}</p></>}
        {catalogLoading && <div className="py-5 text-sm" aria-busy="true">{t("agentWorkbench.loadingCatalog")}</div>}
        {!catalogLoading && !catalogError && catalog.definitions.length === 0 && <div className="rounded border border-dashed p-5 text-sm">{t("agentWorkbench.emptyCatalog")}</div>}
        {!run && catalogReady && !catalogLoading && catalog.definitions.length > 0 && (
          <form onSubmit={(event) => void submit(event)} className="space-y-4">
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.definition")}</span><select className="w-full rounded border bg-[var(--bg-canvas)] p-2" value={definitionId} onChange={(event) => { setDefinitionId(event.target.value); setTaskKind(""); }} required>{catalog.definitions.map((item) => <option value={item.id} key={item.id}>{item.title} · {item.version_id}</option>)}</select></label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.taskKind")}</span><select className="w-full rounded border bg-[var(--bg-canvas)] p-2" value={taskKind} onChange={(event) => setTaskKind(event.target.value)} required><option value="">{t("common.select")}</option>{legalKinds.map((kind) => <option value={kind} key={kind}>{kind}</option>)}</select>{legalKinds.length === 0 && <span className="text-xs text-[var(--status-warning)]">{t("agentWorkbench.unsupportedTask")}</span>}</label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.taskKey")}</span><input className="w-full rounded border bg-[var(--bg-canvas)] p-2 font-mono" value={taskKey} onChange={(event) => setTaskKey(event.target.value)} pattern="[a-z][a-z0-9_]{0,63}" maxLength={64} required /><span className="text-xs text-[var(--text-secondary)]">{t("agentWorkbench.taskKeyHelp")}</span></label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.workflow")}</span><select className="w-full rounded border bg-[var(--bg-canvas)] p-2" value={workflowId} onChange={(event) => setWorkflowId(event.target.value)}><option value="">{t("agentWorkbench.noWorkflow")}</option>{catalog.workflows.map((item) => <option value={item.id} key={item.id}>{item.title} · {item.version_id}</option>)}</select></label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.model")}</span><select className="w-full rounded border bg-[var(--bg-canvas)] p-2" value={modelId} onChange={(event) => setModelId(event.target.value)}><option value="">{t("editor.modelDefault")}</option>{props.models.map((model) => <option value={model.id} key={model.id}>{model.name}</option>)}</select></label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.instruction")}</span><textarea className="min-h-28 w-full rounded border bg-[var(--bg-canvas)] p-2" value={instruction} onChange={(event) => setInstruction(event.target.value)} maxLength={8000} required /></label>
            {props.initialSelection && <div className="rounded border border-[var(--border-decorative)] p-3 text-xs"><div className="font-medium">{t("agentWorkbench.selectionBound")}</div><div className="mt-1 max-h-24 overflow-auto whitespace-pre-wrap">{props.initialSelection.text}</div></div>}
            {runError !== null && <ErrorPresenter error={runError} context={{ operationLabel: t("agentWorkbench.createRun") }} />}
<Button type="submit" variant="primary" disabled={creating || offline || !taskKind || legalKinds.length === 0}>{creating ? t("agentWorkbench.creating") : t("agentWorkbench.createRun")}</Button>
          </form>
        )}
        {run && <div className="space-y-4">
          {staleRun && <div role="status" className="rounded border border-[var(--status-warning)] p-3 text-sm">{t("agentWorkbench.stale")}</div>}
          {streamState === "reconnecting" && <div role="status" className="rounded border border-[var(--border-decorative)] p-3 text-sm">{t("agentWorkbench.reconnecting")}</div>}
          {streamState === "closed" && !terminal(runStatus || run.status) && <div role="status" className="rounded border border-[var(--status-warning)] p-3 text-sm">{t("agentWorkbench.streamEnded")}</div>}
          {runError !== null && <ErrorPresenter error={runError} context={{ operationLabel: t("agentWorkbench.run") }} onRetry={() => void refreshAuthority(run.id).then(() => setRunError(null)).catch(setRunError)} />}
          {streamSnapshot && <RunInspector mode="owner" runId={run.id} status={runStatus === "succeeded" || runStatus === "failed" || runStatus === "cancelled" ? runStatus : runStatus === "pending" ? "pending" : "running"} textBuffer={streamSnapshot.textBuffer} reasoningSummary={streamSnapshot.reasoningSummary} nodes={Array.from(streamSnapshot.nodeStates.values())} usage={streamSnapshot.usage} />}
          {streamSnapshot?.reasoningSummary && <details className="rounded border p-3"><summary className="cursor-pointer text-sm">{t("agentWorkbench.reasoningSummary")}</summary><p className="mt-2 whitespace-pre-wrap text-sm">{streamSnapshot.reasoningSummary}</p></details>}
          {streamSnapshot && <div className="space-y-2 text-sm">
            {Array.from(streamSnapshot.activeTools.entries()).map(([id, item]) => <div key={id} className="rounded border p-2">{t("agentWorkbench.toolRunning", item.toolName)}</div>)}
            {streamSnapshot.completedTools.map((item) => <div key={item.tool_call_id} className="rounded border p-2">{t("agentWorkbench.toolCompleted", item.tool_name, t(`agentWorkbench.outcome.${item.outcome}`))}</div>)}
            {streamSnapshot.activeApproval && <div className="rounded border border-[var(--status-warning)] p-3"><AlertTriangle size={16} className="mr-2 inline" />{t("agentWorkbench.approvalRequired", streamSnapshot.activeApproval.tool_name)}</div>}
          </div>}
          {streamSnapshot?.isTruncated && <div className="text-xs text-[var(--text-muted)]">{t("agentWorkbench.partialStream")}</div>}
          {runStatus && <div className="rounded border p-3 text-sm"><Bot size={16} className="mr-2 inline" />{t("agentWorkbench.runStatus", runStatus)}{streamSnapshot?.lastSeq ? ` · ${t("agentWorkbench.lastSeq", streamSnapshot.lastSeq)}` : ""}{run.error_code ? ` · ${t("agentWorkbench.errorCode", run.error_code)}` : ""}</div>}
          {!terminal(runStatus || run.status) && <Button variant="outline" disabled={cancelling || offline} onClick={() => void cancelRun()}>{cancelling ? t("agentWorkbench.cancelling") : t("agentWorkbench.cancelRun")}</Button>}
          {artifacts.length > 0 && <section className="space-y-2"><h3 className="font-medium">{t("agentWorkbench.artifacts")}</h3>{artifacts.map((artifact) => <article className="rounded border p-3" key={artifact.id}><div className="flex flex-wrap items-center justify-between gap-2"><strong>{artifact.kind}</strong><span className="text-xs">{artifact.is_partial ? t("agentWorkbench.partial") : t("agentWorkbench.complete")}</span></div><div className="mt-2 grid gap-1 text-xs text-[var(--text-secondary)]"><span>{t("agentWorkbench.sourceRevision", artifact.source_revision)}</span><span>{t("agentWorkbench.artifactHash", artifact.content_hash)}</span><span>{t("agentWorkbench.target", artifact.target_resource_id || t("common.unknown"))}</span></div><pre className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap text-sm">{artifact.text_content || ""}</pre><Button variant="outline" disabled={artifact.is_partial || !artifact.text_content || artifact.target_resource_id !== props.chapterId || artifact.source_revision !== props.chapterRevision} onClick={() => applyArtifact(artifact)}>{t("agentWorkbench.reviewAdopt")}</Button></article>)}</section>}
          {artifacts.length === 0 && (runStatus === "succeeded" || runStatus === "completed") && <p role="status" className="text-sm">{t("agentWorkbench.emptyArtifacts")}</p>}
          {terminal(runStatus || run.status) && <Button variant="outline" onClick={startNew} icon={<RefreshCw size={16} />}>{t("agentWorkbench.newRun")}</Button>}
        </div>}
        {review && <ArtifactReviewModal open originalText={props.initialSelection?.text || sourceSnapshot} suggestedText={review.text_content || ""} actionTitle={t("agentWorkbench.artifactReview")} sourceRevision={review.source_revision} sourceHash={sourceHash} artifactHash={review.content_hash} partial={review.is_partial} onClose={() => setReview(null)} onAdopt={() => { props.onAdopt(review, props.initialSelection, sourceSnapshot); setReview(null); }} />}
      </section>
    </div>
  );
}
