/**
 * @inklore/contracts - 传输外包封套定义
 * 严格对齐 internal/platform/response/contract.go
 */

import { ApiErrorDetails } from "./errors";

/** 成功统一外包封套 */
export interface SuccessEnvelope<T = unknown> {
  code: "ok" | "success";
  data: T;
  request_id: string;
  trace_id: string;
}

/** 失败统一外包封套 */
export interface ErrorEnvelope {
  code: string;
  message_key: string;
  details: ApiErrorDetails;
  request_id: string;
  trace_id: string;
}

/** 统一分页列表结构 */
export interface PagedList<T> {
  items: T[];
  next_cursor: string | null;
  has_more: boolean;
  total?: number;
}

/** 二进制下载产物定义 */
export interface DownloadResult {
  blob: Blob;
  filename: string;
  mediaType: string;
  sizeBytes: number;
  sha256?: string;
}
