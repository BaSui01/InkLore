import React from "react";
import { cn } from "../utils/cn";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: "default" | "secondary" | "success" | "warning" | "danger" | "info" | "outline";
  size?: "sm" | "md";
}

export function Badge({ className, variant = "default", size = "md", children, ...props }: BadgeProps) {
  const variants = {
    default: "bg-[var(--action-primary)]/15 text-[var(--action-primary)] border-[var(--action-primary)]/30",
    secondary: "bg-[var(--bg-subtle)] text-[var(--text-secondary)] border-[var(--border-decorative)]",
    success: "bg-[var(--status-success)]/15 text-[var(--status-success)] border-[var(--status-success)]/30",
    warning: "bg-[var(--status-warning)]/15 text-[var(--status-warning)] border-[var(--status-warning)]/30",
    danger: "bg-[var(--status-danger)]/15 text-[var(--status-danger)] border-[var(--status-danger)]/30",
    info: "bg-[var(--status-info)]/15 text-[var(--status-info)] border-[var(--status-info)]/30",
    outline: "bg-transparent text-[var(--text-secondary)] border-[var(--border-control)]",
  };

  const sizes = {
    sm: "text-xs px-2 py-0.5 font-medium leading-normal",
    md: "text-sm px-3 py-1 font-semibold leading-normal",
  };

  return (
    <span
      className={cn("inline-flex items-center gap-1 rounded-full border transition-colors", variants[variant], sizes[size], className)}
      {...props}
    >
      {children}
    </span>
  );
}