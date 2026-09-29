import React from "react";
import { cn } from "../utils/cn";
import { isImeComposing, useImeComposition } from "../utils/ime";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  error?: string;
  prefixIcon?: React.ReactNode;
  /**
   * 当用户敲击 Enter 键且不在输入法组合（IME）状态时触发
   */
  onPressEnter?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  /**
   * 当用户敲击 Ctrl/Cmd + Enter 且不在输入法组合（IME）状态时触发
   */
  onCtrlEnter?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, error, prefixIcon, onPressEnter, onCtrlEnter, onKeyDown, onCompositionStart, onCompositionEnd, ...props }, ref) => {
    const { isComposingRef, justEndedRef, handleCompositionStart, handleCompositionEnd } = useImeComposition();

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
      const composing = isComposingRef.current || justEndedRef.current || isImeComposing(e);

      if (!composing) {
        if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && onCtrlEnter) {
          onCtrlEnter(e);
        } else if (e.key === "Enter" && onPressEnter) {
          onPressEnter(e);
        }
      }

      onKeyDown?.(e);
    };

    const handleCompStart = (e: React.CompositionEvent<HTMLInputElement>) => {
      handleCompositionStart(e);
      onCompositionStart?.(e);
    };

    const handleCompEnd = (e: React.CompositionEvent<HTMLInputElement>) => {
      handleCompositionEnd(e);
      onCompositionEnd?.(e);
    };

    return (
      <div className="w-full relative">
        {prefixIcon && (
          <div className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] pointer-events-none flex items-center">
            {prefixIcon}
          </div>
        )}
        <input
          ref={ref}
          className={cn(
            "w-full h-9 rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-paper)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] hover:border-[var(--border-control)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)]/25 focus:border-[var(--action-primary)] disabled:opacity-50 transition-colors",
            prefixIcon && "pl-9",
            error && "border-[var(--status-danger)] focus:ring-[var(--status-danger)]/25 focus:border-[var(--status-danger)]",
            className
          )}
          onKeyDown={handleKeyDown}
          onCompositionStart={handleCompStart}
          onCompositionEnd={handleCompEnd}
          {...props}
        />
        {error && <p className="text-xs text-[var(--status-danger)] mt-1">{error}</p>}
      </div>
    );
  }
);
Input.displayName = "Input";

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: string;
  /**
   * 当用户敲击 Enter 键且不在输入法组合（IME）状态时触发
   */
  onPressEnter?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  /**
   * 当用户敲击 Ctrl/Cmd + Enter 且不在输入法组合（IME）状态时触发
   */
  onCtrlEnter?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, error, onPressEnter, onCtrlEnter, onKeyDown, onCompositionStart, onCompositionEnd, ...props }, ref) => {
    const { isComposingRef, justEndedRef, handleCompositionStart, handleCompositionEnd } = useImeComposition();

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      const composing = isComposingRef.current || justEndedRef.current || isImeComposing(e);

      if (!composing) {
        if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && onCtrlEnter) {
          onCtrlEnter(e);
        } else if (e.key === "Enter" && onPressEnter) {
          onPressEnter(e);
        }
      }

      onKeyDown?.(e);
    };

    const handleCompStart = (e: React.CompositionEvent<HTMLTextAreaElement>) => {
      handleCompositionStart(e);
      onCompositionStart?.(e);
    };

    const handleCompEnd = (e: React.CompositionEvent<HTMLTextAreaElement>) => {
      handleCompositionEnd(e);
      onCompositionEnd?.(e);
    };

    return (
      <div className="w-full">
        <textarea
          ref={ref}
          className={cn(
            "w-full rounded-lg border border-[var(--border-decorative)] bg-[var(--bg-paper)] p-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] hover:border-[var(--border-control)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)] focus:border-transparent disabled:opacity-50 transition-colors resize-y",
            error && "border-[var(--status-danger)] focus:ring-[var(--status-danger)]",
            className
          )}
          onKeyDown={handleKeyDown}
          onCompositionStart={handleCompStart}
          onCompositionEnd={handleCompEnd}
          {...props}
        />
        {error && <p className="text-xs text-[var(--status-danger)] mt-1">{error}</p>}
      </div>
    );
  }
);
Textarea.displayName = "Textarea";

