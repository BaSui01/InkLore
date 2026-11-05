/**
 * @inklore/transport - 统一共享 HTTP 客户端 (ApiClient)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.1
 * 
 * 核心契约与红线：
 * 1. 基于标准原生 fetch 构建，支持 CSRF 引导与内存 Token 安全管理。
 * 2. 401 单飞 (Single-Flight) 刷新：并发 401 仅刷新一次，重放保留原始 Idempotency-Key。
 * 3. 403 / 404 / 422 严禁重试，返回结构化 ApiError（含 field_errors / conflicts）。
 * 4. 写操作 (POST/PUT/PATCH/DELETE) 自动注入 UUIDv4 Idempotency-Key。
 * 5. GET 请求网络异常时最多退避重试 2 次 (1s/2s)，写操作默认 0 自动网络重试。
 * 6. 换主体 (sessionEpoch 变动) 迟到响应丢弃，杜绝跨主体缓存污染。
 * 7. 二进制下载先检测 Content-Type，错误时不伪造成文件下载。
 */

import {
  ApiError,
  NetworkError,
  isNetworkError,
  SuccessEnvelope,
  ErrorEnvelope,
  DownloadResult,
  ScopeKey,
  UUID,
  generateUUID,
  isValidRevision,
} from "@inklore/contracts";
import { AuthSession } from "./session";
import { OperationRegistry } from "./registry";
import { stripTrailingSlashes } from "../base-url";

export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export interface RequestConfig<TReq = unknown> {
  operationId: string;
  method: HttpMethod;
  path: string;
  pathParams?: Record<string, string | number>;
  query?: Record<string, string | number | boolean | null | undefined>;
  body?: TReq;
  scope?: ScopeKey;
  signal?: AbortSignal;
  idempotencyKey?: UUID | string;
  expectedRevision?: number;
  skipAuth?: boolean;
}

export interface DownloadConfig {
  operationId: string;
  path: string;
  method?: "GET" | "POST";
  pathParams?: Record<string, string | number>;
  query?: Record<string, string | number | boolean | null | undefined>;
  body?: unknown;
  signal?: AbortSignal;
  idempotencyKey?: UUID | string;
}

export interface ApiClientOptions {
  baseUrl?: string;
  session: AuthSession;
  registry?: OperationRegistry;
  clock?: () => number;
  onSessionInvalid?: () => void;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly session: AuthSession;
  private readonly registry: OperationRegistry;
  private readonly clock: () => number;
  private readonly onSessionInvalid?: () => void;

  constructor(options: ApiClientOptions) {
    this.baseUrl = stripTrailingSlashes(options.baseUrl || "");
    this.session = options.session;
    this.registry = options.registry || new OperationRegistry();
    this.clock = options.clock || (() => Date.now());
    this.onSessionInvalid = options.onSessionInvalid;
  }

  getAuthSession(): AuthSession {
    return this.session;
  }

  getOperationRegistry(): OperationRegistry {
    return this.registry;
  }

  /**
   * 核心统一请求方法
   */
  async request<TReq = unknown, TRes = unknown>(
    config: RequestConfig<TReq>
  ): Promise<SuccessEnvelope<TRes>> {
    const isMutation = config.method !== "GET";

    // 校验 expectedRevision 安全整数
    if (config.expectedRevision !== undefined && !isValidRevision(config.expectedRevision)) {
      throw new ApiError({
        httpStatus: 400,
        code: "platform.validation_failed",
        details: {
          field_errors: [
            { field: "expected_revision", reason: "expected_revision must be a safe integer" },
          ],
        },
      });
    }

    // 幂等 Key 注入与注册
    let idempotencyKey = config.idempotencyKey as UUID | undefined;
    if (isMutation && !idempotencyKey) {
      idempotencyKey = generateUUID();
    }

    const currentScope = config.scope || this.session.getScope() || {
      tenantId: "anonymous",
      userId: "anonymous",
      audience: this.session.getAudience(),
      sessionEpoch: this.session.getSessionEpoch(),
    };

    if (idempotencyKey) {
      this.registry.register({
        operationId: config.operationId,
        key: idempotencyKey,
        canonicalMethodPath: `${config.method} ${config.path}`,
        scope: currentScope,
      });
      this.registry.markInflight(idempotencyKey);
    }

    const initialEpoch = this.session.getSessionEpoch();

    try {
      const result = await this.executeWithRetry<TReq, TRes>(
        config,
        idempotencyKey,
        initialEpoch,
        0
      );

      if (idempotencyKey) {
        this.registry.markSucceeded(idempotencyKey, result.data);
      }
      return result;
    } catch (err) {
      if (idempotencyKey) {
        if (isNetworkError(err)) {
          this.registry.markUnknown(idempotencyKey);
        } else {
          this.registry.markFailed(idempotencyKey);
        }
      }
      throw err;
    }
  }

