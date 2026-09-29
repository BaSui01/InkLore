/**
 * @inklore/contracts - 错误契约结构化定义
 * 严格对齐 internal/platform/errors 与 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md
 * 拒绝通过非标准 message 判断业务，采用机器代码 code、message_key 与 typed details
 */

import { parseRevision, isValidRevision } from "./types";

/** 字段级校验错误 (HTTP 422) */
export interface FieldError {
  field: string;
  reason: string;
}

/** 业务事实冲突详情 (HTTP 409) */
export interface ConflictDetail {
  resource_id: string;
  reason: string;
  current_revision: number;
  subject_id?: string;
  fact_key?: string;
  current_event_id?: string;
  proposal_index?: number;
}

/** 结构化错误安全详情 */
export interface ApiErrorDetails {
  current_revision?: number;
  retry_after_seconds?: number;
  operation_id?: string;
  field_errors?: FieldError[];
  conflicts?: ConflictDetail[];
  [key: string]: unknown;
}

export interface ApiErrorInit {
  httpStatus: number;
  code: string;
  message_key?: string;
  details?: ApiErrorDetails;
  request_id?: string;
  trace_id?: string;
  retryAfter?: number;
}

/**
 * 平台统一结构化 API 错误对象
 */
export class ApiError extends Error {
  readonly httpStatus: number;
  readonly code: string;
  readonly message_key: string;
  readonly details: ApiErrorDetails;
  readonly request_id: string;
  readonly trace_id: string;
  readonly retryAfter?: number;

  constructor(init: ApiErrorInit) {
    const code = init.code || "platform.internal_error";
    const msgKey = init.message_key || `errors.${code}`;
    super(`[${code}] ${msgKey} (status: ${init.httpStatus})`);
    this.name = "ApiError";
    this.httpStatus = init.httpStatus;
    this.code = code;
    this.message_key = msgKey;
    this.details = init.details || {};
    this.request_id = init.request_id || "";
    this.trace_id = init.trace_id || "";
    this.retryAfter = init.retryAfter ?? init.details?.retry_after_seconds;

    // 严格检查 current_revision 如果存在必须在安全整数范围
    if (this.details.current_revision !== undefined && !isValidRevision(this.details.current_revision)) {
      this.details.current_revision = undefined;
    }

    Object.setPrototypeOf(this, ApiError.prototype);
  }

  /** 获取首个字段错误（如 422 表单校验失败） */
  getFirstFieldError(): FieldError | null {
    if (this.details.field_errors && this.details.field_errors.length > 0) {
      return this.details.field_errors[0];
    }
    return null;
  }

  /** 获取特定字段的错误提示原因 */
  getFieldError(fieldName: string): string | null {
    if (!this.details.field_errors) return null;
    const match = this.details.field_errors.find(
      (f) => f.field === fieldName || f.field === `/${fieldName}`
    );
    return match ? match.reason : null;
  }
}

/** 判断是否为 ApiError */
export function isApiError(err: unknown): err is ApiError {
  return err instanceof ApiError;
}

/** 是否为 401 未认证 */
export function isUnauthorizedError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 401;
}

/** 是否为 403 权限不足或跨租户 */
export function isForbiddenError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 403;
}

/** 是否为 404 资源不存在 */
export function isNotFoundError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 404;
}

/** 是否为 409 状态/版本冲突 */
export function isConflictError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 409;
}

/** 是否为 410 事件游标已过期 */
export function isCursorExpiredError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 410;
}

/** 是否为 422 数据校验/规则验证失败 */
export function isValidationError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 422;
}

/** 是否为 429 频控/预算耗尽 */
export function isRateLimitedError(err: unknown): boolean {
  return isApiError(err) && err.httpStatus === 429;
}

/** 是否为网络中断或无响应错误 */
export class NetworkError extends Error {
  readonly isNetworkError = true;
  constructor(message: string, readonly originalError?: unknown) {
    super(message);
    this.name = "NetworkError";
    Object.setPrototypeOf(this, NetworkError.prototype);
  }
}

export function isNetworkError(err: unknown): err is NetworkError {
  return err instanceof NetworkError;
}
