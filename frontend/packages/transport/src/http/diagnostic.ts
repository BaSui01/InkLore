/** 仅输出受限机器诊断；绝不序列化原始异常、正文、事件或凭据。 */
import { isApiError, isNetworkError } from "@inklore/contracts";

export function safeErrorText(error: unknown, translate: (key: string) => string): string {
  if (isApiError(error)) {
    const key = `errors.${error.code}`;
    if (/^errors\.[a-z][a-z0-9_.]{0,79}$/.test(key)) {
      const text = translate(key);
      if (text !== key) return text;
    }
  }
  return isNetworkError(error) ? translate("common.offline") : translate("errors.platform.internal_error");
}

export function reportSafeDiagnostic(event: string, error?: unknown): void {
  const diagnostic: Record<string, string | number> = { event };
  if (isApiError(error)) {
    diagnostic.code = /^[a-z][a-z0-9_.]{0,79}$/.test(error.code) ? error.code : "platform.internal_error";
    diagnostic.http_status = error.httpStatus;
    if (/^[a-f0-9-]{36}$/.test(error.request_id)) diagnostic.request_id = error.request_id;
    if (/^[a-f0-9]{32}$/.test(error.trace_id)) diagnostic.trace_id = error.trace_id;
  } else if (isNetworkError(error)) {
    diagnostic.code = "network_error";
  } else {
    diagnostic.code = "platform.internal_error";
  }
  console.warn("InkLore diagnostic", diagnostic);
}
