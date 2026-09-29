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

export function AdminHeader({ title, badge, actions, className = "" }: AdminHeaderProps) {
  return (
    <div className={`flex flex-col sm:flex-row sm:items-center justify-between gap-3.5 pb-4 sm:pb-5 border-b border-[var(--border-decorative)] shrink-0 ${className}`}>
      <div className="space-y-1 min-w-0">
        <div className="flex items-center gap-2.5 sm:gap-3 flex-wrap">
          <h2 className="text-lg sm:text-xl font-bold text-[var(--text-primary)] tracking-tight">
            {title}
          </h2>
          {badge}
        </div>
      </div>
      {actions && (
        <div className="flex items-center gap-2 sm:gap-2.5 flex-wrap sm:justify-end shrink-0 w-full sm:w-auto">
          {actions}
        </div>
      )}
    </div>
  );
}
