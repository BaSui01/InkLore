/**
 * @inklore/transport - 统一业务域 API SDK
 * 严格按照 docs/system-design/api/openapi.json 与后端 7 大业务域路由契约映射
 * 杜绝在前端组件中书写硬编码静态数据，全部经由此 SDK 与后端交互
 */

import { ApiClient } from "../http/client";
import { AuthSession } from "../http/session";
import {
  Novel,
  CreateNovelInput,
  UpdateNovelInput,
  WritingStatistics,
  NetworkError,
  isValidRevision,
  Volume,
  CreateVolumeInput,
  Chapter,
  CreateChapterInput,
  ChapterBodyResponse,
  SaveBodyInput,
  SaveBodyResponse,
  ConfirmChapterInput,
  ConfirmChapterResponse,
  OutlineNode,
  CreateOutlineNodeInput,
  Scene,
  CreateSceneInput,
  PacingAnalysis,
  Character,
  CreateCharacterInput,
  LoreEntry,
  WorldviewGraph,
  WorldRevisionResponse,
  AgentRun,
  CancelRunInput,
  CreateRunInput,
  CatalogDefinition,
  CatalogWorkflow,
  ArtifactView,
  LlmModel,
  ComplianceReport,
  ComplianceIssue,
  CreateScanInput,
  PagedList,
  ContentAPI,
  DownloadResult,
} from "@inklore/contracts";

function requireNovel(value: Novel, novelId: string): Novel {
  if (!value || value.id !== novelId || !isValidRevision(value.revision) ||
      typeof value.title !== "string" || typeof value.summary !== "string" || typeof value.category !== "string" ||
      !["active", "archived", "deleted"].includes(value.status) ||
      !Number.isSafeInteger(value.word_count) || value.word_count < 0 || value.word_count > 400000000 ||
      !Number.isSafeInteger(value.target_words) || value.target_words < 0 || value.target_words > 100000000 ||
      !(value.target_completion_date === null || (typeof value.target_completion_date === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value.target_completion_date)))) {
    throw new NetworkError("Incomplete novel response");
  }
  return value;
}

function requirePagedItems<T>(page: PagedList<T>): T[] {
  if (!Array.isArray(page?.items)) throw new Error("Invalid paged API response");
  return page.items;
}

export class ApiSDK {
  constructor(
    private readonly client: ApiClient,
    private readonly session?: AuthSession
  ) {}

  /** 获取当前底层的身份会话管理实例 */
  getSession(): AuthSession | undefined {
    return this.session;
  }



