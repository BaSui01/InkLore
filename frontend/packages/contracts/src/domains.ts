/**
 * @inklore/contracts - 七大业务域实体模型与操作载荷契约
 * 严格对齐 docs/system-design/api/*.json 与后端 OpenAPI 3.1.0 规范
 */

import { UUID, Revision, Timestamp } from "./types";

// ==========================================
// 1. 内容域 (Content Domain)
// ==========================================
export interface Novel {
  id: string;
  tenant_id?: string;
  owner_id?: string;
  title: string;
  summary: string;
  category: string;
  status: "active" | "archived" | "deleted";
  previous_status?: "active" | "archived" | null;
  target_words: number;
  target_completion_date: string | null;
  word_count: number;
  revision: number;
  created_at: string;
  updated_at: string;
}

export interface CreateNovelInput {
  title: string;
  summary?: string;
  category?: string;
  target_words?: number;
  target_completion_date?: string | null;
}

export interface UpdateNovelInput {
  expected_revision: number;
  title: string;
  summary: string;
  category: string;
  target_words: number;
  target_completion_date: string | null;
}

export interface WritingStatistics {
  novel_id: string;
  from: string;
  to: string;
  current_word_count: number;
  days: Array<{
    date: string;
    added_words: number;
    removed_words: number;
    net_words: number;
    save_count: number;
    timezone: "Asia/Shanghai";
  }>;
}

export interface Volume {
  id: string;
  novel_id: string;
  title: string;
  order_index: number;
  revision: number;
  created_at: string;
  updated_at: string;
}

export interface CreateVolumeInput {
  title: string;
  summary: string;
}

export interface Chapter {
  id: string;
  novel_id: string;
  volume_id: string | null;
  title: string;
  order_index: number;
  word_count: number;
  status?: string;
  confirmed_version: number;
  confirmed_version_id?: string | null;
  current_version_id?: string;
  revision: number;
  created_at: string;
  updated_at: string;
}

export interface CreateChapterInput {
  volume_id: string;
  title: string;
}

export interface ChapterBody {
  chapter: Chapter;
  body: string;
}

export interface SaveBodyInput {
  body: string;
  expected_revision: number;
}

export type ChapterBodyResponse = ChapterBody;
export type SaveBodyResponse = ChapterBody;

export interface FactProposalValue {
  kind: "text" | "number" | "boolean" | "entity";
  text_value?: string;
  number_value?: number;
  boolean_value?: boolean;
  entity_id?: string;
}

export interface FactProposal {
  subject_id: string;
  predicate: string;
  fact_key: string;
  value?: FactProposalValue;
  action: "assert" | "retract";
  target_event_id?: string;
  resolution: "append" | "supersede";
  reason?: string;
}

export interface ConfirmChapterInput {
  expected_revision: number;
  expected_world_revision: number;
  fact_proposals?: FactProposal[];
}

export interface ConfirmChapterResponse {
  chapter: Chapter;
  world_revision: number;
  event_ids: string[];
}

// ==========================================
// 2. 大纲域 (Outline Domain)
// ==========================================
export interface OutlineNode {
  id: string;
  novel_id: string;
  parent_id?: string | null;
  title: string;
  synopsis: string;
  level: "novel" | "volume" | "chapter";
  order_index: number;
  revision: number;
  created_at: string;
}

export interface CreateOutlineNodeInput {
  parent_id?: string | null;
  title: string;
  synopsis?: string;
  level: "novel" | "volume" | "chapter";
  order_index?: number;
}

export interface Scene {
  id: string;
  novel_id: string;
  chapter_id: string;
  title: string;
  summary: string;
  order_index: number;
  word_count: number;
  beats?: string;
  created_at: string;
}

export interface CreateSceneInput {
  chapter_id: string;
  title: string;
  summary?: string;
  order_index?: number;
  beats?: string;
}

export interface PacingPoint {
  chapter_number: number;
  tension: number;      // 0-10 张力
  emotion: number;      // 0-10 情绪强度
  density: number;      // 0-10 信息密度
  filler_risk: number;  // 0-10 注水风险
  phase: "压抑" | "蓄力" | "高潮" | "释放" | "过渡" | "冷却";
}

export interface PacingAnalysis {
  novel_id: string;
  overall_score: number;
  points: PacingPoint[];
}

// ==========================================
// 3. 设定与事实图谱域 (Worldview Domain)
// ==========================================
export interface Character {
  id: string;
  novel_id: string;
  name: string;
  role_type: "protagonist" | "antagonist" | "supporting" | "cameo";
  gender?: string;
  goals: string[];
  summary: string;
  avatar_url?: string;
  revision: number;
}

export interface CreateCharacterInput {
  name: string;
  role_type: "protagonist" | "antagonist" | "supporting" | "cameo";
  gender?: string;
  goals?: string[];
  summary?: string;
}

export interface LoreEntry {
  id: string;
  novel_id: string;
  category: "power_system" | "geography" | "faction" | "item" | "history";
  title: string;
  content: string;
  revision: number;
}

export interface GraphNode {
  id: string;
  label: string;
  type: string;
  color?: string;
}

export interface GraphEdge {
  source: string;
  target: string;
  label: string;
}

