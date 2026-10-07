/**
 * @inklore/admin-console - 通用管理端页面标题栏组件
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 */

import React from "react";

export interface AdminHeaderProps {
  title: string;
  subtitle?: string;
  badge?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}

export function AdminHeader({ title, subtitle, badge, actions, className = "" }: AdminHeaderProps) {
  return (
    <header className={`h-16 px-6 flex items-center justify-between gap-4 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)] shrink-0 ${className}`}>
      <div className="flex items-center gap-2.5 sm:gap-3 min-w-0">
        <h2 className="text-base sm:text-lg font-bold text-[var(--text-primary)] tracking-tight truncate">
          {title}
        </h2>
        {badge}
        {subtitle && (
          <span className="text-xs text-[var(--text-muted)] truncate hidden md:inline">
            {subtitle}
          </span>
        )}
      </div>
      {actions && (
        <div className="flex items-center gap-2 sm:gap-2.5 shrink-0">
          {actions}
        </div>
      )}
    </header>
  );
}