  // ==========================================
  // 1. 内容域 (Content Domain)
  // ==========================================
  readonly content = {
    createExport: async (novelId: string, input: ContentAPI.components["schemas"]["CreateExport"], idempotencyKey: string): Promise<ContentAPI.components["schemas"]["Job"]> => {
      const res = await this.client.request<typeof input, ContentAPI.components["schemas"]["Job"]>({
        operationId: "content_createExport", method: "POST",
        path: "/v1/content/novels/:novel_id/exports", pathParams: { novel_id: novelId },
        body: input, idempotencyKey,
      });
      return res.data;
    },

    listJobs: async (novelId: string): Promise<ContentAPI.components["schemas"]["Job"][]> => {
      const res = await this.client.request<unknown, ContentAPI.components["schemas"]["JobPage"]>({
        operationId: "content_listJobs", method: "GET",
        path: "/v1/content/novels/:novel_id/jobs", pathParams: { novel_id: novelId }, query: { limit: 20 },
      });
      return requirePagedItems(res.data);
    },

    downloadExport: async (fileId: string): Promise<DownloadResult> => {
      return this.client.download({
        operationId: "platform_downloadFile", path: "/v1/platform/files/:file_id/download",
        pathParams: { file_id: fileId },
      });
    },

    /** 获取作品书架列表 (GET /v1/content/novels) */
    listNovels: async (params?: { limit?: number; cursor?: string; status?: string }): Promise<Novel[]> => {
      const res = await this.client.request<unknown, PagedList<Novel>>({
        operationId: "content_listNovels",
        method: "GET",
        path: "/v1/content/novels",
        query: params,
      });
      return requirePagedItems(res.data);
    },

    /** 创建新小说 (POST /v1/content/novels) */
    createNovel: async (input: CreateNovelInput): Promise<Novel> => {
      const res = await this.client.request<CreateNovelInput, Novel>({
        operationId: "content_createNovel",
        method: "POST",
        path: "/v1/content/novels",
        body: input,
      });
      return res.data;
    },

    /** 获取小说详情 (GET /v1/content/novels/:novel_id) */
    getNovel: async (novelId: string): Promise<Novel> => {
      const res = await this.client.request<unknown, Novel>({
        operationId: "content_getNovel",
        method: "GET",
        path: "/v1/content/novels/:novel_id",
        pathParams: { novel_id: novelId },
      });
      return requireNovel(res.data, novelId);
    },

    updateNovel: async (novelId: string, input: UpdateNovelInput, idempotencyKey: string): Promise<Novel> => {
      const res = await this.client.request<UpdateNovelInput, Novel>({
        operationId: "content_updateNovel", method: "PUT",
        path: "/v1/content/novels/:novel_id", pathParams: { novel_id: novelId },
        body: input, idempotencyKey,
      });
      return requireNovel(res.data, novelId);
    },

    writingStatistics: async (novelId: string, from: string, to: string, signal?: AbortSignal): Promise<WritingStatistics> => {
      const res = await this.client.request<unknown, WritingStatistics>({
        operationId: "content_statistics", method: "GET",
        path: "/v1/content/novels/:novel_id/statistics", pathParams: { novel_id: novelId },
        query: { from, to }, signal,
      });
      const value = res.data;
      if (!value || value.novel_id !== novelId || value.from !== from || value.to !== to ||
          !Number.isSafeInteger(value.current_word_count) || value.current_word_count < 0 || !Array.isArray(value.days) || value.days.length === 0 ||
          !value.days.every(day => typeof day.date === "string" && day.timezone === "Asia/Shanghai" &&
            Number.isSafeInteger(day.added_words) && day.added_words >= 0 && Number.isSafeInteger(day.removed_words) && day.removed_words >= 0 &&
            Number.isSafeInteger(day.net_words) && day.net_words === day.added_words - day.removed_words && Number.isSafeInteger(day.save_count) && day.save_count >= 0)) {
        throw new NetworkError("Incomplete writing statistics response");
      }
      return value;
    },

    /** 获取卷列表 (GET /v1/content/novels/:novel_id/volumes) */
    listVolumes: async (novelId: string): Promise<Volume[]> => {
      const res = await this.client.request<unknown, PagedList<Volume>>({
        operationId: "content_listVolumes",
        method: "GET",
        path: "/v1/content/novels/:novel_id/volumes",
        pathParams: { novel_id: novelId },
      });
      return requirePagedItems(res.data);
    },

    /** 创建卷 (POST /v1/content/novels/:novel_id/volumes) */
    createVolume: async (novelId: string, input: CreateVolumeInput): Promise<Volume> => {
      const res = await this.client.request<CreateVolumeInput, Volume>({
        operationId: "content_createVolume",
        method: "POST",
        path: "/v1/content/novels/:novel_id/volumes",
        pathParams: { novel_id: novelId },
        body: input,
      });
      return res.data;
    },

    /** 获取章节列表 (GET /v1/content/novels/:novel_id/chapters) */
    listChapters: async (novelId: string): Promise<Chapter[]> => {
      const res = await this.client.request<unknown, PagedList<Chapter>>({
        operationId: "content_listChapters",
        method: "GET",
        path: "/v1/content/novels/:novel_id/chapters",
        pathParams: { novel_id: novelId },
      });
      return requirePagedItems(res.data);
    },

    /** 创建章节 (POST /v1/content/novels/:novel_id/chapters) */
    createChapter: async (novelId: string, input: CreateChapterInput): Promise<Chapter> => {
      const res = await this.client.request<CreateChapterInput, Chapter>({
        operationId: "content_createChapter",
        method: "POST",
        path: "/v1/content/novels/:novel_id/chapters",
        pathParams: { novel_id: novelId },
        body: input,
      });
      return res.data;
    },

    /** 获取章节正文内容 (GET /v1/content/chapters/:chapter_id/body) */
    getChapterBody: async (chapterId: string): Promise<ChapterBodyResponse> => {
      const res = await this.client.request<unknown, ChapterBodyResponse>({
        operationId: "content_getChapterBody",
        method: "GET",
        path: "/v1/content/chapters/:chapter_id/body",
        pathParams: { chapter_id: chapterId },
      });
      return res.data;
    },

    /** 实时保存章节正文 (PUT /v1/content/chapters/:chapter_id/body 带 CAS 期望版本号) */
    saveChapterBody: async (chapterId: string, input: SaveBodyInput, idempotencyKey?: string): Promise<SaveBodyResponse> => {
      const res = await this.client.request<SaveBodyInput, SaveBodyResponse>({
        operationId: "content_saveBody",
        method: "PUT",
        path: "/v1/content/chapters/:chapter_id/body",
        pathParams: { chapter_id: chapterId },
        body: input,
        expectedRevision: input.expected_revision,
        idempotencyKey,
      });
      return res.data;
    },

    /** 章节正式确认 (POST /v1/content/chapters/:chapter_id/confirm 触发跨域事实入账) */
    confirmChapter: async (chapterId: string, input: ConfirmChapterInput): Promise<ConfirmChapterResponse> => {
      const res = await this.client.request<ConfirmChapterInput, ConfirmChapterResponse>({
        operationId: "content_confirmChapter",
        method: "POST",
        path: "/v1/content/chapters/:chapter_id/confirm",
        pathParams: { chapter_id: chapterId },
        body: input,
      });
      return res.data;
    },
  };