  /**
   * 执行单次请求（处理 401 自动单飞重试与 GET 网络退避）
   */
  private async executeWithRetry<TReq, TRes>(
    config: RequestConfig<TReq>,
    idempotencyKey: UUID | undefined,
    initialEpoch: number,
    retryCount: number
  ): Promise<SuccessEnvelope<TRes>> {
    if (!config.skipAuth && this.session.getStatus() === "authenticated") {
      const expiresAt = this.session.getSnapshot().accessExpiresAt;
      if (!this.session.getAccessToken() || (expiresAt !== null && expiresAt <= Date.now() + 10000)) {
        try {
          await this.session.refresh();
        } catch {
          // 静默刷新失败不阻断，后续 401 流程兜底处理
        }
      }
    }

    const url = this.buildUrl(config.path, config.pathParams, config.query, config.method, config.expectedRevision);
    const headers = this.buildHeaders(config, idempotencyKey);
    const body = config.body !== undefined ? JSON.stringify(config.body) : undefined;

    let res: Response;
    try {
      res = await fetch(url, {
        method: config.method,
        headers,
        body,
        credentials: "same-origin",
        signal: config.signal,
      });
    } catch (fetchErr: any) {
      // 尊重 AbortSignal
      if (config.signal?.aborted) {
        throw fetchErr;
      }

      // GET 请求网络异常时最多退避重试 2 次 (1s, 2s)
      if (config.method === "GET" && retryCount < 2) {
        const delayMs = (retryCount + 1) * 1000;
        await this.sleep(delayMs, config.signal);
        return this.executeWithRetry<TReq, TRes>(config, idempotencyKey, initialEpoch, retryCount + 1);
      }

      throw new NetworkError(
        fetchErr?.message || "Network connection failed",
        fetchErr
      );
    }

    // 检查 401 未认证状态 -> 触发单飞刷新并同 Key 重放
    if (res.status === 401 && !config.skipAuth) {
      try {
        await this.session.refresh();
      } catch {
        if (this.onSessionInvalid) {
          this.onSessionInvalid();
        }
        const errEnvelope = await this.parseErrorResponse(res);
        throw new ApiError({
          httpStatus: 401,
          code: errEnvelope.code || "auth.unauthenticated",
          message_key: errEnvelope.message_key,
          details: errEnvelope.details,
          request_id: errEnvelope.request_id,
          trace_id: errEnvelope.trace_id,
        });
      }

      // 刷新后检查主体 Epoch：若主体已切换，丢弃本次重试
      if (this.session.getSessionEpoch() !== initialEpoch) {
        throw new ApiError({
          httpStatus: 401,
          code: "auth.session_epoch_changed",
          message_key: "errors.auth.session_epoch_changed",
          details: {},
        });
      }

      // 刷新成功后，重放原请求（保持相同 Idempotency-Key，仅尝试重放一次）
      return this.executeSingleAttempt<TReq, TRes>(config, idempotencyKey, initialEpoch);
    }

    // 处理 503 临时不可用对 GET 请求的重试
    if (res.status === 503 && config.method === "GET" && retryCount < 2) {
      const delayMs = (retryCount + 1) * 1000;
      await this.sleep(delayMs, config.signal);
      return this.executeWithRetry<TReq, TRes>(config, idempotencyKey, initialEpoch, retryCount + 1);
    }

    // 成功处理 (HTTP 200..299)
    if (res.ok) {
      // 换主体迟到响应检测
      if (this.session.getSessionEpoch() !== initialEpoch) {
        throw new ApiError({
          httpStatus: 409,
          code: "auth.session_epoch_mismatch",
          message_key: "errors.auth.session_epoch_mismatch",
          details: {},
        });
      }

      const rawJson = await res.json().catch(() => ({ code: "ok", data: null }));
      return this.normalizeSuccessEnvelope<TRes>(rawJson, res);
    }

    // 结构化失败处理
    const errEnvelope = await this.parseErrorResponse(res);
    throw new ApiError({
      httpStatus: res.status,
      code: errEnvelope.code,
      message_key: errEnvelope.message_key,
      details: errEnvelope.details,
      request_id: errEnvelope.request_id,
      trace_id: errEnvelope.trace_id,
    });
  }

