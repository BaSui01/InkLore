import { type AgentAuditRun, type PagedList, NetworkError, isValidRevision } from "@inklore/contracts";
import { adminClient } from "./adminApi";

// Reconstruct the public allowlist before React sees it. Never pass through raw
// run/event objects even if an upstream response unexpectedly gains fields.
function safeAudit(value: AgentAuditRun): AgentAuditRun {
  if (!value || typeof value.id !== "string" || !isValidRevision(value.revision) ||
      !["creator", "diagnostic"].includes(value.kind) || typeof value.task_kind !== "string" ||
      !["queued", "running", "waiting_approval", "waiting_clarification", "cancelling", "succeeded", "failed", "cancelled"].includes(value.status) ||
      !Number.isSafeInteger(value.node_count) || value.node_count < 0 || value.node_count > 64 ||
      !Number.isSafeInteger(value.duration_ms) || value.duration_ms < 0 ||
      (value.cost_micros !== null && (!Number.isSafeInteger(value.cost_micros) || value.cost_micros < 0)) ||
      (value.error_code !== null && typeof value.error_code !== "string")) throw new NetworkError("Incomplete run audit response");
  return { id: value.id, revision: value.revision, created_at: value.created_at, updated_at: value.updated_at,
    kind: value.kind, task_kind: value.task_kind, status: value.status, node_count: value.node_count,
    duration_ms: value.duration_ms, cost_micros: value.cost_micros, error_code: value.error_code };
}

export async function listAgentAuditRuns(cursor?: string): Promise<PagedList<AgentAuditRun>> {
  const { data } = await adminClient.request<unknown, PagedList<AgentAuditRun>>({ operationId: "agent_list_audit_runs", method: "GET", path: "/v1/agents/audit/runs", query: { limit: 20, cursor } });
  if (!data || !Array.isArray(data.items) || typeof data.has_more !== "boolean" || (data.has_more && (!data.next_cursor || data.next_cursor === cursor))) throw new NetworkError("Incomplete run audit page");
  return { items: data.items.map(safeAudit), has_more: data.has_more, next_cursor: data.next_cursor };
}

export async function getAgentAuditRun(id: string): Promise<AgentAuditRun> {
  const { data } = await adminClient.request<unknown, AgentAuditRun>({ operationId: "agent_get_audit_run", method: "GET", path: "/v1/agents/audit/runs/:run_id", pathParams: { run_id: id } });
  const safe = safeAudit(data);
  if (safe.id !== id) throw new NetworkError("Unexpected run audit response");
  return safe;
}
