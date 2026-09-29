/**
 * @inklore/ui - 统一错误展示器 (ErrorPresenter)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.3
 * 
 * 核心契约与红线：
 * 1. 结构化错误解析：支持 400/401/403/404/409/413/422/429/500 安全转译。
 * 2. 字段级错误明确聚焦与标记，Toast 辅助绝不替代字段错误。
 * 3. 409 版本冲突提供 Resolve Conflict 回调，401 提供重新登录动作。
 * 4. 安全诊断脱敏：严禁对外展示 SQL、密钥、密码、全文正文或原始 Stack。
 * 5. 纯净排版：仅使用 Lucide 矢量图标，杜绝 Emoji。
 */

import React, { useState } from "react";
import {
  ApiError,
  isApiError,
  NetworkError,
  isNetworkError,
  ConflictDetail,
} from "@inklore/contracts";
import { useTranslation, StatusDangerIcon, StatusWarningIcon, ActionRefreshIcon, ActionCopyIcon } from "@inklore/i18n";

export interface ErrorPresenterContext {
  operationLabel?: string;
  resourceLabel?: string;
  hasDraft?: boolean;
}

export interface ErrorPresenterProps {
  error: ApiError | NetworkError | Error | null | unknown;
  context?: ErrorPresenterContext;
  variant?: "inline" | "banner" | "dialog";
  onRetry?: () => void;
  onResolveConflict?: (conflicts: ConflictDetail[]) => void;
  onLogin?: () => void;
}