  // ==========================================
  // 2. 大纲域 (Outline Domain)
  // ==========================================
  readonly outline = {
    /** 获取作品大纲节点 (GET /v1/outline/novels/:novel_id/nodes) */
    listNodes: async (novelId: string): Promise<OutlineNode[]> => {
      const res = await this.client.request<unknown, PagedList<OutlineNode>>({
        operationId: "outline_listNodes",
        method: "GET",
        path: "/v1/outline/novels/:novel_id/nodes",
        pathParams: { novel_id: novelId },
      });
      return requirePagedItems(res.data);
    },

    /** 创建大纲节点 (POST /v1/outline/novels/:novel_id/nodes) */
    createNode: async (novelId: string, input: CreateOutlineNodeInput): Promise<OutlineNode> => {
      const res = await this.client.request<CreateOutlineNodeInput, OutlineNode>({
        operationId: "outline_createNode",
        method: "POST",
        path: "/v1/outline/novels/:novel_id/nodes",
        pathParams: { novel_id: novelId },
        body: input,
      });
      return res.data;
    },

    /** 获取作品所有场景规划 (GET /v1/outline/novels/:novel_id/scenes) */
    listScenes: async (novelId: string): Promise<Scene[]> => {
      const res = await this.client.request<unknown, PagedList<Scene>>({
        operationId: "outline_listScenes",
        method: "GET",
        path: "/v1/outline/novels/:novel_id/scenes",
        pathParams: { novel_id: novelId },
      });
      return requirePagedItems(res.data);
    },

    /** 创建场景 (POST /v1/outline/novels/:novel_id/scenes) */
    createScene: async (novelId: string, input: CreateSceneInput): Promise<Scene> => {
      const res = await this.client.request<CreateSceneInput, Scene>({
        operationId: "outline_createScene",
        method: "POST",
        path: "/v1/outline/novels/:novel_id/scenes",
        pathParams: { novel_id: novelId },
        body: input,
      });
      return res.data;
    },

    /** 获取章节节奏与张力曲线诊断数据 (GET /v1/outline/novels/:novel_id/pacing) */
    getPacing: async (novelId: string): Promise<PacingAnalysis> => {
      const res = await this.client.request<unknown, PacingAnalysis>({
        operationId: "outline_getPacing",
        method: "GET",
        path: "/v1/outline/novels/:novel_id/pacing",
        pathParams: { novel_id: novelId },
      });
      return res.data;
    },
  };

