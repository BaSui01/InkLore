/**
 * @inklore/admin-console - 上游模型选择与导入面板 (UpstreamModelPicker)
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 */

import { useState, useMemo } from "react";
import { Sparkles, Search, CheckSquare, Square, X } from "lucide-react";
import { Button } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import type { UpstreamModelItem } from "../api/adminApi";

export interface UpstreamModelPickerProps {
  open: boolean;
  upstreamModels: UpstreamModelItem[];
  selectedIds: string[];
  onToggleSelect: (id: string) => void;
  onSelectAll: () => void;
  onClearAll: () => void;
  onImport: () => void;
  onClose: () => void;
}

export function UpstreamModelPicker({
  open,
  upstreamModels,
  selectedIds,
  onToggleSelect,
  onSelectAll,
  onClearAll,
  onImport,
  onClose,
}: UpstreamModelPickerProps) {
  const { t } = useTranslation();
  const [searchQuery, setSearchQuery] = useState("");

  const filteredModels = useMemo(() => {
    if (!searchQuery.trim()) return upstreamModels;
    const q = searchQuery.toLowerCase();
    return upstreamModels.filter(
      (m) =>
        m.id.toLowerCase().includes(q) ||
        (m.name && m.name.toLowerCase().includes(q))
    );
  }, [upstreamModels, searchQuery]);

  if (!open || upstreamModels.length === 0) return null;

  return (
    <div className="space-y-2.5 p-3 rounded-xl bg-[var(--bg-canvas)] border border-[var(--action-primary)]/30 transition-all">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-2 border-b border-[var(--border-decorative)]">
        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-[var(--action-primary)]" />
          <span className="font-semibold text-xs text-[var(--text-primary)]">
            {t("admin.channels.upstream_models_title")} ({upstreamModels.length})
          </span>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative w-44">
            <Search className="w-3 h-3 absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={t("admin.components.search_ph")}
              className="w-full pl-7 pr-2 py-1 text-[11px] rounded-lg bg-[var(--bg-surface)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)]"
            />
          </div>
          <Button
            type="button"
            size="sm"
            variant="primary"
            onClick={onImport}
            disabled={selectedIds.length === 0}
          >
            {t("admin.channels.import_selected_models")} ({selectedIds.length})
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={onClose}
          >
            <X className="w-3.5 h-3.5" />
          </Button>
        </div>
      </div>

      {/* 快捷全选 / 全不选 */}
      <div className="flex items-center gap-3 text-[11px] px-1 text-[var(--text-muted)]">
        <button
          type="button"
          onClick={onSelectAll}
          className="hover:text-[var(--action-primary)] cursor-pointer flex items-center gap-1"
        >
          <CheckSquare className="w-3 h-3" />
          <span>{t("admin.channels.select_all_upstream")}</span>
        </button>
        <span>|</span>
        <button
          type="button"
          onClick={onClearAll}
          className="hover:text-[var(--status-danger)] cursor-pointer flex items-center gap-1"
        >
          <Square className="w-3 h-3" />
          <span>{t("admin.channels.clear_upstream_selection")}</span>
        </button>
      </div>

      {/* 上游模型列表复选框 */}
      <div className="max-h-40 overflow-y-auto space-y-1 p-2 rounded-lg bg-[var(--bg-surface)] border border-[var(--border-decorative)]">
        {filteredModels.map((item) => {
          const isChecked = selectedIds.includes(item.id);
          return (
            <label
              key={item.id}
              className="flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-[var(--bg-subtle)] cursor-pointer text-xs transition-colors"
            >
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={isChecked}
                  onChange={() => onToggleSelect(item.id)}
                  className="rounded accent-[var(--action-primary)] cursor-pointer h-3.5 w-3.5"
                />
                <span className="font-mono text-xs text-[var(--text-primary)] font-medium">
                  {item.id}
                </span>
              </div>
              {item.name && item.name !== item.id && (
                <span className="text-[11px] text-[var(--text-muted)]">
                  {item.name}
                </span>
              )}
            </label>
          );
        })}
      </div>
    </div>
  );
}
