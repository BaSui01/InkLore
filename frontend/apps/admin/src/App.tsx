/**
 * @inklore/admin-console - 系统治理端主入口
 * 严格遵循 Dual-App Strict Isolation 与 Zero Hardcoding Contract
 * 全面接入纯净国际化体系与 HTML5 History API 路径路由 (无 # 号)
 */

import React, { useState, useEffect, useCallback } from "react";
import { ThemeProvider } from "@inklore/ui";
import { I18nProvider, useTranslation, LoadingSpinner } from "@inklore/i18n";
import { AdminSessionProvider, useAdminSession } from "./context/AdminSessionContext";
import { AdminSidebar } from "./components/AdminSidebar";
import { AdminLoginRoute } from "./routes/AdminLoginRoute";
import { LlmDashboardRoute } from "./routes/LlmDashboardRoute";
import { ChannelsRoute } from "./routes/ChannelsRoute";
import { ModelConfigRoute } from "./routes/ModelConfigRoute";
import { AgentGovernanceRoute } from "./routes/AgentGovernanceRoute";
import { AuditTrailRoute } from "./routes/AuditTrailRoute";
import { ProjectLogsRoute } from "./routes/ProjectLogsRoute";
import { MailSettingsRoute } from "./routes/MailSettingsRoute";
import { UserManagementRoute } from "./routes/UserManagementRoute";
import {
  AdminView,
  AdminAuthView,
  AnyAdminView,
  VALID_ADMIN_VIEWS,
  AUTH_ADMIN_VIEWS,
  readAdminLocation,
  navigateAdmin,
  subscribeAdminLocation,
  normalizeAdminLocation,
} from "./navigation";

function AdminRouter() {
  const { t } = useTranslation();
  const { isAuthenticated, snapshot } = useAdminSession();
  const [currentView, setCurrentView] = useState<AnyAdminView>(() => readAdminLocation().view);
  const [returnToView, setReturnToView] = useState<AdminView | undefined>(() => readAdminLocation().returnTo);

  // 监听浏览器 URL 变化 (PopState 与导航事件)
  useEffect(() => {
    const handleLocationChange = () => {
      const loc = readAdminLocation();
      setCurrentView(loc.view);
      if (loc.returnTo) {
        setReturnToView(loc.returnTo);
      } else if (!AUTH_ADMIN_VIEWS.includes(loc.view as AdminAuthView)) {
        setReturnToView(undefined);
      }
    };

    return subscribeAdminLocation(handleLocationChange);
  }, []);

  // 路由跳转辅助
  const navigateTo = useCallback((view: AnyAdminView, returnTo?: AdminView, replace = false) => {
    navigateAdmin(view, returnTo, replace);
  }, []);

  // 核心鉴权守卫：当未登录且尝试访问受保护页面时，强重定向至登录页并保留 returnTo 目标
  useEffect(() => {
    if (snapshot.status === "bootstrapping") return;

    if (!isAuthenticated) {
      if (!AUTH_ADMIN_VIEWS.includes(currentView as AdminAuthView)) {
        const targetToReturn = VALID_ADMIN_VIEWS.includes(currentView as AdminView) ? (currentView as AdminView) : undefined;
        navigateTo("login", targetToReturn, true);
      }
    } else {
      // 已登录状态下如果还在 auth 路由，自动跳转到 returnToView 或默认 dashboard
      if (AUTH_ADMIN_VIEWS.includes(currentView as AdminAuthView)) {
        const nextView = returnToView || "dashboard";
        navigateTo(nextView, undefined, true);
      }
    }
  }, [isAuthenticated, snapshot.status, currentView, returnToView, navigateTo]);

  // 会话初始化引导中：展示居中优雅加载指示器，避免白屏或未鉴权闪烁
  if (snapshot.status === "bootstrapping") {
    return (
      <div className="h-screen w-screen flex flex-col items-center justify-center bg-[var(--bg-canvas)] text-[var(--text-secondary)]">
        <LoadingSpinner size={36} />
        <p className="mt-4 text-xs font-mono tracking-wider">{t("common.loading")}</p>
      </div>
    );
  }

  // 未登录或处于鉴权路由：直接渲染全屏鉴权登录/注册/找回密码页面
  if (!isAuthenticated || AUTH_ADMIN_VIEWS.includes(currentView as AdminAuthView)) {
    const authMode = (AUTH_ADMIN_VIEWS.includes(currentView as AdminAuthView) ? currentView : "login") as AdminAuthView;
    return (
      <AdminLoginRoute
        mode={authMode}
        onNavigateMode={(mode) => navigateTo(mode === "forgot-password" ? mode : "login", returnToView)}
        onSuccess={() => {
          const nextView = returnToView || "dashboard";
          navigateTo(nextView, undefined, true);
        }}
      />
    );
  }

  // 已登录受控治理大盘：渲染左侧导航侧栏与对应业务路由
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[var(--bg-canvas)] text-[var(--text-primary)] select-none transition-colors">
      {/* Left Admin Navigation Sidebar */}
      <AdminSidebar
        currentView={currentView as AdminView}
        onSelectView={(v) => navigateTo(v)}
      />

      {/* Main Governance View */}
      <div key={snapshot.principal?.tenant_id || "anonymous"} className="flex-1 flex flex-col min-w-0 h-full overflow-hidden bg-[var(--bg-canvas)]">
        {currentView === "dashboard" && <LlmDashboardRoute />}
        {currentView === "channels" && <ChannelsRoute />}
        {currentView === "models" && <ModelConfigRoute />}
        {currentView === "users" && <UserManagementRoute />}
        {currentView === "agents" && <AgentGovernanceRoute />}
        {currentView === "audit" && <AuditTrailRoute />}
        {currentView === "project-logs" && <ProjectLogsRoute />}
        {currentView === "mail-settings" && <MailSettingsRoute />}
      </div>
    </div>
  );
}

export function AdminApp() {
  // 初次装载时执行自动迁移/规范化（将带有 #/ 的链接无缝转换为普通路径）
  useEffect(() => {
    normalizeAdminLocation();
  }, []);

  return (
    <I18nProvider defaultLocale="zh-CN">
      <AdminSessionProvider>
        <ThemeProvider audience="admin" defaultMode="light">
          <AdminRouter />
        </ThemeProvider>
      </AdminSessionProvider>
    </I18nProvider>
  );
}