export function ErrorPresenter({
  error,
  context,
  variant = "banner",
  onRetry,
  onResolveConflict,
  onLogin,
}: ErrorPresenterProps) {
  const { t } = useTranslation();
  const [showDiagnostic, setShowDiagnostic] = useState(false);
  const [copiedDiagnostic, setCopiedDiagnostic] = useState(false);

  if (!error) return null;

  const apiErr = isApiError(error) ? error : null;
  const netErr = isNetworkError(error) ? error : null;
  const httpStatus = apiErr ? apiErr.httpStatus : netErr ? 0 : 500;
  const code = apiErr ? apiErr.code : netErr ? "network_error" : "unknown_error";

  // 解析标题与局部解释
  let title = t(`errors.${code}`);
  if (title === `errors.${code}`) {
    title = t("common.failed");
  }

  // 原始 Error.message 可能包含 URL、正文或请求参数，只展示机器码对应的本地化文案。
  const translatedDescription = apiErr?.message_key ? t(apiErr.message_key) : title;
  const description = apiErr && translatedDescription !== apiErr.message_key
    ? translatedDescription
    : netErr ? t("common.offline") : t("errors.platform.internal_error");

  // 409 版本冲突
  const isConflict = httpStatus === 409;
  const conflicts = apiErr?.details?.conflicts || [];

  // 422 字段错误
  const fieldErrors = apiErr?.details?.field_errors || [];

  // 安全诊断数据复制（不含密钥、正文、密码或堆栈）
  const getSafeDiagnosticText = () => {
    const diagnostic = {
      code,
      http_status: httpStatus,
      request_id: apiErr?.request_id || "N/A",
      trace_id: apiErr?.trace_id || "N/A",
      operation_id: apiErr?.details?.operation_id || "N/A",
      timestamp_utc: new Date().toISOString(),
      operation_label: context?.operationLabel || "N/A",
    };
    return JSON.stringify(diagnostic, null, 2);
  };

  const handleCopyDiagnostic = () => {
    navigator.clipboard.writeText(getSafeDiagnosticText()).then(() => {
      setCopiedDiagnostic(true);
      setTimeout(() => setCopiedDiagnostic(false), 2000);
    });
  };

  return (
    <div
      role="alert"
      aria-live="assertive"
      className={`error-presenter error-presenter-${variant} ${isConflict ? "is-conflict" : ""}`}
      style={{
        border: "1px solid var(--status-danger)",
        backgroundColor: "var(--bg-surface)",
        borderRadius: "8px",
        padding: "16px",
        margin: "12px 0",
        color: "var(--text-primary)",
      }}
    >
      <div style={{ display: "flex", alignItems: "flex-start", gap: "12px" }}>
        <div style={{ color: "var(--status-danger)", marginTop: "2px" }}>
          {isConflict ? <StatusWarningIcon size={20} /> : <StatusDangerIcon size={20} />}
        </div>

        <div style={{ flex: 1 }}>
          <div style={{ fontWeight: 600, fontSize: "15px", marginBottom: "4px" }}>
            {context?.operationLabel ? `${context.operationLabel}: ${title}` : title}
          </div>

          <div style={{ fontSize: "14px", color: "var(--text-secondary)", marginBottom: "8px" }}>
            {description}
          </div>

          {/* 422 字段级错误摘要 */}
          {fieldErrors.length > 0 && (
            <div
              style={{
                backgroundColor: "var(--bg-subtle)",
                borderRadius: "6px",
                padding: "8px 12px",
                margin: "8px 0",
                fontSize: "13px",
              }}
            >
              <div style={{ fontWeight: 500, marginBottom: "4px" }}>{t("common.details")}:</div>
              <ul style={{ margin: 0, paddingLeft: "20px" }}>
                {fieldErrors.map((f, idx) => (
                  <li key={idx}>
                    <strong>{f.field}</strong>: {f.reason}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* 409 冲突指引 */}
          {isConflict && (
            <div style={{ margin: "8px 0", fontSize: "13px", color: "var(--text-muted)" }}>
              {t("errors.content.version_conflict")}
            </div>
          )}

          {/* 动作区 */}
          <div style={{ display: "flex", alignItems: "center", gap: "10px", marginTop: "12px" }}>
            {isConflict && onResolveConflict && (
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => onResolveConflict(conflicts)}
                style={{
                  backgroundColor: "var(--action-primary)",
                  color: "var(--action-on-primary)",
                  border: "none",
                  padding: "6px 14px",
                  borderRadius: "6px",
                  cursor: "pointer",
                  fontSize: "13px",
                }}
              >
                {t("diff.title")}
              </button>
            )}

            {httpStatus === 401 && onLogin && (
              <button
                type="button"
                className="btn btn-primary"
                onClick={onLogin}
                style={{
                  backgroundColor: "var(--action-primary)",
                  color: "var(--action-on-primary)",
                  border: "none",
                  padding: "6px 14px",
                  borderRadius: "6px",
                  cursor: "pointer",
                  fontSize: "13px",
                }}
              >
                {t("auth.login")}
              </button>
            )}

            {onRetry && httpStatus !== 401 && httpStatus !== 403 && (
              <button
                type="button"
                className="btn btn-secondary"
                onClick={onRetry}
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: "6px",
                  backgroundColor: "var(--bg-subtle)",
                  color: "var(--text-primary)",
                  border: "1px solid var(--border-control)",
                  padding: "6px 12px",
                  borderRadius: "6px",
                  cursor: "pointer",
                  fontSize: "13px",
                }}
              >
                <ActionRefreshIcon size={14} />
                {t("common.retry")}
              </button>
            )}

            <button
              type="button"
              onClick={() => setShowDiagnostic(!showDiagnostic)}
              style={{
                background: "none",
                border: "none",
                color: "var(--text-muted)",
                fontSize: "12px",
                textDecoration: "underline",
                cursor: "pointer",
                padding: 0,
              }}
            >
              {showDiagnostic ? t("common.close") : t("common.details")}
            </button>
          </div>

          {/* 安全脱敏诊断区 */}
          {showDiagnostic && (
            <div
              style={{
                marginTop: "10px",
                padding: "10px",
                backgroundColor: "var(--bg-subtle)",
                borderRadius: "6px",
                fontSize: "12px",
                fontFamily: "monospace",
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "6px" }}>
                <span style={{ fontWeight: 600 }}>Diagnostic Packet</span>
                <button
                  type="button"
                  onClick={handleCopyDiagnostic}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "4px",
                    border: "none",
                    background: "none",
                    cursor: "pointer",
                    color: "var(--action-primary)",
                    fontSize: "12px",
                  }}
                >
                  <ActionCopyIcon size={12} />
                  {copiedDiagnostic ? t("common.copied") : t("common.copy")}
                </button>
              </div>
              <pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>{getSafeDiagnosticText()}</pre>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
