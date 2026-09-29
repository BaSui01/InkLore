/**
 * @inklore/admin-console - 通用搜索与筛选栏组件
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 */

import React from "react";
import { Search, X, RotateCcw } from "lucide-react";
import { useTranslation } from "@inklore/i18n";

export interface FilterOption {
  label: string;
  value: string;
}

export interface FilterItem {
  id: string;
  value: string;
  options: FilterOption[];
  onChange: (val: string) => void;
  ariaLabel?: string;
}

export interface SearchFilterBarProps {
  searchQuery: string;
  onSearchChange: (val: string) => void;
  searchPlaceholder?: string;
  filters?: FilterItem[];
  onReset?: () => void;
  showReset?: boolean;
  actions?: React.ReactNode;
  className?: string;
}

export function SearchFilterBar({
  searchQuery,
  onSearchChange,
  searchPlaceholder,
  filters = [],
  onReset,
  showReset,
  actions,
  className = "p-3 bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-xl shadow-sm",
}: SearchFilterBarProps) {
  const { t } = useTranslation();

  return (
    <div className={`flex flex-col sm:flex-row sm:items-center justify-between gap-3 ${className}`}>
      {/* Left: Search input + Filters */}
      <div className="flex flex-col sm:flex-row flex-1 sm:items-center gap-2.5 min-w-0">
        {/* Search input: 窄视口全宽，大视口自适应 */}
        <div className="relative w-full sm:w-auto sm:flex-1 sm:min-w-[200px] max-w-md">
          <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={searchPlaceholder || t("admin.components.search_ph")}
            className="w-full pl-9 pr-8 h-9 text-xs rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--action-primary)] focus:ring-2 focus:ring-[var(--focus-ring)]/15 transition-all font-sans"
          />
          {searchQuery && (
            <button
              onClick={() => onSearchChange("")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              aria-label="Clear search"
            >
              <X className="w-3 h-3" />
            </button>
          )}
        </div>

        {/* Dropdown Filters + Reset */}
        {(filters.length > 0 || (showReset && onReset)) && (
          <div className="flex items-center gap-2 flex-wrap w-full sm:w-auto">
            {filters.map((filter) => (
              <select
                key={filter.id}
                value={filter.value}
                onChange={(e) => filter.onChange(e.target.value)}
                aria-label={filter.ariaLabel || filter.id}
                className="h-9 px-3 text-xs rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] focus:ring-2 focus:ring-[var(--focus-ring)]/15 transition-colors cursor-pointer whitespace-nowrap flex-1 sm:flex-initial"
              >
                {filter.options.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            ))}

            {/* Reset Filters button */}
            {showReset && onReset && (
              <button
                onClick={onReset}
                className="flex items-center gap-1.5 h-9 px-2.5 text-xs rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer whitespace-nowrap shrink-0"
                title={t("admin.components.reset_filters")}
              >
                <RotateCcw className="w-3 h-3" />
                <span>{t("admin.components.reset_filters")}</span>
              </button>
            )}
          </div>
        )}
      </div>

      {/* Right: Actions slot */}
      {actions && (
        <div className="flex items-center gap-2 shrink-0 flex-wrap">
          {actions}
        </div>
      )}
    </div>
  );
}
