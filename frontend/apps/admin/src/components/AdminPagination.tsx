/**
 * @inklore/admin-console - 通用管理端分页控制器组件
 * 严格遵循 Zero Hardcoding Contract 与 No Emoji Policy
 * 全管理端统一复用：完善支持总数统计、页码范围、每页条数切换、数字页码省略、快速跳页与游标流式分页
 */

import React, { useMemo, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "@inklore/i18n";

export interface AdminPaginationProps {
  /** 总条数（适用于总数已知模式；未传入时自动转入游标/流式分页模式） */
  total?: number;
  /** 当前页码（1-indexed） */
  page: number;
  /** 每页条数 */
  pageSize: number;
  /** 页码变化回调（总数分页模式） */
  onPageChange?: (page: number) => void;
  /** 每页条数变化回调 */
  onPageSizeChange?: (pageSize: number) => void;
  /** 每页条数选项列表，默认 [10, 20, 50, 100] */
  pageSizeOptions?: number[];

  /** 游标/流式分页模式扩展参数 */
  hasMore?: boolean;
  hasPrev?: boolean;
  onPrevPage?: () => void;
  onNextPage?: () => void;
  loading?: boolean;

  /** 自定义外层样式 */
  className?: string;
}

export function AdminPagination({
  total,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [10, 20, 50, 100],
  hasMore = false,
  hasPrev,
  onPrevPage,
  onNextPage,
  loading = false,
  className = "",
}: AdminPaginationProps) {
  const { t } = useTranslation();
  const [jumpInput, setJumpInput] = useState("");

  const isTotalMode = typeof total === "number";
  const totalCount = isTotalMode ? Math.max(0, total!) : 0;
  const totalPages = isTotalMode ? Math.max(1, Math.ceil(totalCount / pageSize)) : 1;
  const safePage = isTotalMode ? Math.min(Math.max(1, page), totalPages) : Math.max(1, page);

  const start = totalCount === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const end = Math.min(totalCount, safePage * pageSize);

  // 数字模式下的页码生成（带省略号）
  const pageNumbers = useMemo(() => {
    if (!isTotalMode) return [];
    const pages: (number | "ellipsis")[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      pages.push(1);
      if (safePage > 3) {
        pages.push("ellipsis");
      }
      const startPage = Math.max(2, safePage - 1);
      const endPage = Math.min(totalPages - 1, safePage + 1);
      for (let i = startPage; i <= endPage; i++) {
        pages.push(i);
      }
      if (safePage < totalPages - 2) {
        pages.push("ellipsis");
      }
      pages.push(totalPages);
    }
    return pages;
  }, [isTotalMode, totalPages, safePage]);

  // 上一页处理
  const canGoPrev = isTotalMode
    ? !loading && safePage > 1
    : !loading && (hasPrev !== undefined ? hasPrev : page > 1);

  const handlePrev = () => {
    if (!canGoPrev) return;
    if (onPrevPage) {
      onPrevPage();
    } else if (onPageChange) {
      onPageChange(safePage - 1);
    }
  };

  // 下一页处理
  const canGoNext = isTotalMode
    ? !loading && safePage < totalPages
    : !loading && hasMore;

  const handleNext = () => {
    if (!canGoNext) return;
    if (onNextPage) {
      onNextPage();
    } else if (onPageChange) {
      onPageChange(safePage + 1);
    }
  };

  // 快速跳页处理
  const handleJump = (e: React.FormEvent) => {
    e.preventDefault();
    if (!onPageChange || !jumpInput.trim()) return;
    const target = parseInt(jumpInput.trim(), 10);
    if (!Number.isNaN(target) && target >= 1 && target <= totalPages) {
      onPageChange(target);
      setJumpInput("");
    }
  };

  return (
    <div
      className={`w-full flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-4 py-2.5 text-xs text-[var(--text-secondary)] border-t border-[var(--border-decorative)] bg-[var(--bg-surface)] shrink-0 select-none mt-auto transition-colors ${className}`}
      aria-label="Pagination"
    >
      {/* 左侧信息与每页条数 */}
      <div className="flex flex-wrap items-center gap-3">
        {isTotalMode ? (
          <div className="flex items-center gap-2 font-mono">
            <span>{t("admin.components.pagination_total", totalCount)}</span>
            <span className="text-[var(--border-control)]">/</span>
            <span>{t("admin.components.pagination_range", start, end, totalCount)}</span>
          </div>
        ) : (
          <div className="flex items-center gap-2 font-mono">
            <span className="font-semibold text-[var(--text-primary)]">
              {t("admin.audit.pagination_label", page) || `第 ${page} 页`}
            </span>
          </div>
        )}

        {onPageSizeChange && (
          <div className="flex items-center gap-1.5 pl-2.5 border-l border-[var(--border-decorative)]">
            <select
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              disabled={loading}
              aria-label="Select page size"
              className="h-7 px-2 text-xs rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] transition-colors cursor-pointer font-mono disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {pageSizeOptions.map((opt) => (
                <option key={opt} value={opt}>
                  {opt}
                </option>
              ))}
            </select>
            <span className="text-[var(--text-muted)]">
              {t("admin.components.per_page")}
            </span>
          </div>
        )}
      </div>

      {/* 右侧控制器与快速跳页 */}
      <div className="flex flex-wrap items-center gap-2 self-end sm:self-auto">
        {/* 上一页 */}
        <button
          type="button"
          onClick={handlePrev}
          disabled={!canGoPrev}
          className="inline-flex items-center gap-1 h-7 px-2.5 rounded-lg border border-[var(--border-decorative)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer font-medium"
          aria-label="Previous page"
        >
          <ChevronLeft className="w-3.5 h-3.5 shrink-0" />
          <span className="hidden sm:inline">
            {t("admin.audit.previous_page") || "上一页"}
          </span>
        </button>

        {/* 数字页码（仅在总数模式下展示） */}
        {isTotalMode &&
          pageNumbers.map((p, idx) => {
            if (p === "ellipsis") {
              return (
                <span
                  key={`ellipsis-${idx}`}
                  className="px-1 text-[var(--text-muted)] select-none"
                >
                  ...
                </span>
              );
            }
            const active = p === safePage;
            return (
              <button
                type="button"
                key={p}
                onClick={() => onPageChange && onPageChange(p)}
                disabled={loading}
                className={`min-w-[28px] h-7 px-2 text-xs font-mono font-medium rounded-lg transition-all cursor-pointer ${
                  active
                    ? "bg-[var(--action-primary)] text-[var(--action-on-primary)] font-semibold shadow-xs"
                    : "border border-[var(--border-decorative)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text-primary)]"
                }`}
              >
                {p}
              </button>
            );
          })}

        {/* 游标模式下的当前页 */}
        {!isTotalMode && (
          <span className="px-2.5 py-1 rounded-md bg-[var(--bg-subtle)] border border-[var(--border-decorative)] font-mono text-xs text-[var(--text-primary)] font-semibold">
            {page}
          </span>
        )}

        {/* 下一页 */}
        <button
          type="button"
          onClick={handleNext}
          disabled={!canGoNext}
          className="inline-flex items-center gap-1 h-7 px-2.5 rounded-lg border border-[var(--border-decorative)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer font-medium"
          aria-label="Next page"
        >
          <span className="hidden sm:inline">
            {t("admin.audit.next_page") || "下一页"}
          </span>
          <ChevronRight className="w-3.5 h-3.5 shrink-0" />
        </button>

        {/* 快速跳页（在总数模式且多于1页时展示） */}
        {isTotalMode && totalPages > 1 && onPageChange && (
          <form
            onSubmit={handleJump}
            className="flex items-center gap-1.5 pl-2 border-l border-[var(--border-decorative)] text-xs text-[var(--text-muted)]"
          >
            <span>{t("admin.components.pagination_jump_to")}</span>
            <input
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={jumpInput}
              onChange={(e) => setJumpInput(e.target.value.replace(/[^0-9]/g, ""))}
              placeholder={String(safePage)}
              className="w-10 h-7 px-1 text-center font-mono rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] transition-colors"
            />
            <span>{t("admin.components.pagination_page_unit")}</span>
          </form>
        )}
      </div>
    </div>
  );
}
