/**
 * @inklore/ui - 页面门控组件 (PageGate)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.2
 * 
 * 判定优先级：
 * 未认证 -> authGate
 * 明确 403 -> forbidden
 * 对象 404 -> notFound
 * 无快照首次失败 -> error
 * 无快照请求中 -> loading
 * 成功快照准备就绪 -> ready / empty
 */

import React from "react";
import { SessionSnapshot, ApiError } from "@inklore/contracts";
import { useTranslation, LoadingSpinner, StatusDangerIcon, ShieldIcon, NovelIcon } from "@inklore/i18n";
import { ErrorPresenter } from "./ErrorPresenter";

export type GateState =
  | "unauthenticated"
  | "forbidden"
  | "notFound"
  | "error"
  | "loading"
  | "empty"
  | "ready";

export interface QueryStateLike {
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  data?: unknown;
  isEmpty?: boolean;
}

export interface PageGateProps {
  session: SessionSnapshot;
  resourceQuery?: QueryStateLike;
  requiredCapability?: string;
  children: React.ReactNode;
  onRetry?: () => void;
  onLogin?: () => void;
  emptyFallback?: React.ReactNode;
}

export function PageGate({
  session,
  resourceQuery,
  requiredCapability,
  children,
  onRetry,
  onLogin,
  emptyFallback,
}: PageGateProps) {
  const { t } = useTranslation();

  // 1. 认证状态检查
  if (session.status !== "authenticated" || !session.principal) {
    return (
      <div className="gate-container gate-unauthenticated" style={gateContainerStyle}>
        <div style={iconBoxStyle}>
          <ShieldIcon size={32} color="var(--status-warning)" />
        </div>
        <h2 style={headingStyle}>{t("auth.unauthenticated")}</h2>
        <p style={descStyle}>{t("auth.session_expired")}</p>
        {onLogin && (
          <button
            type="button"
            className="btn btn-primary"
            onClick={onLogin}
            style={primaryBtnStyle}
          >
            {t("auth.login")}
          </button>
        )}
      </div>
    );
  }

  // 2. 资源状态与权限检查
  if (resourceQuery) {
    const { isLoading, isError, error, isEmpty } = resourceQuery;

    // 错误判定
    if (isError && error) {
      const apiErr = error as ApiError;
      if (apiErr?.httpStatus === 403) {
        return (
          <div className="gate-container gate-forbidden" style={gateContainerStyle}>
            <div style={iconBoxStyle}>
              <ShieldIcon size={32} color="var(--status-danger)" />
            </div>
            <h2 style={headingStyle}>{t("auth.forbidden")}</h2>
            <p style={descStyle}>{t("errors.auth.forbidden")}</p>
          </div>
        );
      }

      if (apiErr?.httpStatus === 404) {
        return (
          <div className="gate-container gate-not-found" style={gateContainerStyle}>
            <div style={iconBoxStyle}>
              <NovelIcon size={32} color="var(--text-muted)" />
            </div>
            <h2 style={headingStyle}>{t("errors.content.not_found")}</h2>
            <p style={descStyle}>{t("common.empty")}</p>
            {onRetry && (
              <button type="button" onClick={onRetry} style={secondaryBtnStyle}>
                {t("common.retry")}
              </button>
            )}
          </div>
        );
      }

      return (
        <div style={{ padding: "24px" }}>
          <ErrorPresenter error={error} onRetry={onRetry} onLogin={onLogin} />
        </div>
      );
    }

    // 加载中判定
    if (isLoading) {
      return (
        <div className="gate-container gate-loading" style={gateContainerStyle}>
          <LoadingSpinner size={32} />
          <p style={{ marginTop: "16px", color: "var(--text-secondary)", fontSize: "14px" }}>
            {t("common.loading")}
          </p>
        </div>
      );
    }

    // 空数据判定
    if (isEmpty) {
      return (
        emptyFallback || (
          <div className="gate-container gate-empty" style={gateContainerStyle}>
            <div style={iconBoxStyle}>
              <NovelIcon size={32} color="var(--text-muted)" />
            </div>
            <h3 style={headingStyle}>{t("common.empty")}</h3>
          </div>
        )
      );
    }
  }

  return <>{children}</>;
}

const gateContainerStyle: React.CSSProperties = {
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  justifyContent: "center",
  padding: "64px 24px",
  textAlign: "center",
  backgroundColor: "var(--bg-canvas)",
  minHeight: "320px",
};

const iconBoxStyle: React.CSSProperties = {
  marginBottom: "16px",
};

const headingStyle: React.CSSProperties = {
  fontSize: "18px",
  fontWeight: 600,
  color: "var(--text-primary)",
  margin: "0 0 8px 0",
};

const descStyle: React.CSSProperties = {
  fontSize: "14px",
  color: "var(--text-secondary)",
  margin: "0 0 20px 0",
};

const primaryBtnStyle: React.CSSProperties = {
  backgroundColor: "var(--action-primary)",
  color: "var(--action-on-primary)",
  border: "none",
  padding: "8px 20px",
  borderRadius: "6px",
  fontSize: "14px",
  cursor: "pointer",
};

const secondaryBtnStyle: React.CSSProperties = {
  backgroundColor: "var(--bg-subtle)",
  color: "var(--text-primary)",
  border: "1px solid var(--border-control)",
  padding: "8px 18px",
  borderRadius: "6px",
  fontSize: "14px",
  cursor: "pointer",
};
