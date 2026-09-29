/**
 * @inklore/admin-console - 管理端登录弹窗
 * 基于 @inklore/ui 的通用 AuthModal 实现
 * 纯净排版与零 Emoji 铁律，全面国际化
 * 严格受控：面向系统运维，禁止自主注册，仅保留标准密码认证
 */

import React from "react";
import { AuthModal } from "@inklore/ui";
import { useAdminSession } from "../context/AdminSessionContext";

export function AdminLoginModal() {
  const { isLoginModalOpen, setLoginModalOpen, login } = useAdminSession();

  if (!isLoginModalOpen) return null;

  return (
    <AuthModal
      open={isLoginModalOpen}
      onClose={() => setLoginModalOpen(false)}
      audience="admin"
      allowRegister={false}
      allowForgotPassword={false}
      allowOAuth={false}
      allowUsername={true}
      onLoginSubmit={async ({ email, password, tenant_id, captcha_id, captcha_code }) => {
        if (!password) return;
        const identifier = email.trim();
        await login({
          ...(identifier === "root" ? { username: "root" } : { email: identifier }),
          password: password.trim(),
          tenant_id,
          captcha_id,
          captcha_code,
        });
      }}
    />
  );
}
