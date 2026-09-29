import React, { useEffect } from "react";
import { cn } from "../utils/cn";
import { AlertTriangle, AlertCircle, HelpCircle, X } from "lucide-react";
import { Button } from "./Button";

export interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
  title?: React.ReactNode;
  description?: React.ReactNode;
  confirmText?: string;
  cancelText?: string;
  variant?: "danger" | "warning" | "primary";
  loading?: boolean;
  confirmDisabled?: boolean;
  icon?: React.ReactNode;
  className?: string;
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title = "确认操作",
  description,
  confirmText = "确认执行",
  cancelText = "取消",
  variant = "danger",
  loading = false,
  confirmDisabled = false,
  icon,
  className,
}: ConfirmDialogProps) {
  // Support Escape key to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && open && !loading) {
        onClose();
      }
    };
    if (open) {
      window.addEventListener("keydown", handleKeyDown);
    }
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose, loading]);

  if (!open) return null;

  const getVariantStyles = () => {
    switch (variant) {
      case "danger":
        return {
          iconBg: "bg-[var(--status-danger)]/15 border-[var(--status-danger)]/30 text-[var(--status-danger)]",
          defaultIcon: <AlertTriangle className="w-6 h-6" />,
          buttonVariant: "danger" as const,
        };
      case "warning":
        return {
          iconBg: "bg-[var(--status-warning)]/15 border-[var(--status-warning)]/30 text-[var(--status-warning)]",
          defaultIcon: <AlertCircle className="w-6 h-6" />,
          buttonVariant: "primary" as const,
        };
      case "primary":
      default:
        return {
          iconBg: "bg-[var(--action-primary)]/15 border-[var(--action-primary)]/30 text-[var(--action-primary)]",
          defaultIcon: <HelpCircle className="w-6 h-6" />,
          buttonVariant: "primary" as const,
        };
    }
  };

  const currentStyles = getVariantStyles();

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 select-none">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
        onClick={() => {
          if (!loading) onClose();
        }}
      />

      {/* Centered Dialog Modal */}
      <div
        className={cn(
          "relative z-10 w-full max-w-md bg-[var(--bg-surface)] rounded-2xl border border-[var(--border-decorative)] shadow-2xl overflow-hidden p-6 text-[var(--text-primary)] transition-all",
          className
        )}
      >
        {/* Close Button Top Right */}
        <button
          onClick={onClose}
          disabled={loading}
          className="absolute top-4 right-4 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] rounded-lg p-1.5 transition-colors cursor-pointer disabled:opacity-40"
          title="关闭"
        >
          <X className="w-5 h-5" />
        </button>

        {/* Dialog Content */}
        <div className="flex flex-col items-center text-center space-y-4 pt-2 pb-2">
          {/* Top Icon Badge */}
          <div
            className={cn(
              "w-12 h-12 rounded-2xl border flex items-center justify-center shadow-sm shrink-0",
              currentStyles.iconBg
            )}
          >
            {icon || currentStyles.defaultIcon}
          </div>

          {/* Title & Description */}
          <div className="space-y-2">
            <h3 className="text-lg font-bold text-[var(--text-primary)] tracking-tight">
              {title}
            </h3>
            {description && (
              <div className="text-sm text-[var(--text-secondary)] leading-relaxed max-w-sm">
                {description}
              </div>
            )}
          </div>
        </div>

        {/* Bottom Actions */}
        <div className="flex items-center justify-center gap-3 pt-6 mt-2 border-t border-[var(--border-decorative)]">
          <Button
            variant="outline"
            size="md"
            disabled={loading}
            onClick={onClose}
            className="flex-1"
          >
            {cancelText}
          </Button>
          <Button
            variant={currentStyles.buttonVariant}
            size="md"
            loading={loading}
            disabled={confirmDisabled}
            onClick={onConfirm}
            className="flex-1"
          >
            {confirmText}
          </Button>
        </div>
      </div>
    </div>
  );
}
