import React from "react";
import { cn } from "../utils/cn";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "outline" | "ghost" | "danger";
  size?: "sm" | "md" | "lg" | "icon";
  loading?: boolean;
  icon?: React.ReactNode;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "secondary", size = "md", loading, icon, children, disabled, ...props }, ref) => {
    const base = "inline-flex items-center justify-center font-medium transition-colors focus:outline-none disabled:opacity-50 disabled:pointer-events-none rounded-lg cursor-pointer";
    
    const variants = {
      primary: "bg-[var(--action-primary)] text-[var(--action-on-primary)] hover:bg-[var(--action-hover)] focus:ring-2 focus:ring-[var(--focus-ring)] shadow-sm font-semibold",
      secondary: "bg-[var(--bg-subtle)] text-[var(--text-primary)] hover:bg-[var(--border-decorative)]/70 border border-[var(--border-decorative)] focus:ring-2 focus:ring-[var(--focus-ring)]",
      outline: "border border-[var(--border-control)] bg-transparent text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] focus:ring-2 focus:ring-[var(--focus-ring)]",
      ghost: "bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text-primary)]",
      danger: "bg-[var(--status-danger)] text-white hover:opacity-90 focus:ring-2 focus:ring-[var(--status-danger)] shadow-sm font-semibold",
    };

    const sizes = {
      sm: "h-8 px-2.5 text-xs gap-1.5",
      md: "h-9 px-4 text-sm gap-2",
      lg: "h-11 px-6 text-base gap-2.5",
      icon: "h-9 w-9 p-0 flex items-center justify-center",
    };

    return (
      <button
        ref={ref}
        className={cn(base, variants[variant], sizes[size], className)}
        disabled={disabled || loading}
        {...props}
      >
        {loading ? (
          <span className="inline-block w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin mr-1" />
        ) : icon ? (
          <span className="flex items-center shrink-0">{icon}</span>
        ) : null}
        {children}
      </button>
    );
  }
);

Button.displayName = "Button";
