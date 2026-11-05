import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, type AgentRun, type ArtifactView, isApiError } from "@inklore/contracts";
import { getApiSDK, StreamClient, RunEventBuffer, type AggregatedRunState, type ConnectionState, type StreamSubscription } from "@inklore/transport";

export const terminalRun = (status: AgentRun["status"]) => ["succeeded", "completed", "failed", "cancelled"].includes(status);

/** Only a resource ID is retained in the URL; prompts, answers and tokens stay out. */
function runPointer(id: string | null, chapterId?: string) {
  const url = new URL(window.location.href);
  if (id) { url.searchParams.set("agentRun", id); if (chapterId) url.searchParams.set("agentChapter", chapterId); } else { url.searchParams.delete("agentRun"); url.searchParams.delete("agentCreate"); url.searchParams.delete("agentChapter"); }
  window.history.replaceState(window.history.state, "", url);
}

export function useAgentRun(open: boolean, novelId: string, chapterId: string) {
  const api = getApiSDK();
  const [run, setRun] = useState<AgentRun | null>(null);
  const [status, setStatus] = useState<AgentRun["status"] | null>(null);
  const [snapshot, setSnapshot] = useState<AggregatedRunState | null>(null);
  const [artifacts, setArtifacts] = useState<ArtifactView[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [denied, setDenied] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [restoreId, setRestoreId] = useState<string | null>(() => new URL(window.location.href).searchParams.get("agentRun"));
  const [stale, setStale] = useState(false);
  const [connection, setConnection] = useState<ConnectionState>("idle");
  const [restart, setRestart] = useState(0);
  const active = useRef<AgentRun | null>(null);
  const ended = run ? terminalRun(run.status) : false;
  const subscription = useRef<StreamSubscription | null>(null);
  const cursor = useRef(0);
  const eventBuffer = useRef<RunEventBuffer | null>(null);
  const generation = useRef(0);
  const adopt = useCallback((value: AgentRun) => {
    if (!value.resource || value.resource.resource_id !== chapterId || value.resource.novel_id !== novelId) {
      throw new ApiError({ httpStatus: 403, code: "auth.forbidden" });
    }
    if (active.current?.id === value.id && value.revision < active.current.revision) return;
    if (active.current?.id !== value.id) { cursor.current = 0; eventBuffer.current = new RunEventBuffer(value.id); setSnapshot(null); setArtifacts([]); }
    active.current = value; setRun(value); setStatus(value.status); setStale(false); setError(null); setDenied(false); setRestoreId(value.id); runPointer(value.id, chapterId);
  }, [chapterId, novelId]);
  const clear = useCallback(() => {
    generation.current++; subscription.current?.getBuffer().clear(); subscription.current?.unsubscribe(); subscription.current = null;
    active.current = null; eventBuffer.current?.clear(); eventBuffer.current = null; cursor.current = 0; setRestoreId(null); setRun(null); setStatus(null); setSnapshot(null); setArtifacts([]); setStale(false); setRestoring(false); setConnection("closed"); runPointer(null);
  }, []);
  const handleError = useCallback((cause: unknown) => {
    if (isApiError(cause) && [401, 403, 404].includes(cause.httpStatus)) { clear(); setDenied(true); }
    else setStale(true);
    setError(cause);
  }, [clear]);
  const refresh = useCallback(async (id?: string) => {
    const runId = id ?? active.current?.id ?? new URL(window.location.href).searchParams.get("agentRun");
    if (!runId) return null;
    const currentGeneration = generation.current;
    try {
      const fresh = await api.agent.getRun(runId);
      if (currentGeneration !== generation.current) return null;
      adopt(fresh);
      if (fresh.status === "succeeded" || fresh.status === "completed") {
        const items = await api.agent.listArtifacts(runId);
        if (currentGeneration === generation.current && active.current?.id === runId) setArtifacts(items);
      }
      return fresh;
    } catch (cause) {
      if (currentGeneration === generation.current) handleError(cause);
      throw cause;
    }
  }, [api, adopt, handleError]);

  useEffect(() => {
    const session = api.getSession();
    const epoch = session?.getSessionEpoch();
    return session?.subscribe(value => { if (value.status !== "authenticated" || value.sessionEpoch !== epoch) { clear(); setDenied(true); } });
  }, [api, clear]);
  useEffect(() => () => { generation.current++; subscription.current?.unsubscribe(); }, []);
  useEffect(() => {
    if (!open || !chapterId || active.current || denied) return;
    const id = new URL(window.location.href).searchParams.get("agentRun");
    if (!id) return;
    let mounted = true; setRestoring(true);
    void refresh(id).catch(() => {}).finally(() => { if (mounted) setRestoring(false); });
    return () => { mounted = false; };
  }, [open, chapterId, denied, refresh]);

  useEffect(() => {
    if (!open || !run?.id || terminalRun(run.status)) return;
    const session = api.getSession();
    if (!session) return;
    let alive = true;
    const client = new StreamClient({ session });
    const stream = client.subscribe({
      runId: run.id, lastSeq: cursor.current || undefined, buffer: eventBuffer.current ?? undefined,
      onEvent: event => {
        if (!alive) return;
        setSnapshot(stream.getBuffer().getAggregatedState()); cursor.current = stream.getCursor();
        if (event.event === "run.started") setStatus("running");
        if (event.event === "approval.required") setStatus("waiting_approval");
        if (event.event === "question.required") setStatus("waiting_clarification");
        if (["question.required", "question.answered", "approval.required"].includes(event.event)) void refresh(run.id).catch(() => {});
      },
      onConnection: value => { if (alive) setConnection(value); },
      onError: cause => { if (alive) handleError(cause); },
      onCursorExpired: () => {
        if (!alive) return;
        setStale(true);
        void refresh(run.id).then(fresh => {
          if (alive && fresh && !terminalRun(fresh.status)) { cursor.current = 0; eventBuffer.current = new RunEventBuffer(fresh.id); eventBuffer.current.markTruncated(); setRestart(value => value + 1); }
        }).catch(() => {});
      },
      onTerminal: event => {
        if (!alive) return;
        setStatus(event === "run.succeeded" ? "succeeded" : event === "run.failed" ? "failed" : "cancelled");
        void refresh(run.id).catch(() => {});
      },
    });
    subscription.current = stream;
    return () => { alive = false; if (eventBuffer.current === stream.getBuffer()) cursor.current = stream.getCursor(); stream.unsubscribe(); if (subscription.current === stream) subscription.current = null; };
  }, [open, run?.id, ended, api, refresh, handleError, restart]);

  useEffect(() => {
    if (!open || !run?.id || terminalRun(run.status)) return;
    const reconcile = () => { if (navigator.onLine && document.visibilityState === "visible") void refresh(run.id).catch(() => {}); };
    const timer = setInterval(reconcile, 15_000);
    window.addEventListener("online", reconcile); document.addEventListener("visibilitychange", reconcile);
    return () => { clearInterval(timer); window.removeEventListener("online", reconcile); document.removeEventListener("visibilitychange", reconcile); };
  }, [open, run?.id, run?.status, refresh]);
  return { run, status, snapshot, artifacts, error, setError, denied, restoring, restoreId, stale, connection, adopt, refresh, handleError, clear };
}
