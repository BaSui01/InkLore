import React from "react";
import { cn } from "../utils/cn";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "./Button";

export interface PaginationProps {
  currentPage: number;
  pageSize: number;
  totalItems: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (size: number) => void;
  pageSizeOptions?: number[];
  className?: string;
  disabled?: boolean;
}

export function Pagination({
  currentPage,
  pageSize,
  totalItems,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [6, 12, 24, 48],
  className,
  disabled = false,
}: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  const safePage = Math.min(Math.max(1, currentPage), totalPages);

  const startItem = totalItems === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const endItem = Math.min(safePage * pageSize, totalItems);

  // Generate visible page numbers
  const getPageNumbers = () => {
    const pages: (number | string)[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1);
      if (safePage > 3) pages.push("...");
      const start = Math.max(2, safePage - 1);
      const end = Math.min(totalPages - 1, safePage + 1);
      for (let i = start; i <= end; i++) pages.push(i);
      if (safePage < totalPages - 2) pages.push("...");
      pages.push(totalPages);
    }
    return pages;
  };

  return (
    <div
      className={cn(
        "flex flex-col min-[900px]:flex-row items-center justify-between gap-4 py-3 px-2 select-none text-sm text-[var(--text-secondary)]",
        className
      )}
    >
      {/* Summary Info */}
      <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-2 text-sm font-medium">
        <span>
          显示第 <strong className="text-[var(--text-primary)] font-semibold">{startItem} - {endItem}</strong> 项
        </span>
        <span className="text-[var(--border-control)]">·</span>
        <span>
          共 <strong className="text-[var(--text-primary)] font-semibold">{totalItems}</strong> 项
        </span>

        {/* Page Size Select */}
        {onPageSizeChange && (
          <div className="flex items-center gap-1.5 ml-2">
            <span>每页</span>
            <select
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              disabled={disabled}
              className="h-8 px-2.5 rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-paper)] text-sm text-[var(--text-primary)] font-medium focus:ring-2 focus:ring-[var(--focus-ring)] cursor-pointer"
            >
              {pageSizeOptions.map((opt) => (
                <option key={opt} value={opt}>
                  {opt} 条
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {/* Pagination Controls */}
      <div className="flex items-center gap-1.5">
        {/* Prev Page Button */}
        <button
          onClick={() => onPageChange(safePage - 1)}
          disabled={disabled || safePage <= 1}
          className="h-8 px-3 rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-paper)] text-sm font-medium text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] disabled:opacity-40 disabled:pointer-events-none transition-colors flex items-center gap-1 cursor-pointer"
        >
          <ChevronLeft className="w-4 h-4" />
          <span>上一页</span>
        </button>

        {/* Page Numbers */}
        <div className="flex items-center gap-1 mx-1">
          {getPageNumbers().map((p, idx) => {
            if (p === "...") {
              return (
                <span
                  key={`ellipsis-${idx}`}
                  className="w-8 h-8 flex items-center justify-center text-sm text-[var(--text-muted)]"
                >
                  ...
                </span>
              );
            }

            const isActive = p === safePage;
            return (
              <button
                key={p}
                onClick={() => onPageChange(Number(p))}
                disabled={disabled}
                className={cn(
                  "w-8 h-8 rounded-lg text-sm font-semibold transition-all cursor-pointer flex items-center justify-center",
                  isActive
                    ? "bg-[var(--action-primary)] text-[var(--action-on-primary)] shadow-sm"
                    : "border border-[var(--border-decorative)] bg-[var(--bg-paper)] text-[var(--text-primary)] hover:bg-[var(--bg-subtle)]"
                )}
              >
                {p}
              </button>
            );
          })}
        </div>

        {/* Next Page Button */}
        <button
          onClick={() => onPageChange(safePage + 1)}
          disabled={disabled || safePage >= totalPages}
          className="h-8 px-3 rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-paper)] text-sm font-medium text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] disabled:opacity-40 disabled:pointer-events-none transition-colors flex items-center gap-1 cursor-pointer"
        >
          <span>下一页</span>
          <ChevronRight className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}
