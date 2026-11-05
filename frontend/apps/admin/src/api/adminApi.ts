/**
 * @inklore/admin-console - 管理端统一 API 服务层
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 * 所有展示数据全部从真实后端 REST API 获取
 */

import { AuthSession, ApiClient } from "@inklore/transport";
import { PlatformTenantSummary } from "@inklore/contracts";

// 全局单例管理端 Session 与 Client
export const adminSession = new AuthSession({ audience: "admin" });
export const adminClient = new ApiClient({ session: adminSession });

export async function fetchAdminTenants(): Promise<PlatformTenantSummary[]> {
  const result = await adminClient.request<undefined, PlatformTenantSummary[]>({
    operationId: "platform_listTenants",
    method: "GET",
    path: "/v1/platform/tenants",
  });
  return result.data;
}

// --- 类型契约定义 ---

export interface LlmProvider {
  id: string;
  revision: number;
  code: string;
  title: string;
  protocol: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface LlmModel {
  id: string;
  revision: number;
  provider_id: string;
  upstream_name: string;
  title: string;
  capabilities: string[];
  context_window: number;
  max_output_tokens: number;
  embedding_dimensions?: number | null;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChannelModelBinding {
  id: string;
  channel_id: string;
  model_id: string;
  upstream_model: string;
  priority: number;
  weight: number;
  enabled: boolean;
}

export interface ChannelModelBindingInput {
  model_id: string;
  upstream_model?: string;
  priority?: number;
  weight?: number;
  enabled?: boolean;
}

export interface LlmChannel {
  id: string;
  revision: number;
  provider_id: string;
  model_id?: string;
  model_ids?: string[];
  models?: ChannelModelBinding[];
  title: string;
  base_url: string;
  upstream_model?: string;
  priority: number;
  weight: number;
  rpm: number;
  concurrency: number;
  enabled: boolean;
  breaker_state: string;
  inflight: number;
  remaining_rpm: number;
  capacity_observed_at?: string | null;
  capacity_available: boolean;
  created_at: string;
  updated_at: string;
}

export interface SecretRef {
  kind: "env" | "secret_store" | "plain";
  locator: string;
  version: string;
}

export interface LlmKey {
  id: string;
  revision: number;
  channel_id: string;
  title: string;
  secret_ref: SecretRef;
  weight: number;
  rpm: number;
  concurrency: number;
  enabled: boolean;
  fingerprint?: string | null;
  health: string;
  created_at: string;
  updated_at: string;
}

export interface LlmBudget {
  id: string;
  revision: number;
  daily_limit_micros: number;
  business_date: string;
  spent_micros: number;
  reserved_micros: number;
  remaining_micros: number;
  over_budget: boolean;
  currency: string;
  created_at: string;
  updated_at: string;
}

export interface LlmUsage {
  id: string;
  invocation_id: string;
  attempt: number;
  reservation_id: string;
  policy_version_id: string;
  model_id: string;
  channel_id?: string | null;
  key_id?: string | null;
  business_date: string;
  input_tokens: number;
  output_tokens: number;
  cached_input_tokens: number;
  cost_micros: number;
  currency: string;
  status: string;
  error_code?: string | null;
  duration_ms: number;
  trace_id: string;
  created_at: string;
  updated_at: string;
}

export interface PagedItems<T> {
  items: T[];
  has_more: boolean;
  next_cursor?: string | null;
}

export interface AgentDefinitionView {
  id: string;
  revision: number;
  definition_key: string;
  title: string;
  enabled: boolean;
  system_instruction: string;
  tool_version_ids: string[];
  task_kinds: string[];
  current_version_id: string;
  version: number;
  publication_status: "draft" | "published";
  created_at: string;
  updated_at: string;
}

export interface AgentWorkflowView {
  id: string;
  revision: number;
  workflow_key: string;
  title: string;
  enabled: boolean;
  manifest: {
    nodes: Array<{
      node_key: string;
      role: string;
      depends_on: string[];
      required: boolean;
      definition_version_id?: string;
      tool_version_id?: string;
      input_bindings: Array<{
        target_parameter: string;
        source_node: string;
        source_pointer: string;
      }>;
    }>;
  };
  current_version_id: string;
  version: number;
  manifest_hash: string;
  created_at: string;
  updated_at: string;
}

export interface AgentBuiltinToolView {
  tool_key: string;
  title: string;
  description: string;
  execution_status: string;
  side_effect: string;
  input_schema: unknown;
  output_schema: unknown;
}

export interface AuditSafeChange {
  field: string;
  before?: string | null;
  after?: string | null;
}

export interface AuditEvent {
  id: string;
  actor_id?: string | null;
  audience: string;
  action: string;
  resource_id?: string | null;
  outcome: string;
  request_id: string;
  trace_id: string;
  safe_changes: AuditSafeChange[];
  created_at: string;
}

// --- 请求函数封装 ---

export async function fetchLlmBudget(): Promise<LlmBudget | null> {
  const res = await adminClient.request<undefined, LlmBudget>({
    operationId: "admin.llm.getBudget",
    method: "GET",
    path: "/v1/llm/budget",
  });
  return res.data;
}

export async function fetchLlmProviders(): Promise<LlmProvider[]> {
  const res = await adminClient.request<undefined, PagedItems<LlmProvider>>({
    operationId: "admin.llm.listProviders",
    method: "GET",
    path: "/v1/llm/providers",
    query: { limit: 100 },
  });
  return res.data.items;
}

export async function fetchLlmModels(): Promise<LlmModel[]> {
  const res = await adminClient.request<undefined, PagedItems<LlmModel>>({
    operationId: "admin.llm.listModels",
    method: "GET",
    path: "/v1/llm/models",
    query: { limit: 100 },
  });
  return res.data.items;
}

export async function fetchLlmChannels(): Promise<LlmChannel[]> {
  const res = await adminClient.request<undefined, PagedItems<LlmChannel>>({
    operationId: "admin.llm.listChannels",
    method: "GET",
    path: "/v1/llm/channels",
    query: { limit: 100 },
  });
  return res.data.items;
}

export async function fetchLlmKeys(): Promise<LlmKey[]> {
  const res = await adminClient.request<undefined, PagedItems<LlmKey>>({
    operationId: "admin.llm.listKeys",
    method: "GET",
    path: "/v1/llm/keys",
    query: { limit: 100 },
  });
  return res.data.items;
}

export async function fetchLlmUsage(limit = 20): Promise<LlmUsage[]> {
  const res = await adminClient.request<undefined, PagedItems<LlmUsage>>({
    operationId: "admin.llm.listUsage",
    method: "GET",
    path: "/v1/llm/usage",
    query: { limit },
  });
  return res.data.items;
}

export interface CreateChannelPayload {
  provider_id: string;
  model_id?: string;
  model_ids?: string[];
  models?: ChannelModelBindingInput[];
  title: string;
  base_url: string;
  upstream_model?: string;
  priority?: number;
  weight?: number;
  rpm?: number;
  concurrency?: number;
  enabled?: boolean;
}

export interface UpdateChannelPayload extends CreateChannelPayload {
  expected_revision: number;
}

export async function createLlmChannel(payload: CreateChannelPayload): Promise<LlmChannel> {
  const res = await adminClient.request<CreateChannelPayload, LlmChannel>({
    operationId: "admin.llm.createChannel",
    method: "POST",
    path: "/v1/llm/channels",
    body: payload,
  });
  return res.data;
}

export async function updateLlmChannel(channelId: string, payload: UpdateChannelPayload): Promise<LlmChannel> {
  const res = await adminClient.request<UpdateChannelPayload, LlmChannel>({
    operationId: "admin.llm.updateChannel",
    method: "PUT",
    path: `/v1/llm/channels/${channelId}`,
    body: payload,
  });
  return res.data;
}

export async function deleteLlmChannel(channelId: string, expectedRevision: number): Promise<void> {
  await adminClient.request<undefined, { id: string; deleted: boolean }>({
    operationId: "admin.llm.deleteChannel",
    method: "DELETE",
    path: `/v1/llm/channels/${channelId}`,
    query: { expected_revision: expectedRevision },
  });
}

export interface UpstreamModelItem {
  id: string;
  name?: string;
}

export interface FetchUpstreamModelsPayload {
  protocol: string;
  base_url: string;
  api_key?: string;
}

export async function fetchUpstreamModels(payload: FetchUpstreamModelsPayload): Promise<UpstreamModelItem[]> {
  const res = await adminClient.request<FetchUpstreamModelsPayload, { items: UpstreamModelItem[] }>({
    operationId: "admin.llm.fetchUpstreamModels",
    method: "POST",
    path: "/v1/llm/upstream-models",
    body: payload,
  });
  return res.data.items || [];
}

export interface CreateKeyPayload {
  channel_id: string;
  title: string;
  secret_ref: Omit<SecretRef, "kind"> & { kind: "env" | "secret_store" };
  weight: number;
  rpm: number;
  concurrency: number;
  enabled: boolean;
}

export interface UpdateKeyPayload extends CreateKeyPayload {
  expected_revision: number;
}

export async function createLlmKey(payload: CreateKeyPayload): Promise<LlmKey> {
  const res = await adminClient.request<CreateKeyPayload, LlmKey>({
    operationId: "admin.llm.createKey",
    method: "POST",
    path: "/v1/llm/keys",
    body: payload,
  });
  return res.data;
}

export async function updateLlmKey(keyId: string, payload: UpdateKeyPayload): Promise<LlmKey> {
  const res = await adminClient.request<UpdateKeyPayload, LlmKey>({
    operationId: "admin.llm.updateKey",
    method: "PUT",
    path: `/v1/llm/keys/${keyId}`,
    body: payload,
  });
  return res.data;
}

export async function deleteLlmKey(keyId: string, expectedRevision: number): Promise<void> {
  await adminClient.request<undefined, { id: string; deleted: boolean }>({
    operationId: "admin.llm.deleteKey",
    method: "DELETE",
    path: `/v1/llm/keys/${keyId}`,
    query: { expected_revision: expectedRevision },
  });
}

// --- Provider & Model CRUD ---

export interface CreateProviderPayload {
  code: string;
  title: string;
  protocol: string;
  enabled: boolean;
}

export interface UpdateProviderPayload extends CreateProviderPayload {
  expected_revision: number;
}

export async function createLlmProvider(payload: CreateProviderPayload): Promise<LlmProvider> {
  const res = await adminClient.request<CreateProviderPayload, LlmProvider>({
    operationId: "admin.llm.createProvider",
    method: "POST",
    path: "/v1/llm/providers",
    body: payload,
  });
  return res.data;
}

export async function updateLlmProvider(providerId: string, payload: UpdateProviderPayload): Promise<LlmProvider> {
  const res = await adminClient.request<UpdateProviderPayload, LlmProvider>({
    operationId: "admin.llm.updateProvider",
    method: "PUT",
    path: `/v1/llm/providers/${providerId}`,
    body: payload,
  });
  return res.data;
}

export async function deleteLlmProvider(providerId: string, expectedRevision: number): Promise<void> {
  await adminClient.request<undefined, { id: string; deleted: boolean }>({
    operationId: "admin.llm.deleteProvider",
    method: "DELETE",
    path: `/v1/llm/providers/${providerId}`,
    query: { expected_revision: expectedRevision },
  });
}

export interface CreateModelPayload {
  provider_id: string;
  upstream_name: string;
  title: string;
  capabilities: string[];
  context_window: number;
  max_output_tokens: number;
  embedding_dimensions?: number | null;
  enabled: boolean;
}

export interface UpdateModelPayload extends CreateModelPayload {
  expected_revision: number;
}

export async function createLlmModel(payload: CreateModelPayload): Promise<LlmModel> {
  const res = await adminClient.request<CreateModelPayload, LlmModel>({
    operationId: "admin.llm.createModel",
    method: "POST",
    path: "/v1/llm/models",
    body: payload,
  });
  return res.data;
}

export async function updateLlmModel(modelId: string, payload: UpdateModelPayload): Promise<LlmModel> {
  const res = await adminClient.request<UpdateModelPayload, LlmModel>({
    operationId: "admin.llm.updateModel",
    method: "PUT",
    path: `/v1/llm/models/${modelId}`,
    body: payload,
  });
  return res.data;
}


export interface CreatePricingPayload {
  model_id: string;
  currency: string;
  input_micros_per_million: number;
  output_micros_per_million: number;
  cache_micros_per_million: number;
  effective_at: string;
}

export async function createLlmPricing(payload: CreatePricingPayload): Promise<any> {
  const res = await adminClient.request<CreatePricingPayload, any>({
    operationId: "admin.llm.createPricing",
    method: "POST",
    path: "/v1/llm/pricing",
    body: payload,
  });
  return res.data;
}
export async function deleteLlmModel(modelId: string, expectedRevision: number): Promise<void> {
  await adminClient.request<undefined, { id: string; deleted: boolean }>({
    operationId: "admin.llm.deleteModel",
    method: "DELETE",
    path: `/v1/llm/models/${modelId}`,
    query: { expected_revision: expectedRevision },
  });
}

export async function fetchAgentDefinitions(): Promise<AgentDefinitionView[]> {
  const items: AgentDefinitionView[] = [];
  let cursor: string | undefined;
  for (;;) {
    const res = await adminClient.request<undefined, PagedItems<AgentDefinitionView>>({
      operationId: "admin.agent.listDefinitions",
      method: "GET",
      path: "/v1/agents/definitions",
      query: { limit: 100, ...(cursor ? { cursor } : {}) },
    });
    items.push(...res.data.items);
    if (!res.data.has_more || !res.data.next_cursor) return items;
    cursor = res.data.next_cursor;
  }
}

export async function fetchAgentWorkflows(): Promise<AgentWorkflowView[]> {
  const items: AgentWorkflowView[] = [];
  let cursor: string | undefined;
  for (;;) {
    const res = await adminClient.request<undefined, PagedItems<AgentWorkflowView>>({
      operationId: "admin.agent.listWorkflows",
      method: "GET",
      path: "/v1/agents/workflows",
      query: { limit: 100, ...(cursor ? { cursor } : {}) },
    });
    items.push(...res.data.items);
    if (!res.data.has_more || !res.data.next_cursor) return items;
    cursor = res.data.next_cursor;
  }
}


export interface CreateAgentDefinitionPayload {
  definition_key: string;
  title: string;
  enabled: boolean;
  system_instruction: string;
  tool_version_ids: string[];
  task_kinds: string[];
}

export interface UpdateAgentDefinitionPayload extends CreateAgentDefinitionPayload {
  expected_revision: number;
}

export async function createAgentDefinition(payload: CreateAgentDefinitionPayload): Promise<AgentDefinitionView> {
  const res = await adminClient.request<CreateAgentDefinitionPayload, AgentDefinitionView>({
    operationId: "admin.agent.createDefinition",
    method: "POST",
    path: "/v1/agents/definitions",
    body: payload,
  });
  return res.data;
}

export async function updateAgentDefinition(id: string, payload: UpdateAgentDefinitionPayload): Promise<AgentDefinitionView> {
  const res = await adminClient.request<UpdateAgentDefinitionPayload, AgentDefinitionView>({
    operationId: "admin.agent.updateDefinition",
    method: "PUT",
    path: `/v1/agents/definitions/${id}`,
    body: payload,
  });
  return res.data;
}

export async function deleteAgentDefinition(id: string, expectedRevision: number): Promise<void> {
  await adminClient.request<undefined, { id: string; deleted: boolean }>({
    operationId: "admin.agent.deleteDefinition",
    method: "DELETE",
    path: `/v1/agents/definitions/${id}`,
    query: { expected_revision: expectedRevision },
  });
}

export interface CreateAgentWorkflowPayload {
  workflow_key: string;
  title: string;
  enabled: boolean;
  manifest: AgentWorkflowView["manifest"];
}

export interface UpdateAgentWorkflowPayload extends CreateAgentWorkflowPayload {
  expected_revision: number;
}

export async function createAgentWorkflow(payload: CreateAgentWorkflowPayload): Promise<AgentWorkflowView> {
  const res = await adminClient.request<CreateAgentWorkflowPayload, AgentWorkflowView>({
    operationId: "admin.agent.createWorkflow",
    method: "POST",
    path: "/v1/agents/workflows",
    body: payload,
  });
  return res.data;
}

export async function updateAgentWorkflow(id: string, payload: UpdateAgentWorkflowPayload): Promise<AgentWorkflowView> {
  const res = await adminClient.request<UpdateAgentWorkflowPayload, AgentWorkflowView>({
    operationId: "admin.agent.updateWorkflow",
    method: "PUT",
    path: `/v1/agents/workflows/${id}`,
    body: payload,
  });
  return res.data;
}

export async function deleteAgentWorkflow(id: string, expectedRevision: number): Promise<void> {
  await adminClient.request<undefined, { id: string; deleted: boolean }>({
    operationId: "admin.agent.deleteWorkflow",
    method: "DELETE",
    path: `/v1/agents/workflows/${id}`,
    query: { expected_revision: expectedRevision },
  });
}

// Only the server's registered handlers are shown here; this catalog is not tool configuration.
export async function fetchAgentBuiltinTools(): Promise<AgentBuiltinToolView[]> {
  const items: AgentBuiltinToolView[] = [];
  let cursor: string | undefined;
  for (;;) {
    const res = await adminClient.request<undefined, PagedItems<AgentBuiltinToolView>>({
      operationId: "agent_list_builtin_tools",
      method: "GET",
      path: "/v1/agents/builtin-tools",
      query: { limit: 100, ...(cursor ? { cursor } : {}) },
    });
    items.push(...res.data.items);
    if (!res.data.has_more || !res.data.next_cursor) return items;
    cursor = res.data.next_cursor;
  }
}

export interface ProjectLog {
  id: string;
  created_at: string;
  level: string;
  event: string;
  domain: string;
  operation: string;
  error_code: string;
  request_id: string;
  trace_id: string;
  status: number | null;
  duration_ms: number | null;
}

export interface ProjectLogQueryOptions {
  limit?: number;
  cursor?: string;
  level?: string;
  search?: string;
}

export async function fetchProjectLogs(query: ProjectLogQueryOptions): Promise<PagedItems<ProjectLog>> {
  const res = await adminClient.request<undefined, PagedItems<ProjectLog>>({
    operationId: "platformListProjectLogs",
    method: "GET",
    path: "/v1/platform/project-logs",
    query: {
      limit: query.limit ?? 20,
      ...(query.cursor ? { cursor: query.cursor } : {}),
      ...(query.level ? { level: query.level } : {}),
      ...(query.search ? { search: query.search } : {}),
    },
  });
  return res.data;
}

export interface AuditQueryOptions {
  from?: string;
  to?: string;
  action?: string;
  limit?: number;
  cursor?: string;
}

export async function fetchAuditEvents(query?: AuditQueryOptions): Promise<PagedItems<AuditEvent>> {
  const cleanQuery: Record<string, string | number> = {};
  if (query?.from) cleanQuery.from = query.from;
  if (query?.to) cleanQuery.to = query.to;
  if (query?.action) cleanQuery.action = query.action;
  if (query?.limit) cleanQuery.limit = query.limit;
  if (query?.cursor) cleanQuery.cursor = query.cursor;

  const res = await adminClient.request<undefined, PagedItems<AuditEvent>>({
    operationId: "admin.audit.listEvents",
    method: "GET",
    path: "/v1/platform/audit-events",
    query: cleanQuery,
  });
  return res.data;
}

export async function exportAuditEventsNDJSON(options?: { from?: string; to?: string; action?: string }): Promise<string> {
  const params = new URLSearchParams();
  if (options?.from) params.set("from", options.from);
  if (options?.to) params.set("to", options.to);
  if (options?.action) params.set("action", options.action);

  const url = `/v1/platform/audit-events/export${params.toString() ? "?" + params.toString() : ""}`;
  const token = adminSession.getAccessToken();
  const headers: Record<string, string> = {
    Accept: "application/x-ndjson, text/plain",
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const res = await fetch(url, {
    method: "GET",
    headers,
    credentials: "same-origin",
  });

  if (!res.ok) {
    throw new Error(`Export failed with status ${res.status}`);
  }
  return res.text();
}

// --- 租户成员与用户管理 API ---

export interface PlatformMember {
  id: string;
  revision: number;
  created_at: string;
  updated_at: string;
  user_id: string;
  email: string;
  display_name: string;
  roles: string[];
  status: "active" | "disabled";
}

export interface CreateMemberPayload {
  email: string;
  display_name: string;
  password: string;
  roles: string[];
}

export interface UpdateMemberPayload {
  expected_revision: number;
  roles: string[];
  status: "active" | "disabled";
}

export interface ResetPasswordPayload {
  expected_revision: number;
  new_password: string;
}

export interface RevisionCommandPayload {
  expected_revision: number;
}

export interface LlmPricing {
  id: string;
  revision: number;
  model_id: string;
  currency: string;
  input_micros_per_million: number;
  output_micros_per_million: number;
  cache_micros_per_million: number;
  effective_at: string;
  version: number;
  created_at: string;
  updated_at: string;
}

export async function fetchPlatformMembers(limit = 50, cursor?: string): Promise<PagedItems<PlatformMember>> {
  const query: Record<string, number | string> = { limit };
  if (cursor) query.cursor = cursor;
  const res = await adminClient.request<undefined, PagedItems<PlatformMember>>({
    operationId: "admin.platform.listMembers",
    method: "GET",
    path: "/v1/platform/members",
    query,
  });
  return res.data;
}

export async function fetchPlatformMember(memberId: string): Promise<PlatformMember> {
  const res = await adminClient.request<undefined, PlatformMember>({
    operationId: "admin.platform.getMember",
    method: "GET",
    path: `/v1/platform/members/${memberId}`,
  });
  return res.data;
}

export async function createPlatformMember(payload: CreateMemberPayload): Promise<PlatformMember> {
  const res = await adminClient.request<CreateMemberPayload, PlatformMember>({
    operationId: "admin.platform.createMember",
    method: "POST",
    path: "/v1/platform/members",
    body: payload,
  });
  return res.data;
}

export async function updatePlatformMember(memberId: string, payload: UpdateMemberPayload): Promise<PlatformMember> {
  const res = await adminClient.request<UpdateMemberPayload, PlatformMember>({
    operationId: "admin.platform.updateMember",
    method: "PUT",
    path: `/v1/platform/members/${memberId}`,
    body: payload,
  });
  return res.data;
}

export async function resetMemberPassword(memberId: string, payload: ResetPasswordPayload): Promise<PlatformMember> {
  const res = await adminClient.request<ResetPasswordPayload, PlatformMember>({
    operationId: "admin.platform.resetPassword",
    method: "POST",
    path: `/v1/platform/members/${memberId}/reset-password`,
    body: payload,
  });
  return res.data;
}

export async function revokeMemberSessions(
  memberId: string,
  payload: RevisionCommandPayload
): Promise<{ revoked_count: number; revision: number }> {
  const res = await adminClient.request<RevisionCommandPayload, { revoked_count: number; revision: number }>({
    operationId: "admin.platform.revokeSessions",
    method: "POST",
    path: `/v1/platform/members/${memberId}/revoke-sessions`,
    body: payload,
  });
  return res.data;
}

export async function deletePlatformMember(memberId: string): Promise<{ deleted: boolean }> {
  const res = await adminClient.request<undefined, { deleted: boolean }>({
    operationId: "admin.platform.deleteMember",
    method: "DELETE",
    path: `/v1/platform/members/${memberId}`,
  });
  return res.data;
}

export async function updateLlmBudget(payload: { daily_limit_micros: number; expected_revision: number }): Promise<LlmBudget> {
  const res = await adminClient.request<{ daily_limit_micros: number; expected_revision: number }, LlmBudget>({
    operationId: "admin.llm.updateBudget",
    method: "PUT",
    path: "/v1/llm/budget",
    body: payload,
  });
  return res.data;
}

export async function fetchLlmPricings(limit = 100): Promise<LlmPricing[]> {
  const res = await adminClient.request<undefined, PagedItems<LlmPricing>>({
    operationId: "admin.llm.listPricings",
    method: "GET",
    path: "/v1/llm/pricing",
    query: { limit },
  });
  return res.data.items;
}
