/**
 * @inklore/admin-console - 通用状态指示徽章组件
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 */

import React from "react";
import { Badge } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";

export type SystemStatusKind =
  | "healthy"
  | "cooling"
  | "disabled"
  | "enabled"
  | "open"
  | "closed"
  | "draft"
  | "published"
  | "active";

export interface StatusBadgeProps {
  status: SystemStatusKind | string;
  label?: string;
  size?: "sm" | "md";
}

export function StatusBadge({ status, label, size = "sm" }: StatusBadgeProps) {
  const { t } = useTranslation();

  const normalized = status.toLowerCase();

  let variant: "success" | "warning" | "danger" | "info" | "secondary" = "secondary";
  let defaultLabelKey = "admin.components.status_disabled";

  switch (normalized) {
    case "healthy":
    case "enabled":
    case "closed":
    case "published":
    case "active":
      variant = "success";
      defaultLabelKey = normalized === "healthy" ? "admin.components.status_healthy" : "admin.components.status_enabled";
      break;
    case "cooling":
      variant = "warning";
      defaultLabelKey = "admin.components.status_cooling";
      break;
    case "open":
    case "danger":
      variant = "danger";
      defaultLabelKey = "admin.components.status_cooling";
      break;
    case "draft":
      variant = "info";
      defaultLabelKey = "admin.agents.def_status_draft";
      break;
    case "disabled":
    default:
      variant = "secondary";
      defaultLabelKey = "admin.components.status_disabled";
      break;
  }

  const text = label || t(defaultLabelKey);

  return (
    <Badge variant={variant} size={size}>
      {text}
    </Badge>
  );
}