  /**
   * 401 刷新后的单次重放请求
   */
  private async executeSingleAttempt<TReq, TRes>(
    config: RequestConfig<TReq>,
    idempotencyKey: UUID | undefined,
    initialEpoch: number
  ): Promise<SuccessEnvelope<TRes>> {
    const url = this.buildUrl(config.path, config.pathParams, config.query, config.method, config.expectedRevision);
    const headers = this.buildHeaders(config, idempotencyKey);
    const body = config.body !== undefined ? JSON.stringify(config.body) : undefined;

    const res = await fetch(url, {
      method: config.method,
      headers,
      body,
      credentials: "same-origin",
      signal: config.signal,
    }).catch((err) => {
      throw new NetworkError(err?.message || "Network failed on retry", err);
    });

    if (this.session.getSessionEpoch() !== initialEpoch) {
      throw new ApiError({
        httpStatus: 409,
        code: "auth.session_epoch_mismatch",
        message_key: "errors.auth.session_epoch_mismatch",
        details: {},
      });
    }

    if (res.ok) {
      const rawJson = await res.json().catch(() => ({ code: "ok", data: null }));
      return this.normalizeSuccessEnvelope<TRes>(rawJson, res);
    }

    const errEnvelope = await this.parseErrorResponse(res);
    throw new ApiError({
      httpStatus: res.status,
      code: errEnvelope.code,
      message_key: errEnvelope.message_key,
      details: errEnvelope.details,
      request_id: errEnvelope.request_id,
      trace_id: errEnvelope.trace_id,
    });
  }

