/**
 * @inklore/ui - 主题提供器 (ThemeProvider)
 * 注入语义 CSS 变量与 Tailwind 暗黑模式类名
 */

import React, { createContext, useContext, useState, useEffect, useMemo } from "react";
import { Audience } from "@inklore/contracts";
import { THEME_TOKENS, ThemeMode, ColorTokens } from "./tokens";

interface ThemeContextValue {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  audience: Audience;
  tokens: ColorTokens;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export interface ThemeProviderProps {
  children: React.ReactNode;
  audience: Audience;
  defaultMode?: ThemeMode;
}

export function ThemeProvider({ children, audience, defaultMode = "light" }: ThemeProviderProps) {
  const [mode, setModeState] = useState<ThemeMode>(() => {
    if (typeof localStorage !== "undefined") {
      const saved = localStorage.getItem(`inklore_${audience}_theme`);
      if (saved === "light" || saved === "dark") {
        return saved;
      }
    }
    return defaultMode;
  });

  const setMode = (newMode: ThemeMode) => {
    setModeState(newMode);
    if (typeof localStorage !== "undefined") {
      try {
        localStorage.setItem(`inklore_${audience}_theme`, newMode);
      } catch {}
    }
  };

  const tokens = THEME_TOKENS[audience][mode];

  // 注入 CSS 变量到 document.documentElement
  useEffect(() => {
    const root = document.documentElement;
    root.style.setProperty("--bg-canvas", tokens.bgCanvas);
    root.style.setProperty("--bg-surface", tokens.bgSurface);
    root.style.setProperty("--bg-paper", tokens.bgPaper);
    root.style.setProperty("--bg-subtle", tokens.bgSubtle);
    root.style.setProperty("--text-primary", tokens.textPrimary);
    root.style.setProperty("--text-secondary", tokens.textSecondary);
    root.style.setProperty("--text-muted", tokens.textMuted);
    root.style.setProperty("--border-decorative", tokens.borderDecorative);
    root.style.setProperty("--border-control", tokens.borderControl);
    root.style.setProperty("--action-primary", tokens.actionPrimary);
    root.style.setProperty("--action-on-primary", tokens.actionOnPrimary);
    root.style.setProperty("--action-hover", tokens.actionHover);
    root.style.setProperty("--focus-ring", tokens.focusRing);
    root.style.setProperty("--status-success", tokens.statusSuccess);
    root.style.setProperty("--status-warning", tokens.statusWarning);
    root.style.setProperty("--status-danger", tokens.statusDanger);
    root.style.setProperty("--status-info", tokens.statusInfo);
    root.style.setProperty("--diff-add-bg", tokens.diffAddBg);
    root.style.setProperty("--diff-remove-bg", tokens.diffRemoveBg);

    if (mode === "dark") {
      root.classList.add("dark");
      root.classList.add("dark-mode");
    } else {
      root.classList.remove("dark");
      root.classList.remove("dark-mode");
    }
  }, [mode, tokens]);

  const contextValue = useMemo(
    () => ({ mode, setMode, audience, tokens }),
    [mode, audience, tokens]
  );

  return (
    <ThemeContext.Provider value={contextValue}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return ctx;
}