  // ==========================================
  // 3. 设定与事实图谱域 (Worldview Domain)
  // ==========================================
  readonly worldview = {
    /** 获取角色列表 (GET /v1/worldview/novels/:novel_id/characters) */
    listCharacters: async (novelId: string): Promise<Character[]> => {
      const res = await this.client.request<unknown, PagedList<Character>>({
        operationId: "worldview_listCharacters",
        method: "GET",
        path: "/v1/worldview/novels/:novel_id/characters",
        pathParams: { novel_id: novelId },
      });
      return requirePagedItems(res.data);
    },

    /** 创建角色档案 (POST /v1/worldview/novels/:novel_id/characters) */
    createCharacter: async (novelId: string, input: CreateCharacterInput): Promise<Character> => {
      const res = await this.client.request<CreateCharacterInput, Character>({
        operationId: "worldview_createCharacter",
        method: "POST",
        path: "/v1/worldview/novels/:novel_id/characters",
        pathParams: { novel_id: novelId },
        body: input,
      });
      return res.data;
    },

    /** 获取世界观词条列表 (GET /v1/worldview/novels/:novel_id/lore) */
    listLore: async (novelId: string): Promise<LoreEntry[]> => {
      const res = await this.client.request<unknown, PagedList<LoreEntry>>({
        operationId: "worldview_listLore",
        method: "GET",
        path: "/v1/worldview/novels/:novel_id/lore",
        pathParams: { novel_id: novelId },
      });
      return requirePagedItems(res.data);
    },

    /** 获取世界观当前修订版本号 */
    getWorldRevision: async (novelId: string): Promise<WorldRevisionResponse> => {
      const res = await this.client.request<unknown, WorldRevisionResponse>({
        operationId: "worldview_getWorldRevision",
        method: "GET",
        path: "/v1/worldview/novels/:novel_id/world-revision",
        pathParams: { novel_id: novelId },
      });
      return res.data;
    },

    /** 获取力导向关系知识图谱 (GET /v1/worldview/novels/:novel_id/graph) */
    getGraph: async (novelId: string): Promise<WorldviewGraph> => {
      const res = await this.client.request<unknown, WorldviewGraph>({
        operationId: "worldview_getGraph",
        method: "GET",
        path: "/v1/worldview/novels/:novel_id/graph",
        pathParams: { novel_id: novelId },
      });
      return res.data;
    },
  };

  // ==========================================
  // 4. 智能体域 (Agent Domain)
  // ==========================================
  readonly agent = {
    /** Creator-safe published workflow catalog. */
    listCatalogWorkflows: async (): Promise<CatalogWorkflow[]> => {
      const items: CatalogWorkflow[] = [];
      let cursor: string | undefined;
      do {
        const res = await this.client.request<unknown, PagedList<CatalogWorkflow>>({
          operationId: "agent_listCatalogWorkflows",
          method: "GET",
          path: "/v1/agents/catalog/workflows",
          query: { limit: 100, cursor },
        });
        const page = res.data;
        if (!page || !Array.isArray(page.items) || typeof page.has_more !== "boolean" || (page.has_more && !page.next_cursor)) {
          throw new Error("Invalid workflow catalog response");
        }
        items.push(...page.items);
        cursor = page.has_more ? page.next_cursor || undefined : undefined;
      } while (cursor);
      return items;
    },

    /** Creator-safe published definition catalog. */
    listCatalogDefinitions: async (): Promise<CatalogDefinition[]> => {
      const items: CatalogDefinition[] = [];
      let cursor: string | undefined;
      do {
        const res = await this.client.request<unknown, PagedList<CatalogDefinition>>({
          operationId: "agent_listCatalogDefinitions",
          method: "GET",
          path: "/v1/agents/catalog/definitions",
          query: { limit: 100, cursor },
        });
        const page = res.data;
        if (!page || !Array.isArray(page.items) || typeof page.has_more !== "boolean" || (page.has_more && !page.next_cursor)) {
          throw new Error("Invalid definition catalog response");
        }
        items.push(...page.items);
        cursor = page.has_more ? page.next_cursor || undefined : undefined;
      } while (cursor);
      return items;
    },

    /** 启动智能体运行任务 (POST /v1/agents/runs) */
    createRun: async (input: CreateRunInput): Promise<AgentRun> => {
      const res = await this.client.request<CreateRunInput, AgentRun>({
        operationId: "agent_createRun",
        method: "POST",
        path: "/v1/agents/runs",
        body: input,
      });
      return res.data;
    },

    cancelRun: async (runId: string, input: CancelRunInput): Promise<AgentRun> => {
      const res = await this.client.request<CancelRunInput, AgentRun>({
        operationId: "agent_cancel_run",
        method: "POST",
        path: "/v1/agents/runs/:run_id/cancel",
        pathParams: { run_id: runId },
        body: input,
      });
      return res.data;
    },

    /** 获取指定运行详情 (GET /v1/agents/runs/:run_id) */
    getRun: async (runId: string): Promise<AgentRun> => {
      const res = await this.client.request<unknown, AgentRun>({
        operationId: "agent_getRun",
        method: "GET",
        path: "/v1/agents/runs/:run_id",
        pathParams: { run_id: runId },
      });
      return res.data;
    },

    /** 获取运行生成的产物列表 (GET /v1/agents/runs/:run_id/artifacts) */
    listArtifacts: async (runId: string): Promise<ArtifactView[]> => {
      const res = await this.client.request<unknown, PagedList<ArtifactView>>({
        operationId: "agent_listArtifacts",
        method: "GET",
        path: "/v1/agents/runs/:run_id/artifacts",
        pathParams: { run_id: runId },
      });
      return requirePagedItems(res.data);
    },

    /** 获取 SSE 事件流 URL */
    getRunEventsUrl: (runId: string): string => {
      return `/v1/agents/runs/${runId}/events`;
    },
  };

