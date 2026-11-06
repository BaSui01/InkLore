import React, { useCallback, useEffect, useRef, useState } from "react";
import { Bot, RefreshCw, X } from "lucide-react";
import { type ArtifactView, type CatalogDefinition, type CatalogWorkflow, type CatalogTaskPolicy, type CreateRunInput, type LlmModel, type AgentRunSummary, NetworkError, isApiError, isNetworkError } from "@inklore/contracts";
import { useTranslation } from "@inklore/i18n";
import { Button, ErrorPresenter, useImeComposition } from "@inklore/ui";
import { getApiSDK } from "@inklore/transport";
import { QuestionCard, RunInspector, RunApprovals } from "@inklore/run-ui";
import { ArtifactReviewModal } from "./ArtifactReviewModal";
import { terminalRun, useAgentRun } from "./useAgentRun";

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
  onPendingChange?: (pending: boolean) => void;
  onAdopt: (artifact: ArtifactView, selection: AgentWorkbenchProps["initialSelection"], sourceSnapshot: string) => void;
}

type CatalogState = { definitions: CatalogDefinition[]; workflows: CatalogWorkflow[]; policies: CatalogTaskPolicy[] };
const emptyCatalog: CatalogState = { definitions: [], workflows: [], policies: [] };
const hashText = async (value: string) => {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value.replace(/\r\n?/g, "\n")));
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
};

