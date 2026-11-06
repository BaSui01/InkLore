import { NetworkError, isValidRevision } from "@inklore/contracts";
import { adminClient, type LlmModel, type PagedItems } from "./adminApi";

export interface AgentPolicyInput {
  task_key: string;
  task_kinds: string[];
  title: string;
  enabled: boolean;
  primary_model_id: string;
  fallback_model_ids: string[];
  max_output_tokens: number;
  temperature: number;
  timeout_seconds: number;
  max_retries: number;
  required_capabilities: string[];
  output_mode: "text" | "structured" | "embedding";
}
export interface AgentPolicy extends AgentPolicyInput {
  id: string;
  revision: number;
  current_version_id: string;
  version: number;
  created_at: string;
  updated_at: string;
}
function policy(value: AgentPolicy): AgentPolicy {
  if (!value || typeof value.id !== "string" || !isValidRevision(value.revision) || typeof value.current_version_id !== "string" ||
      !Number.isInteger(value.version) || value.version < 1 || typeof value.task_key !== "string" || typeof value.title !== "string" ||
      !Array.isArray(value.task_kinds) || !Array.isArray(value.fallback_model_ids) || !Array.isArray(value.required_capabilities) ||
      !Number.isInteger(value.max_output_tokens) || !Number.isInteger(value.timeout_seconds)) throw new NetworkError("Incomplete policy version response");
  return value;
}
async function pages<T>(path: string): Promise<T[]> {
  const items: T[] = []; const seen = new Set<string>(); let cursor: string | undefined;
  do {
    const { data } = await adminClient.request<unknown, PagedItems<T>>({ operationId: `admin_${path.split("/").at(-1)}_list`, method: "GET", path, query: { limit: 100, cursor } });
    if (!data || !Array.isArray(data.items) || typeof data.has_more !== "boolean") throw new NetworkError("Incomplete catalog page");
    items.push(...data.items); cursor = data.has_more ? data.next_cursor ?? undefined : undefined;
    if (data.has_more && (!cursor || seen.has(cursor))) throw new NetworkError("Invalid catalog cursor");
    if (cursor) seen.add(cursor);
  } while (cursor);
  return items;
}
export async function listAgentPolicies() { return (await pages<AgentPolicy>("/v1/llm/task-policies")).map(policy); }
export async function listPolicyModels() { return pages<LlmModel>("/v1/llm/models"); }
export async function getAgentPolicy(id: string) {
  const { data } = await adminClient.request<unknown, AgentPolicy>({ operationId: "admin_get_task_policy", method: "GET", path: "/v1/llm/task-policies/:policy_id", pathParams: { policy_id: id } });
  const value = policy(data); if (value.id !== id) throw new NetworkError("Unexpected policy response"); return value;
}
export async function publishAgentPolicy(id: string | null, input: AgentPolicyInput & { expected_revision?: number }, key: string) {
  const { data } = await adminClient.request<typeof input, AgentPolicy>({ operationId: id ? "admin_update_task_policy" : "admin_create_task_policy", method: id ? "PUT" : "POST", path: id ? "/v1/llm/task-policies/:policy_id" : "/v1/llm/task-policies", ...(id ? { pathParams: { policy_id: id } } : {}), body: input, idempotencyKey: key });
  const value = policy(data);
  if (id && value.id !== id) throw new NetworkError("Unexpected published policy");
  return value;
}