export interface WorldviewGraph {
  novel_id: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface WorldRevisionResponse {
  revision: number;
}

// ==========================================
// 4. 智能体与工作流域 (Agent Domain)
// ==========================================
export interface AgentRun {
  id: string;
  revision: number;
  workflow_id?: string;
  resource: AgentResourceRef | null;
  policy_version_id: string;
  pending_questions: AgentQuestion[];
  nodes?: Array<{id: string; role: string; status: string}>;
  status: "queued" | "pending" | "running" | "waiting_approval" | "waiting_clarification" | "cancelling" | "succeeded" | "completed" | "failed" | "cancelled";
  prompt?: string;
  context_assembly?: Record<string, unknown>;
  output?: string;
  created_at: string;
  finished_at?: string;
  error_code?: string;
  terminal_reason?: string;
}

export interface AgentResourceSelection {
  start_offset: number;
  end_offset: number;
  source_hash: string;
}

export interface AgentResourceRef {
  resource_id: string;
  resource_type: string;
  source_revision: number;
  novel_id?: string;
  source_hash?: string;
  selection?: AgentResourceSelection;
  selection_scope_verified?: boolean;
}

export interface CancelRunInput {
  expected_revision: number;
}

export interface CreateRunInput {
  task_kind: string;
  task_key: string;
  resource: AgentResourceRef;
  instruction: string;
  definition_id: string;
  workflow_id?: string;
  model_id?: string;
  model_key?: string;
}

export type QuestionResponseSchema =
  | { type: "string"; enum: string[] }
  | { type: "string"; minLength?: number; maxLength: number }
  | { type: "array"; uniqueItems: true; items: { type: "string"; enum: string[] }; minItems?: number; maxItems: number };

export interface AgentQuestion {
  id: string;
  revision: number;
  question_version: 1;
  run_id: string;
  node_id: string;
  status: "open" | "answered" | "expired" | "cancelled";
  title_key: string;
  help_key: string;
  response_schema: QuestionResponseSchema;
  options: Array<{ value: string; label_key: string }>;
  context_summary: string;
  source_revision: number;
  source_hash: string;
  expires_at: string;
  answer_hash: string | null;
  answered_at: string | null;
}

export interface AnswerQuestionInput {
  expected_revision: number;
  question_version: 1;
  answer: string | string[];
}

export interface CatalogTaskPolicy {
  policy_id: string;
  policy_version_id: string;
  task_key: string;
  title: string;
  version: number;
  task_kinds: string[];
  input_schema: Record<string, unknown>;
  budget: { max_output_tokens: number; timeout_seconds: number };
}

export interface AgentRunSummary {
  id: string;
  revision: number;
  task_kind: string;
  status: AgentRun["status"];
  resource: AgentResourceRef | null;
  created_at: string;
}

export interface AgentAuditRun {
  created_at: string;
  updated_at: string;
  kind: "creator" | "diagnostic";
  id: string;
  revision: number;
  task_kind: string;
  status: AgentRun["status"];
  node_count: number;
  duration_ms: number;
  cost_micros: number | null;
  error_code: string | null;
}

export interface CatalogDefinition {
  id: string;
  title: string;
  version_id: string;
  task_kinds: string[];
}

export interface CatalogWorkflow {
  id: string;
  title: string;
  version_id: string;
}

export interface ArtifactView {
  id: string;
  run_id: string;
  node_id: string;
  kind: "chapter_draft" | "rewrite_suggestion" | "outline_suggestion" | "worldview_suggestion" | "analysis_result" | "audit_result";
  target_resource_id: string | null;
  source_revision: number;
  content_hash: string;
  text_content: string | null;
  file_id: string | null;
  is_partial: boolean;
  created_at: string;
  updated_at: string;
}

export interface AgentStreamEvent {
  seq: number;
  type: "text_delta" | "thought_summary" | "tool_call" | "tool_result" | "run_completed" | "run_failed";
  data: Record<string, unknown>;
}

// ==========================================
// 5. 模型网关域 (LLM Domain)
// ==========================================
export interface LlmModel {
  id: string;
  name: string;
  provider: string;
  context_window: number;
  pricing_input_per_m: number;
  pricing_output_per_m: number;
  status: "healthy" | "degraded" | "offline";
}

export interface KeyChannel {
  id: string;
  provider: string;
  channel_name: string;
  secret_mask: string;
  weight: number;
  status: "active" | "cooling" | "disabled";
  error_rate: string;
}

// ==========================================
// 6. 合规审查与质检域 (Compliance Domain)
// ==========================================
export interface ComplianceReport {
  id: string;
  novel_id: string;
  chapter_id: string;
  chapter_revision: number;
  content_hash: string;
  rule_set_id: string;
  mode: "rules" | "deep";
  state: "pending" | "running" | "succeeded" | "failed" | "cancelled";
  summary?: string;
  ai_flavor_score?: number | null;
  risk_count: number;
  open_count: number;
  block_count: number;
  created_at: string;
}

export interface ComplianceIssue {
  id: string;
  report_id: string;
  rule_id?: string | null;
  detector: "rule" | "llm";
  kind: "sensitive" | "ai_flavor";
  severity: "warning" | "block";
  start_offset: number;
  end_offset: number;
  excerpt: string;
  excerpt_truncated?: boolean;
  message: string;
  suggestion: string;
  confidence: number;
  state: "open" | "ignored";
  created_at: string;
}

export interface CreateScanInput {
  novel_id: string;
  chapter_id: string;
  chapter_revision: number;
  content_hash: string;
  text: string;
}

export interface AgentApproval {
  id: string;
  revision: number;
  created_at: string;
  updated_at: string;
  run_id: string;
  node_id: string;
  owner_user_id: string;
  tool_version_id: string;
  arguments_hash: string;
  arguments_json: string;
  source_revision: number;
  status: "pending" | "consumed" | "rejected" | "expired" | "cancelled";
  expires_at: string;
  decided_at: string | null;
  decision_reason: string | null;
}
export interface ApprovalDecisionInput {
  expected_revision: number;
  arguments_hash: string;
  source_revision: number;
  reason?: string;
}