export function AgentWorkbench(props: AgentWorkbenchProps) {
  const { t } = useTranslation();
  const api = getApiSDK();
  const state = useAgentRun(props.open, props.novelId, props.chapterId);
  const { run, status, snapshot, artifacts } = state;
  const [catalog, setCatalog] = useState<CatalogState>(emptyCatalog);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogError, setCatalogError] = useState<unknown>(null);
  const [catalogReady, setCatalogReady] = useState(false);
  const [definitionId, setDefinitionId] = useState("");
  const [workflowId, setWorkflowId] = useState("");
  const [policyVersionId, setPolicyVersionId] = useState("");
  const [taskKind, setTaskKind] = useState("");
  const [instruction, setInstruction] = useState("");
  const [draftTouched, setDraftTouched] = useState(false);
  const [creating, setCreating] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [history, setHistory] = useState<AgentRunSummary[]>([]);
  const [historyChecked, setHistoryChecked] = useState(false);
  const [recoveringCreate, setRecoveringCreate] = useState(() => new URL(window.location.href).searchParams.has("agentCreate"));
  const [review, setReview] = useState<ArtifactView | null>(null);
  const [sourceSnapshot, setSourceSnapshot] = useState("");
  const [sourceHash, setSourceHash] = useState("");
  const [verifiedRunId, setVerifiedRunId] = useState<string | null>(null);
  const [scopeUnknown, setScopeUnknown] = useState(false);
  const [sourceUnavailable, setSourceUnavailable] = useState(false);
  const [sourceRefresh, setSourceRefresh] = useState(0);
  const [sourceSelection, setSourceSelection] = useState<AgentWorkbenchProps["initialSelection"]>(null);
  const [offline, setOffline] = useState(!navigator.onLine);
  const createIntent = useRef<{ key: string; body: CreateRunInput } | null>(null);
  const cancelIntent = useRef<{ key: string; revision: number } | null>(null);
  const createLock = useRef(false);
  const cancelLock = useRef(false);
  useEffect(() => {
    props.onPendingChange?.(recoveringCreate || uncertain || draftTouched || !!createIntent.current || state.restoring || creating || (!!run && !terminalRun(status ?? run.status)));
  }, [props.onPendingChange, recoveringCreate, uncertain, draftTouched, state.restoring, creating, run?.id, run?.status, status]);
  const catalogGeneration = useRef(0);
  const panel = useRef<HTMLElement>(null);
  const ime = useImeComposition();

  const loadCatalog = useCallback(async () => {
    const generation = ++catalogGeneration.current;
    setCatalogLoading(true); setCatalogError(null); setCatalogReady(false);
    try {
      const [definitions, workflows, policies] = await Promise.all([api.agent.listCatalogDefinitions(), api.agent.listCatalogWorkflows(), api.agent.listCatalogTaskPolicies()]);
      if (generation !== catalogGeneration.current) return;
      setCatalog({ definitions, workflows, policies }); setCatalogReady(true);
      setDefinitionId(current => definitions.some(item => item.id === current) ? current : definitions[0]?.id ?? "");
    } catch (cause) {
      if (generation !== catalogGeneration.current) return;
      setCatalogError(cause);
      if (isApiError(cause) && [401, 403].includes(cause.httpStatus)) state.handleError(cause);
    } finally { if (generation === catalogGeneration.current) setCatalogLoading(false); }
  }, [api, state.handleError]);
  useEffect(() => { if (props.open && !state.denied) void loadCatalog(); return () => { catalogGeneration.current++; }; }, [props.open, state.denied, loadCatalog]);
  useEffect(() => {
    const update = () => setOffline(!navigator.onLine);
    window.addEventListener("online", update); window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);
  useEffect(() => {
    if (state.denied) { setCatalog(emptyCatalog); setCatalogReady(false); setInstruction(""); setDraftTouched(false); setSourceSnapshot(""); setSourceHash(""); setSourceSelection(null); setVerifiedRunId(null); setScopeUnknown(false); setReview(null); createIntent.current = null; cancelIntent.current = null; setHistory([]); setUncertain(false); }
  }, [state.denied]);
  useEffect(() => {
    if (!props.open) return;
    const previous = document.activeElement as HTMLElement | null;
    panel.current?.focus();
    return () => previous?.focus();
  }, [props.open]);

  const definition = catalog.definitions.find(item => item.id === definitionId);
  const policies = catalog.policies.filter(item => item.task_kinds.some(kind => ["chapter_draft", "chapter_rewrite"].includes(kind) && definition?.task_kinds.includes(kind)));
  const policy = policies.find(item => item.policy_version_id === policyVersionId);
  const legalKinds = policy?.task_kinds.filter(kind => ["chapter_draft", "chapter_rewrite"].includes(kind) && definition?.task_kinds.includes(kind)) ?? [];
  const restoreHistory = useCallback(async () => {
    setHistoryChecked(false);
    try {
      let cursor: string | undefined;
      const items: AgentRunSummary[] = [];
      const seen = new Set<string>();
      do {
        const page = await api.agent.listRuns(cursor);
        items.push(...page.items.filter(item => item.resource?.resource_id === props.chapterId && item.resource?.novel_id === props.novelId));
        cursor = page.has_more ? page.next_cursor ?? undefined : undefined;
        if (cursor && seen.has(cursor)) throw new NetworkError("Run history cursor repeated");
        if (cursor) seen.add(cursor);
      } while (cursor && items.length < 20);
      setHistory(items);
      setHistoryChecked(true);
    } catch (cause) { state.handleError(cause); }
  }, [api, props.chapterId, props.novelId, state.handleError]);
  useEffect(() => { if (props.open && recoveringCreate && !run && !state.denied && props.chapterId) void restoreHistory(); }, [props.open, recoveringCreate, run, state.denied, props.chapterId, restoreHistory]);

  const clearCreatePointer = () => { const url = new URL(window.location.href); url.searchParams.delete("agentCreate"); window.history.replaceState(window.history.state, "", url); setRecoveringCreate(false); };
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (createLock.current || offline || state.denied || ime.isComposingRef.current || ime.justEndedRef.current) return;
    if (!createIntent.current && (!definition || !policy || !catalogReady || !props.chapterRevision || !legalKinds.includes(taskKind) || !instruction.trim() || [...instruction.trim()].length > 8000)) return;
    createLock.current = true; setCreating(true); state.setError(null);
    try {
      if (!createIntent.current) {
        const source = props.chapterText.replace(/\r\n?/g, "\n");
        const hash = await hashText(source);
        setSourceSnapshot(source); setSourceHash(hash); setSourceSelection(props.initialSelection);
        createIntent.current = { key: crypto.randomUUID(), body: {
          task_kind: taskKind, task_policy_version_id: policy!.policy_version_id,
          resource: { resource_id: props.chapterId, resource_type: "chapter", source_revision: props.chapterRevision!, novel_id: props.novelId, source_hash: hash,
            ...(props.initialSelection ? { selection: { start_offset: props.initialSelection.startOffset, end_offset: props.initialSelection.endOffset, source_hash: hash } } : {}) },
          instruction: instruction.trim(), definition_id: definition!.id, ...(workflowId ? { workflow_id: workflowId } : {}),
        } };
      }
      const url = new URL(window.location.href); url.searchParams.set("agentCreate", createIntent.current.key); url.searchParams.set("agentChapter", props.chapterId); window.history.replaceState(window.history.state, "", url);
      const created = await api.agent.createRun(createIntent.current.body, createIntent.current.key);
      setVerifiedRunId(created.id); setScopeUnknown(false);
      state.adopt(created); createIntent.current = null; setUncertain(false); setDraftTouched(false); clearCreatePointer();
    } catch (cause) {
      const unknown = isNetworkError(cause) || (isApiError(cause) && (cause.httpStatus >= 500 || cause.code === "platform.operation_in_progress"));
      setUncertain(unknown);
      if (!unknown) { createIntent.current = null; clearCreatePointer(); }
      state.handleError(cause);
      if (isApiError(cause) && cause.code === "agent.task_policy_version_changed") void loadCatalog();
    } finally { createLock.current = false; setCreating(false); }
  };
  const cancel = async () => {
    if (!run || terminalRun(run.status) || cancelLock.current || offline) return;
    cancelLock.current = true; setCancelling(true);
    try {
      const current = await state.refresh();
      if (!current || terminalRun(current.status)) return;
      if (!cancelIntent.current) cancelIntent.current = { key: crypto.randomUUID(), revision: current.revision };
      state.adopt(await api.agent.cancelRun(run.id, { expected_revision: cancelIntent.current.revision }, cancelIntent.current.key));
      cancelIntent.current = null;
    } catch (cause) {
      state.handleError(cause);
      if (isApiError(cause) && cause.httpStatus === 409 && cause.code !== "platform.operation_in_progress") { cancelIntent.current = null; void state.refresh().catch(() => {}); }
    } finally { cancelLock.current = false; setCancelling(false); }
  };
  useEffect(() => {
    if (!run?.resource || (verifiedRunId === run.id && sourceHash) || props.chapterRevision === null) return;
    if (run.resource.source_revision !== props.chapterRevision) {
      setSourceHash(""); setVerifiedRunId(null); setSourceUnavailable(true); return;
    }
    let alive = true;
    void (async () => {
      try {
        const value = await api.content.getChapterBody(props.chapterId);
        const body = value.body.replace(/\r\n?/g, "\n");
        const digest = await hashText(body);
        if (!alive) return;
        const selected = run.resource!.selection;
        const codepoints = Array.from(body);
        if (value.chapter.revision !== run.resource!.source_revision ||
            run.resource!.source_hash !== digest ||
            (selected && (selected.source_hash !== digest || !Number.isSafeInteger(selected.start_offset) ||
              !Number.isSafeInteger(selected.end_offset) || selected.start_offset < 0 ||
              selected.end_offset <= selected.start_offset || selected.end_offset > codepoints.length))) {
          setSourceHash(""); setVerifiedRunId(null); setSourceUnavailable(true); return;
        }
        // Historical runs without a persisted selection may have been partial
        // rewrites. Never treat missing scope as whole-chapter consent.
        setScopeUnknown(!selected && !run.resource!.selection_scope_verified);
        setSourceUnavailable(false); setSourceSnapshot(body); setSourceHash(digest); setVerifiedRunId(run.id);
        setSourceSelection(selected ? {
          startOffset: selected.start_offset, endOffset: selected.end_offset,
          text: codepoints.slice(selected.start_offset, selected.end_offset).join(""),
        } : null);
      } catch (cause) {
        if (alive) { setSourceHash(""); setVerifiedRunId(null); setSourceUnavailable(true); state.handleError(cause); }
      }
    })();
    return () => { alive = false; };
  }, [run?.id, props.chapterId, props.chapterRevision, api, sourceHash, sourceRefresh, verifiedRunId, state.handleError]);

  const selectClass = "w-full min-w-0 rounded border bg-[var(--bg-canvas)] p-2";
  // Keep the owner question subtree mounted while consulting the chapter so
  // drafts and unknown-response idempotency intents remain in memory only.
  return <div hidden={!props.open} className={props.open ? "fixed inset-0 z-40 flex justify-end bg-black/40" : "hidden"} role="presentation">
    <section ref={panel} tabIndex={-1} className="h-full w-full min-w-0 max-w-2xl overflow-y-auto border-l border-[var(--border-decorative)] bg-[var(--bg-surface)] p-5 text-[var(--text-primary)] shadow-2xl" role="dialog" aria-modal="true" aria-labelledby="agent-workbench-title" onKeyDown={event => {
      if (event.key === "Escape" && !ime.shouldBlockKeyAction(event)) props.onClose();
      if (event.key === "Tab") {
        const items = Array.from(panel.current?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]') ?? []).filter(item => item.offsetParent !== null);
        if (items.length && event.shiftKey && (document.activeElement === items[0] || document.activeElement === panel.current)) { event.preventDefault(); items[items.length - 1].focus(); }
        else if (items.length && !event.shiftKey && document.activeElement === items[items.length - 1]) { event.preventDefault(); items[0].focus(); }
      }
    }}>
      <header className="mb-5 flex items-start justify-between gap-3 border-b border-[var(--border-decorative)] pb-4"><div><h2 id="agent-workbench-title" className="text-lg font-semibold">{t("agentWorkbench.title")}</h2><p className="mt-1 text-sm text-[var(--text-secondary)]">{t("agentWorkbench.description")}</p></div><button type="button" onClick={props.onClose} aria-label={t("common.close")} className="rounded p-2"><X size={18} /></button></header>
      {offline && <p className="mb-3 rounded border p-3 text-sm" role="status">{t("agentWorkbench.offline")}</p>}
      {state.denied && <p role="alert">{t("agentQuestion.denied")}</p>}
      {state.restoring && <p role="status">{t("agentQuestion.restoring")}</p>}
      {state.error != null && <ErrorPresenter error={state.error} onRetry={run || state.restoreId ? () => void state.refresh().catch(() => {}) : undefined} />}
      {!run && !state.denied && !state.restoring && recoveringCreate && <section className="space-y-3"><p>{t("agentQuestion.restoreCreate")}</p><Button disabled={offline} onClick={() => void restoreHistory()}>{t("common.refresh")}</Button>{history.map(item => <Button key={item.id} variant="outline" onClick={() => void state.refresh(item.id).then(value => { if (value) clearCreatePointer(); }).catch(() => {})}>{t("agentQuestion.resume", item.id, t(`agentState.${item.status}`))}</Button>)}<p className="text-sm">{t("agentQuestion.abandonCreateNotice")}</p><Button variant="outline" disabled={!historyChecked || offline} onClick={() => { createIntent.current = null; setUncertain(false); state.setError(null); clearCreatePointer(); }}>{t("agentQuestion.abandonCreate")}</Button></section>}
      {!run && !state.denied && !state.restoring && !state.restoreId && !recoveringCreate && <>
        {catalogError != null && <ErrorPresenter error={catalogError} onRetry={() => void loadCatalog()} />}
        {catalogLoading && <p role="status">{t("agentWorkbench.loadingCatalog")}</p>}
        {catalogReady && !catalogLoading && (catalog.definitions.length === 0 || catalog.policies.length === 0) && <p>{t("agentQuestion.emptyPolicies")}</p>}
        {catalogReady && !catalogLoading && catalog.definitions.length > 0 && catalog.policies.length > 0 && <form onSubmit={submit} className="space-y-4" onKeyDown={event => { if (event.key === "Enter" && ime.shouldBlockKeyAction(event)) { event.preventDefault(); event.stopPropagation(); } }}>
          <fieldset disabled={creating || uncertain} className="space-y-4 min-w-0">
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.definition")}</span><select className={selectClass} value={definitionId} onChange={event => { setDefinitionId(event.target.value); setPolicyVersionId(""); setTaskKind(""); setDraftTouched(true); }} required>{catalog.definitions.map(item => <option value={item.id} key={item.id}>{item.title} · {item.version_id}</option>)}</select></label>
            <label className="block space-y-1 text-sm"><span>{t("agentQuestion.policy")}</span><select className={selectClass} value={policyVersionId} onChange={event => { setPolicyVersionId(event.target.value); setTaskKind(""); setDraftTouched(true); }} required><option value="">{t("common.select")}</option>{policies.map(item => <option value={item.policy_version_id} key={item.policy_version_id}>{item.title} · v{item.version}</option>)}</select></label>
            {policy && <p className="text-xs break-words">{t("agentQuestion.policyBudget", policy.version, policy.budget.max_output_tokens, policy.budget.timeout_seconds)}</p>}
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.taskKind")}</span><select className={selectClass} value={taskKind} onChange={event => { setTaskKind(event.target.value); setDraftTouched(true); }} required><option value="">{t("common.select")}</option>{legalKinds.map(kind => <option key={kind} value={kind}>{kind}</option>)}</select></label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.workflow")}</span><select className={selectClass} value={workflowId} onChange={event => { setWorkflowId(event.target.value); setDraftTouched(true); }}><option value="">{t("agentWorkbench.noWorkflow")}</option>{catalog.workflows.map(item => <option value={item.id} key={item.id}>{item.title} · {item.version_id}</option>)}</select></label>
            <label className="block space-y-1 text-sm"><span>{t("agentWorkbench.instruction")}</span><textarea className={selectClass+" min-h-28"} maxLength={16000} value={instruction} onChange={event => { setInstruction(event.target.value); setDraftTouched(true); }} onCompositionStart={ime.handleCompositionStart} onCompositionEnd={ime.handleCompositionEnd} required /></label>
          </fieldset>
          {[...instruction.trim()].length > 8000 && <p role="alert">{t("agentQuestion.instructionLimit")}</p>}
          {uncertain && <p role="status">{t("agentQuestion.createUnknown")}</p>}
          <div className="flex flex-wrap gap-2">
            <Button type="submit" loading={creating} disabled={offline || (!uncertain && (!taskKind || !policy))}>{t(uncertain ? "agentQuestion.retryCreate" : "agentWorkbench.createRun")}</Button>
            {draftTouched && <Button type="button" variant="outline" disabled={creating || uncertain} onClick={() => { createIntent.current = null; setInstruction(""); setDefinitionId(""); setWorkflowId(""); setPolicyVersionId(""); setTaskKind(""); setDraftTouched(false); }}>{t("common.cancel")}</Button>}
          </div>
        </form>}
      </>}
      {run && <div className="space-y-4">
        {scopeUnknown && <p role="alert">{t("agentQuestion.sourceUnavailable")}</p>}
        {sourceUnavailable && !sourceHash && <div role="alert" className="rounded border p-3 text-sm"><p>{t("agentQuestion.sourceUnavailable")}</p><Button variant="ghost" disabled={offline} onClick={() => setSourceRefresh(value => value + 1)}>{t("common.refresh")}</Button></div>}
        {state.stale && <p role="status">{t("agentWorkbench.stale")}</p>}
        {state.connection === "reconnecting" && <p role="status">{t("agentWorkbench.reconnecting")}</p>}
        <div className="rounded border p-3 text-sm break-words"><Bot size={16} className="mr-2 inline" />{t("agentWorkbench.runStatus", t(`agentState.${status ?? run.status}`))}<p className="mt-1 text-xs">{t("agentQuestion.frozenPolicy", run.policy_version_id)}</p></div>
        {run.pending_questions.map(question => <QuestionCard key={question.id} question={question} disabled={offline || run.status !== "waiting_clarification"} sourceChanged={question.source_revision !== props.chapterRevision} onRefresh={() => state.refresh()} onDenied={state.handleError} />)}
        {snapshot && <RunInspector mode="owner" runId={run.id} status={status ?? run.status} textBuffer={snapshot.textBuffer} reasoningSummary={snapshot.reasoningSummary} nodes={Array.from(snapshot.nodeStates.values())} usage={snapshot.usage} />}
        {run.status === "waiting_approval" && <RunApprovals runId={run.id} runRevision={run.revision} sourceRevision={props.chapterRevision} disabled={offline} onRefresh={() => state.refresh()} onDenied={state.handleError} />}
        {snapshot && Array.from(snapshot.activeTools).map(([id, tool]) => <p key={id} className="text-sm break-words">{t("agentWorkbench.toolRunning", tool.toolName)}</p>)}
        {snapshot?.completedTools.map(tool => <p key={tool.invocation_id} className="text-sm break-words">{t("agentWorkbench.toolCompleted", tool.invocation_id, t(tool.success ? "agentWorkbench.outcome.success" : "agentWorkbench.outcome.error"))}{tool.error_code && ` (${tool.error_code})`}</p>)}
        {snapshot?.isTruncated && <p>{t("agentWorkbench.partialStream")}</p>}
        {!terminalRun(status ?? run.status) && <Button variant="outline" loading={cancelling} disabled={offline} onClick={() => void cancel()}>{t("agentWorkbench.cancelRun")}</Button>}
        {status === "succeeded" && artifacts.length === 0 && <p>{t("agentWorkbench.emptyArtifacts")}</p>}
        {artifacts.map(artifact => <article key={artifact.id} className="rounded border p-3 space-y-2 break-words"><strong>{artifact.kind}</strong><p className="text-xs">{t("agentWorkbench.sourceRevision", artifact.source_revision)}</p><pre className="max-h-40 overflow-auto whitespace-pre-wrap text-sm">{artifact.text_content ?? ""}</pre><Button variant="outline" disabled={artifact.is_partial || !artifact.text_content || artifact.target_resource_id !== props.chapterId || artifact.source_revision !== props.chapterRevision || !sourceHash || verifiedRunId !== run.id || scopeUnknown} onClick={() => setReview(artifact)}>{t("agentWorkbench.reviewAdopt")}</Button></article>)}
        {terminalRun(status ?? run.status) && <Button variant="outline" icon={<RefreshCw size={16} />} onClick={() => { state.clear(); state.setError(null); setSourceHash(""); setSourceSnapshot(""); setSourceSelection(null); setVerifiedRunId(null); setScopeUnknown(false); cancelIntent.current = null; }}>{t("agentWorkbench.newRun")}</Button>}
      </div>}
      {review && !scopeUnknown && verifiedRunId === run?.id && <ArtifactReviewModal open={props.open} originalText={sourceSelection?.text ?? sourceSnapshot} suggestedText={review.text_content ?? ""} actionTitle={t("agentWorkbench.artifactReview")} sourceRevision={review.source_revision} sourceHash={sourceHash} artifactHash={review.content_hash} partial={review.is_partial} onClose={() => setReview(null)} onAdopt={() => { props.onAdopt(review, sourceSelection, sourceSnapshot); setReview(null); }} />}
    </section>
  </div>;
}
