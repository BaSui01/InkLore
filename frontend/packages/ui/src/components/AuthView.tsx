/**
 * @inklore/ui - 统一认证页面与组件 (AuthView)
 * 纯净排版与零 Emoji 铁律，全面国际化
 * 遵循设计系统语义 Token，彻底杜绝颜色与表单硬编码
 * 现代开源鉴权表单体系（参考 Supabase / Clerk / Shadcn UI）：
 * - 登录模式：支持「密码登录」与「验证码登录」双模平滑切换
 * - 注册模式：邮箱、笔名/昵称、验证码、密码、确认密码与条款
 * - 找回密码模式：邮箱、安全验证码（60s 倒计时）、新密码重置
 * - 全流程图形防刷验证码（前后端双验支持与交互式 Canvas 动态生成）
 * - 完整流转：登录、注册、找回密码、第三方登录（GitHub、Google、微信）无缝切换
 */

import React, { useState, useEffect, useCallback } from "react";
import { useTranslation } from "@inklore/i18n";
import { Input } from "./Input";
import { Button } from "./Button";
import { formatAuthError } from "./authError";
import { AuthPageLayout } from "./AuthPageLayout";
import { GithubIcon, GoogleIcon, WeChatIcon } from "./AuthSocialIcons";
import { GraphicCaptcha } from "./GraphicCaptcha";
import { LegalModal, LegalTab } from "./LegalModal";
import {
  Mail,
  Lock,
  User,
  Eye,
  EyeOff,
  Check,
  CheckCircle2,
  AlertCircle,
  ArrowLeft,
  Info,
  Terminal,
  Feather,
  KeyRound,
  ShieldCheck,
} from "lucide-react";

export type AuthMode = "login" | "register" | "forgot-password";
export type LoginMethod = "password" | "code";
export type OAuthProvider = "github" | "google" | "wechat";

export interface AuthViewProps {
  variant?: "page" | "card";
  audience?: "creator" | "admin";
  mode?: AuthMode;
  initialMode?: AuthMode;
  defaultEmail?: string;
  allowRegister?: boolean;
  allowForgotPassword?: boolean;
  allowOAuth?: boolean;
  allowUsername?: boolean;
  onNavigateMode?: (mode: AuthMode) => void;
  onLoginSubmit?: (credentials: {
    email: string;
    password?: string;
    code?: string;
    captcha?: string;
    captcha_id?: string;
    captcha_code?: string;
    rememberMe: boolean;
    tenant_id?: string;
  }) => Promise<any>;
  onRegisterSubmit?: (data: {
    email: string;
    password: string;
    penName: string;
    code?: string;
    captcha?: string;
    captcha_id?: string;
    captcha_code?: string;
  }) => Promise<any>;
  onForgotPasswordSubmit?: (data: {
    email: string;
    code: string;
    newPassword: string;
    captcha?: string;
    captcha_id?: string;
    captcha_code?: string;
  }) => Promise<any>;
  onSendVerificationCode?: (email: string, purpose: "login" | "register" | "reset") => Promise<any>;
  onOAuthClick?: (provider: OAuthProvider) => void;
  onSuccess?: (mode: AuthMode) => void;
  title?: React.ReactNode;
  subtitle?: string;
  className?: string;
}

