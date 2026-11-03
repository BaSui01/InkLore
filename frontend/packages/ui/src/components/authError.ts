/**
 * @inklore/ui - 统一认证错误格式化工具 (formatAuthError)
 * 将后端返回的 ApiError / 网络异常转化为友好的国际化提示信息，杜绝暴露技术堆栈
 */

import { ApiError } from "@inklore/contracts";

export function formatAuthError(
  err: any,
  t: (key: string, ...args: any[]) => string
): string {
  if (!err) {
    return t("errors.auth.generic_failed");
  }

  // 1. 网络连接失败或超时异常
  if (err instanceof TypeError && (err.message?.includes("fetch") || err.message?.includes("Failed to fetch") || err.message?.includes("NetworkError"))) {
    return t("errors.auth.network_error");
  }
  if (err?.name === "AbortError") {
    return t("errors.auth.network_error");
  }

  // 2. 提取错误码
  const code: string =
    err?.code ||
    (err instanceof ApiError ? err.code : "") ||
    (typeof err?.message === "string" && err.message.startsWith("auth.") ? err.message : "") ||
    "";

  // 对真实验证码挑战的拒绝给出可操作提示，不向用户输出内部校验结构。
  const rejectedField = err?.details?.field;
  const fieldErrors = err?.details?.field_errors;
  const rejectedCaptcha =
    rejectedField === "captcha_code" || rejectedField === "captcha_id" ||
    (Array.isArray(fieldErrors) &&
      fieldErrors.some((item: { field?: string }) => item?.field === "captcha_code" || item?.field === "captcha_id"));
  if (code === "platform.validation_failed" && rejectedCaptcha) {
    return t("auth.captcha_mismatch");
  }

  // 3. 特殊错误码附带动态参数（如限流重试倒计时）
  if (code === "auth.rate_limited") {
    const retrySec =
      err?.retryAfter ??
      err?.details?.retry_after_seconds ??
      err?.details?.retryAfter;
    if (typeof retrySec === "number" && retrySec > 0) {
      return t("errors.auth.rate_limited_retry", retrySec);
    }
    return t("errors.auth.rate_limited");
  }

  // 4. 标准 i18n 映射
  if (code) {
    const i18nKey = `errors.${code}`;
    const translated = t(i18nKey);
    if (translated && translated !== i18nKey) {
      return translated;
    }
  }

  // 5. 兜底错误提示，绝不显示含技术代码（如 [auth.xxx] 或 status: 400）的内部信息
  if (err?.message && typeof err.message === "string" && !err.message.includes("[") && !err.message.includes("status:") && !err.message.includes("errors.")) {
    return err.message;
  }

  return t("errors.auth.generic_failed");
}