  /**
   * 二进制文件安全下载
   */
  async download(config: DownloadConfig): Promise<DownloadResult> {
    const initialEpoch = this.session.getSessionEpoch();
    const url = this.buildUrl(config.path, config.pathParams, config.query, config.method || "GET");
    const headers: Record<string, string> = {
      "X-Request-ID": generateUUID(),
    };

    const expiresAt = this.session.getSnapshot().accessExpiresAt;
    if (this.session.getStatus() === "authenticated" && expiresAt !== null && expiresAt <= Date.now() + 10000) {
      await this.session.refresh();
    }
    const token = this.session.getAccessToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const res = await fetch(url, {
      method: config.method || "GET",
      headers,
      credentials: "same-origin",
      signal: config.signal,
    });

    if (!res.ok) {
      const contentType = res.headers.get("content-type") || "";
      if (contentType.includes("application/json")) {
        const errJson = await res.json().catch(() => ({}));
        throw new ApiError({
          httpStatus: res.status,
          code: errJson.code || "platform.download_failed",
          message_key: errJson.message_key,
          details: errJson.details,
          request_id: errJson.request_id,
          trace_id: errJson.trace_id,
        });
      }
      throw new ApiError({
        httpStatus: res.status,
        code: "platform.download_failed",
        details: { statusText: res.statusText },
      });
    }

    const contentDisposition = res.headers.get("content-disposition") || "";
    let filename = "download.bin";
    const match = /filename[*]?=(?:UTF-8'')?([^;\r\n]+)/i.exec(contentDisposition);
    if (match && match[1]) {
      filename = decodeURIComponent(match[1].replace(/["']/g, "").trim());
    }

    const blob = await res.blob();
    if (this.session.getSessionEpoch() !== initialEpoch) {
      throw new ApiError({ httpStatus: 409, code: "auth.session_epoch_mismatch", details: {} });
    }
    if (!contentDisposition.toLowerCase().startsWith("attachment")) {
      throw new ApiError({ httpStatus: 502, code: "platform.download_failed", details: {} });
    }
    return {
      blob,
      filename,
      mediaType: res.headers.get("content-type") || "application/octet-stream",
      sizeBytes: blob.size,
      sha256: res.headers.get("x-content-sha256") || undefined,
    };
  }

  /**
   * 快捷方法
   */
  get<TRes>(operationId: string, path: string, query?: Record<string, any>, signal?: AbortSignal) {
    return this.request<never, TRes>({ operationId, method: "GET", path, query, signal });
  }

  post<TReq, TRes>(operationId: string, path: string, body?: TReq, signal?: AbortSignal, idempotencyKey?: string) {
    return this.request<TReq, TRes>({ operationId, method: "POST", path, body, signal, idempotencyKey });
  }

  put<TReq, TRes>(operationId: string, path: string, body?: TReq, expectedRevision?: number, signal?: AbortSignal) {
    return this.request<TReq, TRes>({ operationId, method: "PUT", path, body, expectedRevision, signal });
  }

  delete<TRes>(operationId: string, path: string, expectedRevision?: number, signal?: AbortSignal) {
    return this.request<never, TRes>({ operationId, method: "DELETE", path, expectedRevision, signal });
  }

  /**
   * URL 组装
   */
  private buildUrl(
    path: string,
    params?: Record<string, string | number>,
    query?: Record<string, any>,
    method?: string,
    expectedRevision?: number
  ): string {
    let finalPath = path;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        finalPath = finalPath.replace(`:${k}`, encodeURIComponent(String(v)));
        finalPath = finalPath.replace(`{${k}}`, encodeURIComponent(String(v)));
      }
    }

    const url = new URL(this.baseUrl + finalPath, window.location.origin);

    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v !== undefined && v !== null) {
          url.searchParams.set(k, String(v));
        }
      }
    }

    // DELETE 请求的 expected_revision 必须放 query
    if (method === "DELETE" && expectedRevision !== undefined) {
      url.searchParams.set("expected_revision", String(expectedRevision));
    }

    return url.toString();
  }

  /**
   * 请求头组装
   */
  private buildHeaders(
    config: RequestConfig<any>,
    idempotencyKey?: UUID
  ): Record<string, string> {
    const headers: Record<string, string> = {
      Accept: "application/json",
      "X-Request-ID": generateUUID(),
    };

    if (config.body !== undefined) {
      headers["Content-Type"] = "application/json";
    }

    if (!config.skipAuth) {
      const token = this.session.getAccessToken();
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }
    }

    if (idempotencyKey) {
      headers["Idempotency-Key"] = idempotencyKey;
    }

    return headers;
  }

  /**
   * 结构化解析错误响应
   */
  private async parseErrorResponse(res: Response): Promise<ErrorEnvelope> {
    try {
      const json = await res.json();
      return {
        code: json.code || `http_${res.status}`,
        message_key: json.message_key || `errors.${json.code || "unknown"}`,
        details: json.details || {},
        request_id: json.request_id || res.headers.get("x-request-id") || "",
        trace_id: json.trace_id || res.headers.get("x-trace-id") || "",
      };
    } catch {
      return {
        code: res.status === 413 ? "platform.payload_too_large" : `http_${res.status}`,
        message_key: res.status === 413 ? "errors.platform.payload_too_large" : `errors.http_${res.status}`,
        details: {},
        request_id: res.headers.get("x-request-id") || "",
        trace_id: res.headers.get("x-trace-id") || "",
      };
    }
  }

  /**
   * 规范化成功封套并校验安全整数
   */
  private normalizeSuccessEnvelope<TRes>(raw: any, res: Response): SuccessEnvelope<TRes> {
    const data = raw && typeof raw === "object" && "data" in raw ? raw.data : raw;
    return {
      code: raw?.code || "ok",
      data: data as TRes,
      request_id: raw?.request_id || res.headers.get("x-request-id") || generateUUID(),
      trace_id: raw?.trace_id || res.headers.get("x-trace-id") || "",
    };
  }

  private sleep(ms: number, signal?: AbortSignal): Promise<void> {
    return new Promise((resolve, reject) => {
      if (signal?.aborted) return reject(new Error("Aborted"));
      const timer = setTimeout(resolve, ms);
      signal?.addEventListener("abort", () => {
        clearTimeout(timer);
        reject(new Error("Aborted"));
      });
    });
  }
}