export function AuthView({
  variant = "card",
  audience = "creator",
  mode: controlledMode,
  initialMode = "login",
  defaultEmail = "",
  allowRegister = true,
  allowForgotPassword = true,
  allowOAuth = true,
  allowUsername = false,
  onNavigateMode,
  onLoginSubmit,
  onRegisterSubmit,
  onForgotPasswordSubmit,
  onSendVerificationCode,
  onOAuthClick,
  onSuccess,
  title,
  subtitle,
  className,
}: AuthViewProps) {
  const { t } = useTranslation();

  // 模式与登录子方式控制
  const [internalMode, setInternalMode] = useState<AuthMode>(controlledMode || initialMode);
  const activeMode = controlledMode || internalMode;
  const [loginMethod, setLoginMethod] = useState<LoginMethod>("password");

  const changeMode = useCallback(
    (newMode: AuthMode) => {
      setInternalMode(newMode);
      setErrorMsg(null);
      setSuccessNotice(null);
      setOauthNotice(null);
      onNavigateMode?.(newMode);
    },
    [onNavigateMode]
  );

  useEffect(() => {
    if (controlledMode) {
      setInternalMode(controlledMode);
      setErrorMsg(null);
      setSuccessNotice(null);
      setOauthNotice(null);
    }
  }, [controlledMode]);

  // 表单状态
  const [email, setEmail] = useState(defaultEmail);
  const [penName, setPenName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmNewPassword, setConfirmNewPassword] = useState("");
  const [verifyCode, setVerifyCode] = useState("");
  const [captchaInput, setCaptchaInput] = useState("");
  const [expectedCaptcha, setExpectedCaptcha] = useState("");
  const [rememberMe, setRememberMe] = useState(true);
  const [agreeTerms, setAgreeTerms] = useState(false);
  const [captchaId, setCaptchaId] = useState("");
  const [captchaRefreshKey, setCaptchaRefreshKey] = useState(0);

  const handleCaptchaChange = useCallback((c: { id: string; code?: string }) => {
    setCaptchaId(c.id);
    setCaptchaInput("");
    setExpectedCaptcha(c.code || "");
  }, []);

  const handleCodeChange = useCallback((c: string) => {
    setExpectedCaptcha(c);
  }, []);
  const [isLegalModalOpen, setIsLegalModalOpen] = useState(false);
  const [legalTab, setLegalTab] = useState<LegalTab>("terms");

  // 监听 URL Hash 自动唤起服务条款与隐私政策
  useEffect(() => {
    const checkHash = () => {
      const hash = window.location.hash.toLowerCase();
      if (hash === "#terms" || hash === "#/terms") {
        setLegalTab("terms");
        setIsLegalModalOpen(true);
      } else if (hash === "#privacy" || hash === "#/privacy") {
        setLegalTab("privacy");
        setIsLegalModalOpen(true);
      }
    };
    checkHash();
    window.addEventListener("hashchange", checkHash);
    return () => window.removeEventListener("hashchange", checkHash);
  }, []);

  // 密码可见性
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmNewPassword, setShowConfirmNewPassword] = useState(false);

  // 加载与提示状态
  const [loading, setLoading] = useState(false);
  const [sendingCode, setSendingCode] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [successNotice, setSuccessNotice] = useState<string | null>(null);
  const [oauthNotice, setOauthNotice] = useState<string | null>(null);
  const [codeCooldown, setCodeCooldown] = useState(0);

  useEffect(() => {
    if (codeCooldown <= 0) return;
    const timer = setTimeout(() => {
      setCodeCooldown((prev) => prev - 1);
    }, 1000);
    return () => clearTimeout(timer);
  }, [codeCooldown]);

  const validateEmail = (val: string): boolean => {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(val.trim());
  };

  const validateLoginIdentifier = (val: string): boolean =>
    (allowUsername && val.trim() === "root") || validateEmail(val);

  const validateCaptcha = (): boolean => {
    const trimmed = captchaInput.trim();
    if (!trimmed) {
      setErrorMsg(t("auth.captcha_required"));
      return false;
    }
    if (trimmed.length !== 4) {
      setErrorMsg(t("auth.captcha_mismatch"));
      return false;
    }
    if (
      expectedCaptcha &&
      trimmed.toUpperCase() !== expectedCaptcha.trim().toUpperCase()
    ) {
      setErrorMsg(t("auth.captcha_mismatch"));
      return false;
    }
    return true;
  };

  // 发送邮箱验证码
  const handleSendCode = async (purpose: "login" | "register" | "reset") => {
    setErrorMsg(null);
    setSuccessNotice(null);

    const trimmedEmail = email.trim();
    if (!trimmedEmail || !validateLoginIdentifier(trimmedEmail)) {
      setErrorMsg(t("auth.email_required"));
      return;
    }

    setSendingCode(true);
    try {
      if (onSendVerificationCode) {
        await onSendVerificationCode(trimmedEmail, purpose);
      } else {
        await new Promise((r) => setTimeout(r, 600));
      }
      setSuccessNotice(t("auth.send_code_success"));
      setCodeCooldown(60);
    } catch (err: any) {
      setErrorMsg(formatAuthError(err, t));
    } finally {
      setSendingCode(false);
    }
  };

  // 登录提交
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);
    setSuccessNotice(null);
    setOauthNotice(null);

    const trimmedEmail = email.trim();
    if (!trimmedEmail || !validateLoginIdentifier(trimmedEmail)) {
      setErrorMsg(t(allowUsername ? "auth.login_identifier_required" : "auth.email_required"));
      return;
    }

    if (loginMethod === "password") {
      if (!password || Array.from(password).length < 6 || new TextEncoder().encode(password).length > 72) {
        setErrorMsg(t("auth.password_required"));
        return;
      }
    } else {
      if (!verifyCode.trim()) {
        setErrorMsg(t("auth.verify_code_required"));
        return;
      }
    }

    if (!validateCaptcha()) {
      return;
    }

    setLoading(true);
    try {
      if (onLoginSubmit) {
        await onLoginSubmit({
          email: trimmedEmail,
          password: loginMethod === "password" ? password : undefined,
          code: loginMethod === "code" ? verifyCode.trim() : undefined,
          captcha: captchaInput.trim(),
          captcha_id: captchaId || undefined,
          captcha_code: captchaInput.trim(),
          rememberMe,
        });
      }
      onSuccess?.("login");
    } catch (err: any) {
      setErrorMsg(formatAuthError(err, t));
      setCaptchaInput("");
      setCaptchaId("");
      setExpectedCaptcha("");
      setCaptchaRefreshKey((key) => key + 1);
    } finally {
      setLoading(false);
    }
  };

  // 注册提交
  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);
    setSuccessNotice(null);
    setOauthNotice(null);

    const trimmedEmail = email.trim();
    const trimmedPenName = penName.trim();

    if (!trimmedEmail || !validateEmail(trimmedEmail)) {
      setErrorMsg(t("auth.email_required"));
      return;
    }
    if (!trimmedPenName) {
      setErrorMsg(t("auth.pen_name_required"));
      return;
    }
    if (!verifyCode.trim()) {
      setErrorMsg(t("auth.verify_code_required"));
      return;
    }
    if (!password || Array.from(password).length < 6 || new TextEncoder().encode(password).length > 72) {
      setErrorMsg(t("auth.password_required"));
      return;
    }
    if (password !== confirmPassword) {
      setErrorMsg(t("auth.password_mismatch"));
      return;
    }
    if (!validateCaptcha()) {
      return;
    }
    if (!agreeTerms) {
      setErrorMsg(t("auth.terms_required"));
      return;
    }

    setLoading(true);
    try {
      if (onRegisterSubmit) {
        await onRegisterSubmit({
          email: trimmedEmail,
          password,
          penName: trimmedPenName,
          code: verifyCode.trim(),
          captcha: captchaInput.trim(),
          captcha_id: captchaId || undefined,
          captcha_code: captchaInput.trim(),
        });
      }
      onSuccess?.("register");
    } catch (err: any) {
      setErrorMsg(formatAuthError(err, t));
    } finally {
      setLoading(false);
    }
  };

  // 找回密码提交
  const handleForgotPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);
    setSuccessNotice(null);
    setOauthNotice(null);

    const trimmedEmail = email.trim();
    if (!trimmedEmail || !validateEmail(trimmedEmail)) {
      setErrorMsg(t("auth.email_required"));
      return;
    }
    if (!verifyCode.trim()) {
      setErrorMsg(t("auth.verify_code_required"));
      return;
    }
    if (!newPassword || Array.from(newPassword).length < 6 || new TextEncoder().encode(newPassword).length > 72) {
      setErrorMsg(t("auth.password_required"));
      return;
    }
    if (newPassword !== confirmNewPassword) {
      setErrorMsg(t("auth.password_mismatch"));
      return;
    }
    if (!validateCaptcha()) {
      return;
    }

    setLoading(true);
    try {
      if (onForgotPasswordSubmit) {
        await onForgotPasswordSubmit({
          email: trimmedEmail,
          code: verifyCode.trim(),
          newPassword,
          captcha: captchaInput.trim(),
          captcha_id: captchaId || undefined,
          captcha_code: captchaInput.trim(),
        });
      }
      setSuccessNotice(t("auth.reset_success"));
      setTimeout(() => {
        changeMode("login");
      }, 1500);
      onSuccess?.("forgot-password");
    } catch (err: any) {
      setErrorMsg(formatAuthError(err, t));
    } finally {
      setLoading(false);
    }
  };

  const handleOAuth = (provider: OAuthProvider) => {
    setErrorMsg(null);
    if (onOAuthClick) {
      onOAuthClick(provider);
      return;
    }
    const providerNames: Record<OAuthProvider, string> = {
      github: "GitHub",
      google: "Google",
      wechat: t("auth.oauth_wechat"),
    };
    setOauthNotice(t("auth.oauth_coming_soon", providerNames[provider]));
  };

  const isCreator = audience === "creator";

  // Shared heading for full-page and modal authentication views.
  const cardHeaderNode = title || (
    <div className="auth-card-header">
      <div className="auth-heading-mark">
        {!isCreator ? <Terminal className="w-5 h-5" /> : <Feather className="w-5 h-5" />}
      </div>
      <h3 className="text-2xl font-semibold text-[var(--text-primary)] leading-tight">
        {!isCreator
          ? t("auth.admin_login_title")
          : activeMode === "login"
          ? t("auth.login_title")
          : activeMode === "register"
          ? t("auth.register_title")
          : t("auth.forgot_title")}
      </h3>
      <p className="text-sm text-[var(--text-secondary)] mt-2 font-normal">
        {subtitle ||
          (!isCreator
            ? t("auth.admin_login_subtitle")
            : activeMode === "login"
            ? t("auth.login_subtitle")
            : activeMode === "register"
            ? t("auth.register_subtitle")
            : t("auth.forgot_subtitle"))}
      </p>
    </div>
  );

  // 表单主体内容
  const formContent = (
    <div className="space-y-4">
      {/* 1. 状态提示条（错误 / 成功 / 第三方渠道） */}
      {errorMsg && (
        <div className="flex items-start gap-2.5 p-3 rounded-xl bg-[var(--diff-remove-bg)] border border-[var(--status-danger)]/30 text-[var(--status-danger)] text-xs animate-in fade-in duration-200">
          <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
          <span className="flex-1 leading-relaxed font-medium">{errorMsg}</span>
        </div>
      )}

      {successNotice && (
        <div className="flex items-start gap-2.5 p-3 rounded-xl bg-[var(--diff-add-bg)] border border-[var(--status-success)]/30 text-[var(--status-success)] text-xs animate-in fade-in duration-200">
          <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" />
          <span className="flex-1 leading-relaxed font-medium">{successNotice}</span>
        </div>
      )}

      {oauthNotice && (
        <div className="flex items-start gap-2.5 p-3 rounded-xl bg-[var(--bg-subtle)] border border-[var(--status-warning)]/30 text-[var(--status-warning)] text-xs animate-in fade-in duration-200">
          <Info className="w-4 h-4 shrink-0 mt-0.5 text-[var(--status-warning)]" />
          <span className="flex-1 leading-relaxed font-medium">{oauthNotice}</span>
        </div>
      )}

      {/* 2. 登录模式 (Login Mode) */}
      {activeMode === "login" && (
        <form onSubmit={handleLogin} className="space-y-3.5">
          {/* 登录子方式切换（密码登录 vs 验证码登录） */}
          <div className="p-1 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center gap-1 mb-2">
            <button
              type="button"
              onClick={() => {
                setLoginMethod("password");
                setErrorMsg(null);
              }}
              className={`flex-1 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                loginMethod === "password"
                  ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs font-bold"
                  : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              }`}
            >
              {t("auth.tab_pwd_login")}
            </button>
            <button
              type="button"
              onClick={() => {
                setLoginMethod("code");
                setErrorMsg(null);
              }}
              className={`flex-1 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                loginMethod === "code"
                  ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs font-bold"
                  : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              }`}
            >
              {t("auth.tab_code_login")}
            </button>
          </div>

          {/* 邮箱输入 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {allowUsername ? t("auth.login_identifier") : t("auth.email")}
            </label>
            <div className="relative">
              <Input
                type={allowUsername ? "text" : "email"}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={allowUsername ? t("auth.login_identifier_ph") : t("auth.email_ph")}
                className="pl-9 text-sm h-10.5 rounded-xl"
                autoComplete={allowUsername ? "username" : "email"}
                autoFocus
                required
              />
              <Mail className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
            </div>
          </div>

          {/* 密码登录方式：密码输入 */}
          {loginMethod === "password" && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className="text-xs font-medium text-[var(--text-secondary)]">
                  {t("auth.password")}
                </label>
                {allowForgotPassword && (
                  <button
                    type="button"
                    onClick={() => changeMode("forgot-password")}
                    className="text-xs text-[var(--text-muted)] hover:text-[var(--action-primary)] transition-colors cursor-pointer"
                  >
                    {t("auth.forgot_password_link")}
                  </button>
                )}
              </div>
              <div className="relative">
                <Input
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("auth.password_ph")}
                  className="pl-9 pr-10 text-sm h-10.5 rounded-xl"
                  required
                />
                <Lock className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors p-1 rounded-md cursor-pointer"
                  title={showPassword ? t("auth.hide_password") : t("auth.show_password")}
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>
          )}

          {/* 验证码登录方式：邮箱验证码输入 */}
          {loginMethod === "code" && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className="text-xs font-medium text-[var(--text-secondary)]">
                  {t("auth.verify_code")}
                </label>
                {allowForgotPassword && (
                  <button
                    type="button"
                    onClick={() => changeMode("forgot-password")}
                    className="text-xs text-[var(--text-muted)] hover:text-[var(--action-primary)] transition-colors cursor-pointer"
                  >
                    {t("auth.forgot_password_link")}
                  </button>
                )}
              </div>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <Input
                    type="text"
                    value={verifyCode}
                    onChange={(e) => setVerifyCode(e.target.value)}
                    placeholder={t("auth.verify_code_ph")}
                    className="pl-9 text-sm h-10.5 rounded-xl font-mono"
                    maxLength={6}
                    required
                  />
                  <KeyRound className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                </div>
                <Button
                  type="button"
                  variant="outline"
                  className="h-10.5 px-3 text-xs font-semibold rounded-xl shrink-0 cursor-pointer"
                  disabled={codeCooldown > 0 || sendingCode}
                  onClick={() => handleSendCode("login")}
                >
                  {codeCooldown > 0
                    ? `${t("auth.resend_code_cooldown").replace("{0}", String(codeCooldown))}`
                    : sendingCode
                    ? t("auth.submitting")
                    : t("auth.send_code")}
                </Button>
              </div>
            </div>
          )}

          {/* 图形防刷验证码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.captcha")}
            </label>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Input
                  type="text"
                  value={captchaInput}
                  onChange={(e) => setCaptchaInput(e.target.value.toUpperCase())}
                  placeholder={t("auth.captcha_ph")}
                  className="pl-9 text-sm h-10.5 rounded-xl font-mono uppercase tracking-widest"
                  maxLength={4}
                  required
                />
                <ShieldCheck className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              </div>
              <GraphicCaptcha
                key={captchaRefreshKey}
                onCaptchaChange={handleCaptchaChange}
                onCodeChange={handleCodeChange}
                refreshTitle={t("auth.refresh_captcha")}
              />
            </div>
          </div>

          {/* 记住登录状态 */}
          <div className="flex items-center text-xs pt-0.5">
            <label className="inline-flex items-center gap-2 cursor-pointer select-none text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors group">
              <div className="relative flex items-center justify-center">
                <input
                  type="checkbox"
                  checked={rememberMe}
                  onChange={(e) => setRememberMe(e.target.checked)}
                  className="sr-only peer"
                />
                <div className={`w-4 h-4 rounded-md border transition-all flex items-center justify-center ${
                  rememberMe
                    ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-[var(--action-on-primary)] shadow-xs"
                    : "border-[var(--border-control)] bg-[var(--bg-paper)] group-hover:border-[var(--action-primary)]/70"
                } peer-focus-visible:ring-2 peer-focus-visible:ring-[var(--focus-ring)]/30`}>
                  {rememberMe && <Check className="w-3 h-3 stroke-[3]" />}
                </div>
              </div>
              <span className="font-medium">{t("auth.remember_me")}</span>
            </label>
          </div>

          <Button
            type="submit"
            variant="primary"
            className="w-full h-11 text-sm font-semibold rounded-xl cursor-pointer active:scale-[0.99] transition-all shadow-sm"
            loading={loading}
          >
            {loading ? t("auth.submitting") : t("auth.submit_login")}
          </Button>
        </form>
      )}

      {/* 3. 注册模式 (Register Mode) */}
      {activeMode === "register" && (
        <form onSubmit={handleRegister} className="space-y-3">
          {/* 邮箱 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.email")}
            </label>
            <div className="relative">
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={t("auth.email_ph")}
                className="pl-9 text-sm h-10 rounded-xl"
                autoFocus
                required
              />
              <Mail className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
            </div>
          </div>

          {/* 昵称/笔名 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.pen_name")}
            </label>
            <div className="relative">
              <Input
                type="text"
                value={penName}
                onChange={(e) => setPenName(e.target.value)}
                placeholder={t("auth.pen_name_ph")}
                className="pl-9 text-sm h-10 rounded-xl"
                required
              />
              <User className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
            </div>
          </div>

          {/* 邮箱安全验证码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.verify_code")}
            </label>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Input
                  type="text"
                  value={verifyCode}
                  onChange={(e) => setVerifyCode(e.target.value)}
                  placeholder={t("auth.verify_code_ph")}
                  className="pl-9 text-sm h-10 rounded-xl font-mono"
                  maxLength={6}
                  required
                />
                <KeyRound className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              </div>
              <Button
                type="button"
                variant="outline"
                className="h-10 px-3 text-xs font-semibold rounded-xl shrink-0 cursor-pointer"
                disabled={codeCooldown > 0 || sendingCode}
                onClick={() => handleSendCode("register")}
              >
                {codeCooldown > 0
                  ? `${t("auth.resend_code_cooldown").replace("{0}", String(codeCooldown))}`
                  : sendingCode
                  ? t("auth.submitting")
                  : t("auth.send_code")}
              </Button>
            </div>
          </div>

          {/* 密码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.password")}
            </label>
            <div className="relative">
              <Input
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("auth.password_ph")}
                className="pl-9 pr-10 text-sm h-10 rounded-xl"
                required
              />
              <Lock className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors p-1 rounded-md cursor-pointer"
                title={showPassword ? t("auth.hide_password") : t("auth.show_password")}
              >
                {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {/* 确认密码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.password_confirm")}
            </label>
            <div className="relative">
              <Input
                type={showConfirmPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder={t("auth.password_confirm_ph")}
                className="pl-9 pr-10 text-sm h-10 rounded-xl"
                required
              />
              <Lock className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              <button
                type="button"
                onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors p-1 rounded-md cursor-pointer"
                title={showConfirmPassword ? t("auth.hide_password") : t("auth.show_password")}
              >
                {showConfirmPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {/* 图形防刷验证码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.captcha")}
            </label>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Input
                  type="text"
                  value={captchaInput}
                  onChange={(e) => setCaptchaInput(e.target.value.toUpperCase())}
                  placeholder={t("auth.captcha_ph")}
                  className="pl-9 text-sm h-10 rounded-xl font-mono uppercase"
                  maxLength={4}
                  required
                />
                <ShieldCheck className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              </div>
              <GraphicCaptcha
                onCaptchaChange={handleCaptchaChange}
                onCodeChange={handleCodeChange}
                refreshTitle={t("auth.refresh_captcha")}
              />
            </div>
          </div>

          {/* 服务条款 */}
          <div className="text-xs pt-0.5">
            <label className="inline-flex items-start gap-2.5 cursor-pointer select-none text-[var(--text-secondary)] leading-relaxed group">
              <div className="relative flex items-center justify-center mt-0.5 shrink-0">
                <input
                  type="checkbox"
                  checked={agreeTerms}
                  onChange={(e) => setAgreeTerms(e.target.checked)}
                  className="sr-only peer"
                />
                <div className={`w-4 h-4 rounded-md border transition-all flex items-center justify-center ${
                  agreeTerms
                    ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-[var(--action-on-primary)] shadow-xs"
                    : "border-[var(--border-control)] bg-[var(--bg-paper)] group-hover:border-[var(--action-primary)]/70"
                } peer-focus-visible:ring-2 peer-focus-visible:ring-[var(--focus-ring)]/30`}>
                  {agreeTerms && <Check className="w-3 h-3 stroke-[3]" />}
                </div>
              </div>
              <span>
                {t("auth.agree_terms")}{" "}
                <button
                  type="button"
                  onClick={(e) => {
                    e.preventDefault();
                    setLegalTab("terms");
                    setIsLegalModalOpen(true);
                  }}
                  className="text-[var(--action-primary)] hover:text-[var(--action-hover)] hover:underline font-semibold cursor-pointer"
                >
                  {t("auth.terms_service")}
                </button>{" "}
                {t("auth.and")}{" "}
                <button
                  type="button"
                  onClick={(e) => {
                    e.preventDefault();
                    setLegalTab("privacy");
                    setIsLegalModalOpen(true);
                  }}
                  className="text-[var(--action-primary)] hover:text-[var(--action-hover)] hover:underline font-semibold cursor-pointer"
                >
                  {t("auth.privacy_policy")}
                </button>
              </span>
            </label>
          </div>

          <Button
            type="submit"
            variant="primary"
            className="w-full h-11 text-sm font-semibold rounded-xl cursor-pointer active:scale-[0.99] transition-all shadow-sm"
            loading={loading}
          >
            {loading ? t("auth.submitting") : t("auth.submit_register")}
          </Button>
        </form>
      )}

      {/* 4. 找回密码模式 (Forgot Password Mode) */}
      {activeMode === "forgot-password" && (
        <form onSubmit={handleForgotPassword} className="space-y-3.5">
          {/* 邮箱 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.email")}
            </label>
            <div className="relative">
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder={t("auth.email_ph")}
                className="pl-9 text-sm h-10.5 rounded-xl"
                autoFocus
                required
              />
              <Mail className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
            </div>
          </div>

          {/* 邮箱安全验证码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.verify_code")}
            </label>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Input
                  type="text"
                  value={verifyCode}
                  onChange={(e) => setVerifyCode(e.target.value)}
                  placeholder={t("auth.verify_code_ph")}
                  className="pl-9 text-sm h-10.5 rounded-xl font-mono"
                  maxLength={6}
                  required
                />
                <KeyRound className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              </div>
              <Button
                type="button"
                variant="outline"
                className="h-10.5 px-3 text-xs font-semibold rounded-xl shrink-0 cursor-pointer"
                disabled={codeCooldown > 0 || sendingCode}
                onClick={() => handleSendCode("reset")}
              >
                {codeCooldown > 0
                  ? `${t("auth.resend_code_cooldown").replace("{0}", String(codeCooldown))}`
                  : sendingCode
                  ? t("auth.submitting")
                  : t("auth.send_code")}
              </Button>
            </div>
          </div>

          {/* 设置新密码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.new_password")}
            </label>
            <div className="relative">
              <Input
                type={showNewPassword ? "text" : "password"}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder={t("auth.new_password_ph")}
                className="pl-9 pr-10 text-sm h-10.5 rounded-xl"
                required
              />
              <Lock className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              <button
                type="button"
                onClick={() => setShowNewPassword(!showNewPassword)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors p-1 rounded-md cursor-pointer"
                title={showNewPassword ? t("auth.hide_password") : t("auth.show_password")}
              >
                {showNewPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {/* 确认新密码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.new_password_confirm")}
            </label>
            <div className="relative">
              <Input
                type={showConfirmNewPassword ? "text" : "password"}
                value={confirmNewPassword}
                onChange={(e) => setConfirmNewPassword(e.target.value)}
                placeholder={t("auth.new_password_confirm_ph")}
                className="pl-9 pr-10 text-sm h-10.5 rounded-xl"
                required
              />
              <Lock className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              <button
                type="button"
                onClick={() => setShowConfirmNewPassword(!showConfirmNewPassword)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-subtle)] transition-colors p-1 rounded-md cursor-pointer"
                title={showConfirmNewPassword ? t("auth.hide_password") : t("auth.show_password")}
              >
                {showConfirmNewPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {/* 图形防刷验证码 */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
              {t("auth.captcha")}
            </label>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Input
                  type="text"
                  value={captchaInput}
                  onChange={(e) => setCaptchaInput(e.target.value.toUpperCase())}
                  placeholder={t("auth.captcha_ph")}
                  className="pl-9 text-sm h-10.5 rounded-xl font-mono uppercase tracking-widest"
                  maxLength={4}
                  required
                />
                <ShieldCheck className="w-4 h-4 text-[var(--text-muted)] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
              </div>
              <GraphicCaptcha
                onCaptchaChange={handleCaptchaChange}
                onCodeChange={handleCodeChange}
                refreshTitle={t("auth.refresh_captcha")}
              />
            </div>
          </div>

          <Button
            type="submit"
            variant="primary"
            className="w-full h-11 text-sm font-semibold rounded-xl cursor-pointer active:scale-[0.99] transition-all shadow-sm"
            loading={loading}
          >
            {loading ? t("auth.submitting") : t("auth.submit_reset")}
          </Button>

          <div className="text-center pt-1">
            <button
              type="button"
              onClick={() => changeMode("login")}
              className="inline-flex items-center gap-1.5 text-xs text-[var(--text-secondary)] hover:text-[var(--action-primary)] transition-colors cursor-pointer"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              <span>{t("auth.back_to_login")}</span>
            </button>
          </div>
        </form>
      )}

      {/* 5. 第三方快速登录入口（GitHub / Google / 微信） */}
      {allowOAuth && activeMode !== "forgot-password" && (
        <div className="pt-1.5 space-y-2.5">
          <div className="relative flex items-center justify-center">
            <div className="border-t border-[var(--border-decorative)] w-full" />
            <span className="bg-[var(--bg-surface)] px-2.5 text-[11px] text-[var(--text-muted)] shrink-0 absolute font-medium">
              {t("auth.or_oauth")}
            </span>
          </div>

          <div className="grid grid-cols-3 gap-2 pt-0.5">
            <button
              type="button"
              onClick={() => handleOAuth("github")}
              className="flex items-center justify-center gap-2 py-2 px-3 rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-surface)] hover:bg-[var(--bg-subtle)] text-[var(--text-primary)] text-xs font-medium transition-all shadow-2xs hover:border-[var(--border-control)] active:scale-[0.98] cursor-pointer"
              title={t("auth.oauth_github")}
            >
              <GithubIcon className="w-4 h-4" />
              <span>GitHub</span>
            </button>

            <button
              type="button"
              onClick={() => handleOAuth("google")}
              className="flex items-center justify-center gap-2 py-2 px-3 rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-surface)] hover:bg-[var(--bg-subtle)] text-[var(--text-primary)] text-xs font-medium transition-all shadow-2xs hover:border-[var(--border-control)] active:scale-[0.98] cursor-pointer"
              title={t("auth.oauth_google")}
            >
              <GoogleIcon className="w-4 h-4" />
              <span>Google</span>
            </button>

            <button
              type="button"
              onClick={() => handleOAuth("wechat")}
              className="flex items-center justify-center gap-2 py-2 px-3 rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-surface)] hover:bg-[var(--bg-subtle)] text-[var(--text-primary)] text-xs font-medium transition-all shadow-2xs hover:border-[var(--border-control)] active:scale-[0.98] cursor-pointer"
              title={t("auth.oauth_wechat")}
            >
              <WeChatIcon className="w-4 h-4" />
              <span>{t("auth.oauth_wechat")}</span>
            </button>
          </div>
        </div>
      )}

      {/* 6. 底部账号状态切换辅助链接 */}
      <div className="pt-2 text-center text-xs text-[var(--text-muted)]">
        {activeMode === "login" ? (
          allowRegister && (
            <span>
              {t("auth.no_account")}{" "}
              <button
                type="button"
                onClick={() => changeMode("register")}
                className="text-[var(--action-primary)] hover:text-[var(--action-hover)] hover:underline font-semibold cursor-pointer"
              >
                {t("auth.sign_up_now")}
              </button>
            </span>
          )
        ) : activeMode === "register" ? (
          <span>
            {t("auth.has_account")}{" "}
            <button
              type="button"
              onClick={() => changeMode("login")}
              className="text-[var(--action-primary)] hover:text-[var(--action-hover)] hover:underline font-semibold cursor-pointer"
            >
              {t("auth.sign_in_now")}
            </button>
          </span>
        ) : null}
      </div>
    </div>
  );

  return (
    <>
      {variant === "page" ? (
        <AuthPageLayout audience={audience} className={className}>
          {cardHeaderNode}
          {formContent}
        </AuthPageLayout>
      ) : (
        <div className={`space-y-4 pt-1 ${className || ""}`}>
          {cardHeaderNode}
          {formContent}
        </div>
      )}
      <LegalModal
        open={isLegalModalOpen}
        onClose={() => setIsLegalModalOpen(false)}
        defaultTab={legalTab}
        onAgree={() => setAgreeTerms(true)}
      />
    </>
  );
}
