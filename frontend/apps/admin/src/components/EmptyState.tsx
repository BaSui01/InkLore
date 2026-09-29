/**
 * @inklore/admin-console - 通用空数据展示组件
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 */

import React from "react";
import { FolderOpen } from "lucide-react";
import { useTranslation } from "@inklore/i18n";

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title?: string;
  description?: string;
  action?: React.ReactNode;
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col items-center justify-center p-12 text-center space-y-3 bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-2xl">
      <div className="w-12 h-12 rounded-2xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-[var(--text-muted)] shadow-xs">
        {icon || <FolderOpen className="w-6 h-6" />}
      </div>
      <div className="space-y-1 max-w-sm">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">
          {title || t("admin.components.empty_title")}
        </h3>
        <p className="text-xs text-[var(--text-muted)] leading-relaxed">
          {description || t("admin.components.empty_desc")}
        </p>
      </div>
      {action && <div className="pt-2 whitespace-nowrap">{action}</div>}
    </div>
  );
}
