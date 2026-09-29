/**
 * @inklore/admin-console - 管理端全屏鉴权登录页面
 * 基于 @inklore/ui 的通用 AuthView (variant="page")
 * 包含密码登录、验证码登录、找回密码、注册与图形验证码
 */

import React from "react";
import { AuthView, AuthMode } from "@inklore/ui";
import { useAdminSession } from "../context/AdminSessionContext";

export interface AdminLoginRouteProps {
  mode?: AuthMode;
  onNavigateMode?: (mode: AuthMode) => void;
  onSuccess?: () => void;
}

export function AdminLoginRoute({
  mode = "login",
  onNavigateMode,
  onSuccess,
}: AdminLoginRouteProps) {
  const { login } = useAdminSession();

  return (
    <AuthView
      variant="page"
      audience="admin"
      mode={mode}
      allowRegister={false}
      allowForgotPassword={false}
      allowOAuth={false}
      allowUsername={true}
      onNavigateMode={onNavigateMode}
      onLoginSubmit={async ({ email, password, tenant_id, captcha_id, captcha_code }) => {
        if (!password) return;
        const identifier = email.trim();
        await login({
          ...(identifier === "root" ? { username: "root" } : { email: identifier }),
          password,
          tenant_id,
          captcha_id,
          captcha_code,
        });
        onSuccess?.();
      }}
    />
  );
}
