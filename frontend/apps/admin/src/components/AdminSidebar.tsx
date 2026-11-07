/**
 * @inklore/admin-console - 管理端左侧导航栏
 * 严格遵循 Dual-App Strict Isolation、No Emoji Policy 与零硬编码色彩主题
 */

import React, { useEffect, useState } from "react";
import {
  Activity,
  Mail,
  Layers,
  Cpu,
  Bot,
  ShieldCheck,
  Users,
  Terminal,
  Globe,
  LogIn,
  LogOut,
  Sun,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";
import { useTranslation } from "@inklore/i18n";
import { useTheme } from "@inklore/ui";
import { useAdminSession } from "../context/AdminSessionContext";

export type AdminView = "dashboard" | "channels" | "models" | "agents" | "audit" | "users" | "project-logs" | "mail-settings";

export interface AdminSidebarProps {
  currentView: AdminView;
  onSelectView: (view: AdminView) => void;
}

export function AdminSidebar({ currentView, onSelectView }: AdminSidebarProps) {
  const { t, locale, setLocale } = useTranslation();
  const { mode, setMode } = useTheme();
  const { isAuthenticated, snapshot, logout, setLoginModalOpen } = useAdminSession();

  const [collapsed, setCollapsed] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem("inklore_admin_sidebar_collapsed");
      if (stored !== null) return stored === "true";
      return window.innerWidth < 1024;
    } catch {
      return false;
    }
  });

  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth < 768) {
        setCollapsed(true);
      }
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem("inklore_admin_sidebar_collapsed", String(next));
      } catch {}
      return next;
    });
  };

  const navItems: { id: AdminView; labelKey: string; icon: React.ReactNode }[] = [
    { id: "dashboard", labelKey: "admin.sidebar.nav_dashboard", icon: <Activity className="w-4 h-4" /> },
    { id: "channels", labelKey: "admin.sidebar.nav_channels", icon: <Layers className="w-4 h-4" /> },
    { id: "models", labelKey: "admin.sidebar.nav_models", icon: <Cpu className="w-4 h-4" /> },
    { id: "users", labelKey: "admin.sidebar.nav_users", icon: <Users className="w-4 h-4" /> },
    { id: "agents", labelKey: "admin.sidebar.nav_agents", icon: <Bot className="w-4 h-4" /> },
    { id: "audit", labelKey: "admin.sidebar.nav_audit", icon: <ShieldCheck className="w-4 h-4" /> },
    { id: "project-logs", labelKey: "admin.sidebar.nav_project_logs", icon: <Terminal className="w-4 h-4" /> },
  ];

  if (snapshot.principal?.global_roles?.includes("super_admin")) {
    navItems.push({ id: "mail-settings", labelKey: "mail.title", icon: <Mail className="w-4 h-4" /> });
  }

  const toggleLanguage = () => {
    setLocale(locale.startsWith("zh") ? "en-US" : "zh-CN");
  };

  const toggleTheme = () => {
    setMode(mode === "dark" ? "light" : "dark");
  };

  return (
    <aside
      className={`bg-[var(--bg-surface)] border-r border-[var(--border-decorative)] flex flex-col shrink-0 select-none transition-all duration-200 ${
        collapsed ? "w-16" : "w-60"
      }`}
    >
      {/* Brand Header */}
      <div
        className={`h-16 flex items-center border-b border-[var(--border-decorative)] ${
          collapsed ? "justify-center px-2" : "px-4 gap-3 justify-between"
        }`}
      >
        <div className="flex items-center gap-3 min-w-0">
          <button
            type="button"
            className="w-8 h-8 rounded-xl bg-[var(--action-primary)] flex items-center justify-center text-[var(--action-on-primary)] shadow-sm shrink-0 cursor-pointer"
            onClick={toggleCollapsed}
            title={collapsed ? t("admin.sidebar.expand") : t("admin.sidebar.collapse")}
            aria-label={collapsed ? t("admin.sidebar.expand") : t("admin.sidebar.collapse")}
          >
            <Terminal className="w-4 h-4" />
          </button>
          {!collapsed && (
            <div className="min-w-0 flex-1">
              <div className="font-bold text-sm text-[var(--text-primary)] truncate tracking-tight">
                {t("admin.sidebar.brand")}
              </div>
              <div className="text-[10px] text-[var(--text-muted)] truncate font-mono">
                {t("admin.sidebar.brand_subtitle")}
              </div>
            </div>
          )}
        </div>
        {!collapsed && (
          <button
            type="button"
            onClick={toggleCollapsed}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer shrink-0"
            title={t("admin.sidebar.collapse")}
            aria-label={t("admin.sidebar.collapse")}
          >
            <PanelLeftClose className="w-4 h-4" />
          </button>
        )}
      </div>

      {/* Navigation Links */}
      <nav className={`flex-1 py-4 space-y-1 overflow-y-auto ${collapsed ? "px-2" : "px-2.5"}`}>
        {!collapsed && (
          <div className="text-[11px] font-semibold text-[var(--text-muted)] px-3 py-1 uppercase tracking-wider">
            {t("admin.sidebar.section_title")}
          </div>
        )}
        {navItems.map((item) => {
          const active = currentView === item.id;
          return (
            <button
              type="button"
              key={item.id}
              onClick={() => onSelectView(item.id)}
              title={collapsed ? t(item.labelKey) : undefined}
              className={`w-full flex items-center rounded-xl text-xs font-medium transition-all cursor-pointer ${
                collapsed ? "justify-center py-2.5 px-0" : "gap-3 px-3 py-2"
              } ${
                active
                  ? "bg-[var(--action-primary)]/15 text-[var(--action-primary)] border border-[var(--action-primary)]/30 font-semibold shadow-xs"
                  : "text-[var(--text-secondary)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text-primary)] border border-transparent"
              }`}
            >
              <span className={`shrink-0 transition-colors ${active ? "text-[var(--action-primary)]" : "text-[var(--text-muted)]"}`}>
                {item.icon}
              </span>
              {!collapsed && <span className="truncate">{t(item.labelKey)}</span>}
            </button>
          );
        })}
      </nav>

      {/* Bottom Simplified Controls */}
      <div className={`border-t border-[var(--border-decorative)] ${collapsed ? "p-2 space-y-2" : "p-3 space-y-2.5"}`}>
        {collapsed ? (
          <div className="flex flex-col items-center gap-1.5">
            <button
              type="button"
              onClick={toggleCollapsed}
              className="p-2 rounded-lg bg-[var(--bg-subtle)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-decorative)] transition-colors cursor-pointer w-full flex items-center justify-center"
              title={t("admin.sidebar.expand")}
              aria-label={t("admin.sidebar.expand")}
            >
              <PanelLeftOpen className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={toggleTheme}
              className="p-2 rounded-lg bg-[var(--bg-subtle)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-decorative)] transition-colors cursor-pointer w-full flex items-center justify-center"
              title={mode === "dark" ? t("auth.theme_light") : t("auth.theme_dark")}
              aria-label={mode === "dark" ? t("auth.theme_light") : t("auth.theme_dark")}
            >
              {mode === "dark" ? (
                <Moon className="w-3.5 h-3.5 text-[var(--action-primary)]" />
              ) : (
                <Sun className="w-3.5 h-3.5 text-[var(--status-warning)]" />
              )}
            </button>
            <button
              type="button"
              onClick={toggleLanguage}
              className="p-2 rounded-lg bg-[var(--bg-subtle)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-decorative)] transition-colors cursor-pointer w-full flex items-center justify-center font-mono text-[10px] font-semibold"
              title={t("admin.sidebar.lang_switch")}
              aria-label={t("admin.sidebar.lang_switch")}
            >
              {locale.startsWith("zh") ? "EN" : "中"}
            </button>
            {isAuthenticated ? (
              <button
                type="button"
                onClick={() => logout()}
                className="p-2 rounded-lg text-[var(--text-muted)] hover:text-[var(--status-danger)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer w-full flex items-center justify-center"
                title={t("admin.sidebar.logout_btn")}
                aria-label={t("admin.sidebar.logout_btn")}
              >
                <LogOut className="w-3.5 h-3.5" />
              </button>
            ) : (
              <button
                type="button"
                onClick={() => setLoginModalOpen(true)}
                className="p-2 rounded-lg text-[var(--action-primary)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer w-full flex items-center justify-center"
                title={t("admin.sidebar.login_btn")}
                aria-label={t("admin.sidebar.login_btn")}
              >
                <LogIn className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        ) : (
          <>
            {/* Quick toggles row: Theme & Language */}
            <div className="grid grid-cols-2 gap-1.5">
              <button
                type="button"
                onClick={toggleTheme}
                className="flex items-center justify-center gap-1.5 py-1.5 px-2 rounded-lg bg-[var(--bg-subtle)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-decorative)] text-[11px] font-medium transition-colors cursor-pointer"
                title={mode === "dark" ? t("auth.theme_light") : t("auth.theme_dark")}
                aria-label={mode === "dark" ? t("auth.theme_light") : t("auth.theme_dark")}
              >
                {mode === "dark" ? (
                  <Moon className="w-3 h-3 text-[var(--action-primary)]" />
                ) : (
                  <Sun className="w-3 h-3 text-[var(--status-warning)]" />
                )}
              </button>

              <button
                type="button"
                onClick={toggleLanguage}
                className="flex items-center justify-center gap-1.5 py-1.5 px-2 rounded-lg bg-[var(--bg-subtle)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] border border-[var(--border-decorative)] text-[11px] font-medium transition-colors cursor-pointer"
                title={t("admin.sidebar.lang_switch")}
                aria-label={t("admin.sidebar.lang_switch")}
              >
                <Globe className="w-3 h-3 text-[var(--text-muted)]" />
                <span className="font-mono">{locale.startsWith("zh") ? "EN" : "中"}</span>
              </button>
            </div>

            {/* User Session Pill */}
            <div className="px-3 py-2 rounded-xl bg-[var(--bg-canvas)] border border-[var(--border-decorative)] flex items-center justify-between">
              <div className="flex items-center gap-2 min-w-0">
                <span
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    isAuthenticated ? "bg-[var(--status-success)]" : "bg-[var(--status-warning)]"
                  }`}
                />
                <span className="text-[11px] text-[var(--text-primary)] truncate font-medium">
                  {isAuthenticated
                    ? snapshot.principal?.display_name || snapshot.principal?.user_id || t("admin.sidebar.session_authenticated")
                    : t("admin.sidebar.session_anonymous")}
                </span>
              </div>
              {isAuthenticated ? (
                <button
                  type="button"
                  onClick={() => logout()}
                  className="text-[var(--text-muted)] hover:text-[var(--status-danger)] transition-colors p-1 rounded-md hover:bg-[var(--bg-subtle)] cursor-pointer"
                  title={t("admin.sidebar.logout_btn")}
                  aria-label={t("admin.sidebar.logout_btn")}
                >
                  <LogOut className="w-3.5 h-3.5" />
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => setLoginModalOpen(true)}
                  className="text-[var(--action-primary)] hover:opacity-80 transition-colors flex items-center gap-1 text-[11px] font-semibold cursor-pointer"
                  title={t("admin.sidebar.login_btn")}
                  aria-label={t("admin.sidebar.login_btn")}
                >
                  <LogIn className="w-3.5 h-3.5" />
                  <span>{t("admin.sidebar.login_btn")}</span>
                </button>
              )}
            </div>
          </>
        )}
      </div>
    </aside>
  );
}
