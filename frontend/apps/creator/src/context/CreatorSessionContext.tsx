/**
 * @inklore/creator - 创作者端身份会话上下文
 * 纯净排版与零 Emoji 铁律，100% 面向个人作者定位
 */

import React, { createContext, useContext, useState, useEffect, useCallback } from "react";
import { SessionSnapshot, UserPrincipal } from "@inklore/contracts";
import { getApiSDK, AuthSession, LoginCredentials, reportSafeDiagnostic } from "@inklore/transport";
import { AuthMode } from "@inklore/ui";
import { AUTH_VIEWS, authPath, navigateCreator, readCreatorLocation } from "../navigation";

export interface CreatorSessionContextValue {
  snapshot: SessionSnapshot;
  isAuthenticated: boolean;
  principal: UserPrincipal | null;
  login: (credentials: LoginCredentials) => Promise<UserPrincipal>;
  register: (data: { email: string; password: string; penName: string }) => Promise<void>;
  forgotPassword: (email: string) => Promise<void>;
  logout: () => Promise<void>;
  openAuthPage: (mode?: AuthMode) => void;
}

const CreatorSessionContext = createContext<CreatorSessionContextValue | null>(null);

// 个人作者端默认工作区租户（后台静默映射，绝不对作者展示）
export function CreatorSessionProvider({ children }: { children: React.ReactNode }) {
  const sdk = getApiSDK();
  const session = sdk.getSession() || new AuthSession({ audience: "creator" });

  const [snapshot, setSnapshot] = useState<SessionSnapshot>(() => session.getSnapshot());

  useEffect(() => {
    // 订阅 Session 变更
    const unsubscribe = session.subscribe((newSnapshot) => {
      setSnapshot(newSnapshot);
    });

    // 尝试从 Cookie 恢复或初始化会话
    session.bootstrap().catch((err) => {
      reportSafeDiagnostic("creator.bootstrap_failed", err);
    });

    return unsubscribe;
  }, [session]);

  const openAuthPage = useCallback((mode: AuthMode = "login") => {
    const route = readCreatorLocation();
    const returnTo = AUTH_VIEWS.includes(route.view)
      ? route.returnTo : window.location.pathname + window.location.search;
    navigateCreator(authPath(mode, returnTo || "/"));
  }, []);

  const login = useCallback(
    async (credentials: LoginCredentials) => session.login(credentials),
    [session]
  );

  const register = useCallback(
    async (data: { email: string; password: string; penName: string }) => {
      await session.register({
        email: data.email,
        password: data.password,
        pen_name: data.penName,
      });
    },
    [session]
  );

  const forgotPassword = useCallback(
    async (email: string) => {
      await session.forgotPassword(email);
    },
    [session]
  );

  const logout = useCallback(async () => {
    await session.logout();
  }, [session]);

  const value: CreatorSessionContextValue = {
    snapshot,
    isAuthenticated: session.isAuthenticated(),
    principal: session.getPrincipal(),
    login,
    register,
    forgotPassword,
    logout,
    openAuthPage,
  };

  return (
    <CreatorSessionContext.Provider value={value}>
      {children}
    </CreatorSessionContext.Provider>
  );
}

export function useCreatorSession(): CreatorSessionContextValue {
  const ctx = useContext(CreatorSessionContext);
  if (!ctx) {
    throw new Error("useCreatorSession must be used within a CreatorSessionProvider");
  }
  return ctx;
}
