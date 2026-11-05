/**
 * @inklore/admin-console - 管理端会话上下文
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.1 & §3.2
 * 仅保留标准密码认证，彻底移除 Token 直接认证入口
 */

import React, { createContext, useContext, useState, useEffect, useCallback } from "react";
import { SessionSnapshot, UserPrincipal } from "@inklore/contracts";
import { adminSession } from "../api/adminApi";
import { LoginCredentials, EmailCapabilities, EmailChallenge, EmailPurpose, EmailResetData, reportSafeDiagnostic } from "@inklore/transport";

export interface AdminSessionContextValue {
  snapshot: SessionSnapshot;
  isAuthenticated: boolean;
  login: (credentials: LoginCredentials) => Promise<UserPrincipal>;
  emailCapabilities: () => Promise<EmailCapabilities>;
  sendVerificationCode: (email: string, purpose: EmailPurpose) => Promise<EmailChallenge>;
  getEmailCooldown: (email: string) => number;
  resetPassword: (data: EmailResetData) => Promise<void>;
  switchTenant: (tenantId: string) => Promise<UserPrincipal>;
  logout: () => Promise<void>;
  isLoginModalOpen: boolean;
  setLoginModalOpen: (open: boolean) => void;
}

const AdminSessionContext = createContext<AdminSessionContextValue | null>(null);

// Session 仅从既有 HttpOnly Cookie 恢复；失败留在登录页由操作员显式认证。
let isBootstrapping = false;

export function AdminSessionProvider({ children }: { children: React.ReactNode }) {
  const [snapshot, setSnapshot] = useState<SessionSnapshot>(() => adminSession.getSnapshot());
  const [isLoginModalOpen, setLoginModalOpen] = useState(false);

  useEffect(() => {
    // 订阅 Session 状态变更
    const unsubscribe = adminSession.subscribe((newSnapshot) => {
      setSnapshot(newSnapshot);
    });

    if (isBootstrapping) return unsubscribe;
    isBootstrapping = true;

    // 页面初次加载时尝试引导初始化（从 HttpOnly Cookie 恢复会话）
    adminSession.bootstrap().catch((err) => {
      reportSafeDiagnostic("admin.bootstrap_failed", err);
    });

    return unsubscribe;
  }, []);

  const login = useCallback(async (credentials: LoginCredentials) => {
    const principal = await adminSession.login(credentials);
    setLoginModalOpen(false);
    return principal;
  }, []);

  const emailCapabilities = useCallback(() => adminSession.emailCapabilities(), []);
  const sendVerificationCode = useCallback((email: string, purpose: EmailPurpose) => adminSession.sendVerificationCode(email, purpose), []);
  const getEmailCooldown = useCallback((email: string) => adminSession.getEmailCooldown(email), []);
  const resetPassword = useCallback((data: EmailResetData) => adminSession.resetPassword(data), []);

  const switchTenant = useCallback((tenantId: string) => adminSession.switchTenant(tenantId), []);

  const logout = useCallback(async () => {
    await adminSession.logout();
  }, []);

  const value: AdminSessionContextValue = {
    snapshot,
    isAuthenticated: adminSession.isAuthenticated(),
    login,
    switchTenant, emailCapabilities, sendVerificationCode, getEmailCooldown, resetPassword,
    logout,
    isLoginModalOpen,
    setLoginModalOpen,
  };

  return (
    <AdminSessionContext.Provider value={value}>
      {children}
    </AdminSessionContext.Provider>
  );
}

export function useAdminSession(): AdminSessionContextValue {
  const ctx = useContext(AdminSessionContext);
  if (!ctx) {
    throw new Error("useAdminSession must be used within an AdminSessionProvider");
  }
  return ctx;
}
