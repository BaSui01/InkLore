import React from "react";
import { cn } from "../utils/cn";
import { X } from "lucide-react";

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  maxWidth?: "sm" | "md" | "lg" | "xl" | "2xl" | "4xl";
  className?: string;
}

export function Modal({
  open,
  onClose,
  title,
  children,
  footer,
  maxWidth = "md",
  className,
}: ModalProps) {
  if (!open) return null;

  const maxWidths = {
    sm: "max-w-sm",
    md: "max-w-md",
    lg: "max-w-lg",
    xl: "max-w-xl",
    "2xl": "max-w-2xl",
    "4xl": "max-w-4xl",
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
        onClick={onClose}
      />
      {/* Modal Dialog */}
      <div
        className={cn(
          "relative z-10 w-full bg-[var(--bg-surface)] rounded-2xl border border-[var(--border-decorative)] shadow-2xl overflow-hidden flex flex-col max-h-[90vh] text-[var(--text-primary)]",
          maxWidths[maxWidth],
          className
        )}
      >
        {title && (
          <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border-decorative)]">
            <div className="font-semibold text-[var(--text-primary)] text-lg">
              {title}
            </div>
            <button
              onClick={onClose}
              className="text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] rounded-lg p-1.5 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        )}
        <div className="px-6 py-4 overflow-y-auto flex-1 text-[var(--text-primary)]">
          {children}
        </div>
        {footer && (
          <div className="flex items-center justify-end gap-2 px-6 py-3 border-t border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
