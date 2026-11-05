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
  const { login, emailCapabilities, sendVerificationCode, getEmailCooldown, resetPassword } = useAdminSession();

  return (
    <AuthView
      variant="page"
      audience="admin"
      mode={mode}
      allowRegister={false}
      allowForgotPassword={true}
      getEmailCapabilities={emailCapabilities}
      getEmailCooldown={getEmailCooldown}
      onSendVerificationCode={sendVerificationCode}
      onForgotPasswordSubmit={resetPassword}
      allowOAuth={false}
      allowUsername={true}
      onNavigateMode={onNavigateMode}
      onLoginSubmit={async ({ email, password, code, tenant_id, captcha_id, captcha_code }) => {
        const identifier = email.trim();
        await login({
          ...(identifier === "root" ? { username: "root" } : { email: identifier }),
          password: password || "",
          ...(code ? { code } : {}),
          tenant_id,
          captcha_id,
          captcha_code,
        });
      }}
      onSuccess={(completed) => { if (completed !== "forgot-password") onSuccess?.(); }}
    />
  );
}