  // ==========================================
  // 5. 模型网关域 (LLM Domain)
  // ==========================================
  readonly llm = {
    /** 获取在册可用模型列表 (GET /v1/llm/models) */
    listModels: async (params?: { limit?: number; cursor?: string }): Promise<LlmModel[]> => {
      const res = await this.client.request<unknown, PagedList<LlmModel>>({
        operationId: "llm_list_models",
        method: "GET",
        path: "/v1/llm/models",
        query: params,
      });
      return requirePagedItems(res.data);
    },
  };

  // ==========================================
  // 6. 合规审查与质检域 (Compliance Domain)
  // ==========================================
  readonly compliance = {
    /** 提交章节快速扫描 (POST /v1/compliance/scans) */
    scan: async (input: CreateScanInput): Promise<ComplianceReport> => {
      const res = await this.client.request<CreateScanInput, ComplianceReport>({
        operationId: "compliance_scan",
        method: "POST",
        path: "/v1/compliance/scans",
        body: input,
      });
      return res.data;
    },

    /** 获取扫描报告详情 (GET /v1/compliance/reports/:report_id) */
    getReport: async (reportId: string): Promise<ComplianceReport> => {
      const res = await this.client.request<unknown, ComplianceReport>({
        operationId: "compliance_get_report",
        method: "GET",
        path: "/v1/compliance/reports/:report_id",
        pathParams: { report_id: reportId },
      });
      return res.data;
    },

    /** 获取报告问题列表 (GET /v1/compliance/reports/:report_id/issues) */
    listIssues: async (reportId: string, params?: { limit?: number; cursor?: string; severity?: string }): Promise<ComplianceIssue[]> => {
      const res = await this.client.request<unknown, PagedList<ComplianceIssue>>({
        operationId: "compliance_list_issues",
        method: "GET",
        path: "/v1/compliance/reports/:report_id/issues",
        pathParams: { report_id: reportId },
        query: params,
      });
      return requirePagedItems(res.data);
    },

    /** 忽略问题 (POST /v1/compliance/reports/:report_id/issues/:issue_id/ignore) */
    ignoreIssue: async (reportId: string, issueId: string, reason?: string): Promise<ComplianceIssue> => {
      const res = await this.client.request<{ reason: string }, ComplianceIssue>({
        operationId: "compliance_ignore_issue",
        method: "POST",
        path: "/v1/compliance/reports/:report_id/issues/:issue_id/ignore",
        pathParams: { report_id: reportId, issue_id: issueId },
        body: { reason: reason || "Author ignored" },
      });
      return res.data;
    },
  };
}

/** 创建单例 API SDK */
let defaultSDK: ApiSDK | null = null;

export function getApiSDK(client?: ApiClient, session?: AuthSession): ApiSDK {
  if (client) {
    return new ApiSDK(client, session);
  }
  if (!defaultSDK) {
    const defaultSession = new AuthSession({ audience: "creator" });
    const defaultClient = new ApiClient({
      baseUrl: "",
      session: defaultSession,
    });
    defaultSDK = new ApiSDK(defaultClient, defaultSession);
  }
  return defaultSDK;
}
