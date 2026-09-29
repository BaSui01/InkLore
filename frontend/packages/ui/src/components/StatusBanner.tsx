/**
 * @inklore/ui - 统一状态横幅 (StatusBanner)
 * 严格遵循纯净排版与零 Emoji 铁律
 */

import React from "react";
import {
  StatusSuccessIcon,
  StatusWarningIcon,
  StatusDangerIcon,
  StatusInfoIcon,
} from "@inklore/i18n";

export type BannerTone = "info" | "success" | "warning" | "danger";

export interface StatusBannerProps {
  tone?: BannerTone;
  title?: string;
  children: React.ReactNode;
  action?: React.ReactNode;
  onClose?: () => void;
}

export function StatusBanner({
  tone = "info",
  title,
  children,
  action,
  onClose,
}: StatusBannerProps) {
  const getToneStyles = () => {
    switch (tone) {
      case "success":
        return {
          border: "1px solid var(--status-success)",
          bg: "var(--bg-surface)",
          color: "var(--status-success)",
          icon: <StatusSuccessIcon size={18} />,
        };
      case "warning":
        return {
          border: "1px solid var(--status-warning)",
          bg: "var(--bg-surface)",
          color: "var(--status-warning)",
          icon: <StatusWarningIcon size={18} />,
        };
      case "danger":
        return {
          border: "1px solid var(--status-danger)",
          bg: "var(--bg-surface)",
          color: "var(--status-danger)",
          icon: <StatusDangerIcon size={18} />,
        };
      case "info":
      default:
        return {
          border: "1px solid var(--status-info)",
          bg: "var(--bg-surface)",
          color: "var(--status-info)",
          icon: <StatusInfoIcon size={18} />,
        };
    }
  };

  const currentTone = getToneStyles();

  return (
    <div
      role="status"
      className={`status-banner status-banner-${tone}`}
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        border: currentTone.border,
        backgroundColor: currentTone.bg,
        borderRadius: "6px",
        padding: "10px 16px",
        margin: "8px 0",
        fontSize: "14px",
        color: "var(--text-primary)",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
        <div style={{ color: currentTone.color, display: "flex" }}>
          {currentTone.icon}
        </div>
        <div>
          {title && <span style={{ fontWeight: 600, marginRight: "8px" }}>{title}</span>}
          <span>{children}</span>
        </div>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
        {action}
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            aria-label="Close banner"
            style={{
              background: "none",
              border: "none",
              cursor: "pointer",
              color: "var(--text-muted)",
              fontSize: "16px",
              lineHeight: 1,
            }}
          >
            x
          </button>
        )}
      </div>
    </div>
  );
}
