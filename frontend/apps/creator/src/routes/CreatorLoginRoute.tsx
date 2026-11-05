/**
 * @inklore/creator - 创作者端全屏鉴权登录/注册/找回密码页面
 * 基于 @inklore/ui 的通用 AuthView (variant="page")
 * 严格遵循 No Emoji Policy 与双端物理隔离铁律
 */

import React from "react";
import { AuthView, AuthMode } from "@inklore/ui";
import { useCreatorSession } from "../context/CreatorSessionContext";

export interface CreatorLoginRouteProps {
  mode?: AuthMode;
  onNavigateMode?: (mode: AuthMode) => void;
  onSuccess?: () => void;
}

export function CreatorLoginRoute({
  mode = "login",
  onNavigateMode,
  onSuccess,
}: CreatorLoginRouteProps) {
  const { login, register, emailCapabilities, sendVerificationCode, getEmailCooldown, resetPassword } = useCreatorSession();

  return (
    <AuthView
      variant="page"
      audience="creator"
      mode={mode}
      allowRegister={true}
      allowForgotPassword={true}
      getEmailCapabilities={emailCapabilities}
      getEmailCooldown={getEmailCooldown}
      onSendVerificationCode={sendVerificationCode}
      onForgotPasswordSubmit={resetPassword}
      allowOAuth={false}
      allowUsername={true}
      onNavigateMode={onNavigateMode}
      onLoginSubmit={async (credentials) => {
        const identifier = credentials.email.trim();
        await login({
          ...(identifier === "root" ? { username: "root" } : { email: identifier }),
          password: credentials.password || "",
          ...(credentials.code ? { code: credentials.code } : {}),
          tenant_id: credentials.tenant_id,
          captcha_id: credentials.captcha_id,
          captcha_code: credentials.captcha_code,
        });
      }}
      onRegisterSubmit={async (data) => {
        await register({
          email: data.email,
          password: data.password,
          penName: data.penName,
          code: data.code, captcha_id: data.captcha_id, captcha_code: data.captcha_code,
        });
      }}
      onSuccess={(completedMode) => {
        if (completedMode !== "forgot-password") onSuccess?.();
      }}
    />
  );
}
