/**
 * @inklore/transport - 身份会话管理 (AuthSession)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.1 & §3.2
 * 
 * 核心铁律：
 * 1. access_token 与 csrf_token 严格仅在内存持有，绝不持久化到 localStorage/sessionStorage/IndexedDB。
 * 2. 刷新页面时通过 GET /v1/auth/session-csrf 引导获取 nonce，再 POST /v1/auth/refresh。
 * 3. 401 触发单飞 (Single-Flight) 刷新：同页面并发 401 仅触发一次网络刷新，其余请求挂起等待。
 * 4. 登出先清空内存凭据并递增 sessionEpoch，防止已失效主体缓存闪现或串门。
 */

import {
  Audience,
  SessionSnapshot,
  SessionStatus,
  UserPrincipal,
  ScopeKey,
  SuccessEnvelope,
  ApiError,
} from "@inklore/contracts";

import { reportSafeDiagnostic } from "./diagnostic";
import { stripTrailingSlashes } from "../base-url";

export interface AuthSessionOptions {
  audience: Audience;
  baseUrl?: string;
  onSessionInvalid?: () => void;
}

export type LoginCredentials = ({
  email: string;
} | {
  username: string;
}) & {
  password: string;
  tenant_id?: string;
  captcha_id?: string;
  captcha_code?: string;
};

export interface RegisterData {
  email: string;
  password: string;
  pen_name: string;
}


export type SessionListener = (snapshot: SessionSnapshot) => void;

export class AuthSession {
  private readonly audience: Audience;
  private readonly baseUrl: string;
  private readonly onSessionInvalidCallback?: () => void;

  private accessToken: string | null = null;
  private csrfToken: string | null = null;
  private principal: UserPrincipal | null = null;
  private accessExpiresAt: number | null = null;
  private status: SessionStatus = "bootstrapping";
  private sessionEpoch = 1;

  private refreshPromise: Promise<string> | null = null;
  private bootstrapPromise: Promise<SessionSnapshot> | null = null;
  private readonly listeners = new Set<SessionListener>();

  constructor(options: AuthSessionOptions) {
    this.audience = options.audience;
    this.baseUrl = stripTrailingSlashes(options.baseUrl || "");
    this.onSessionInvalidCallback = options.onSessionInvalid;
  }

  /** 获取当前端别 (creator 或 admin) */
  getAudience(): Audience {
    return this.audience;
  }

  /** 获取当前内存访问令牌 */
  getAccessToken(): string | null {
    return this.accessToken;
  }

  /** 获取当前 CSRF 令牌 */
  getCsrfToken(): string | null {
    return this.csrfToken;
  }

  /** 获取当前主体信息 */
  getPrincipal(): UserPrincipal | null {
    return this.principal ? { ...this.principal } : null;
  }

  /** 获取当前作用域键 */
  getScope(novelId?: string): ScopeKey | null {
    if (!this.principal) return null;
    return {
      tenantId: this.principal.tenant_id,
      userId: this.principal.user_id,
      audience: this.audience,
      sessionEpoch: this.sessionEpoch,
      permissionRevision: this.principal.membership_revision,
      novelId,
    };
  }

  /** 获取当前主体会话纪元代数（每次登录/登出/重置时单调自增） */
  getSessionEpoch(): number {
    return this.sessionEpoch;
  }

  /** 获取当前会话状态 */
  getStatus(): SessionStatus {
    return this.status;
  }

  /** 是否已完成认证 */
  isAuthenticated(): boolean {
    return this.status === "authenticated" && Boolean(this.accessToken);
  }

  /** 获取只读会话快照 */
  getSnapshot(): SessionSnapshot {
    return {
      status: this.status,
      principal: this.getPrincipal(),
      scope: this.getScope(),
      accessExpiresAt: this.accessExpiresAt,
      sessionEpoch: this.sessionEpoch,
    };
  }

  /** 订阅会话状态变更 */
  subscribe(listener: SessionListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private notify(): void {
    const snapshot = this.getSnapshot();
    for (const listener of this.listeners) {
      try {
        listener(snapshot);
      } catch {
        reportSafeDiagnostic("auth.listener_failed");
      }
    }
  }

  /**
   * 引导初始化会话 (Session Bootstrap)
   * 页面首次加载或刷新时调用：
   * 1. GET /v1/auth/session-csrf?audience={audience} (依赖 HttpOnly Cookie)
   * 2. POST /v1/auth/refresh (携 X-CSRF-Token 与 credentials: same-origin)
   */
  bootstrap(): Promise<SessionSnapshot> {
    if (this.bootstrapPromise) return this.bootstrapPromise;
    this.bootstrapPromise = this.executeBootstrap().finally(() => {
      this.bootstrapPromise = null;
    });
    return this.bootstrapPromise;
  }

  private async executeBootstrap(): Promise<SessionSnapshot> {
    this.status = "bootstrapping";
    this.notify();

    try {
      // 步骤 1：同源请求获取 CSRF nonce
      const csrfUrl = `${this.baseUrl}/v1/auth/session-csrf?audience=${encodeURIComponent(this.audience)}`;
      const csrfRes = await fetch(csrfUrl, {
        method: "GET",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
          "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
        },
      });

      if (!csrfRes.ok) {
        this.clearCredentials("anonymous");
        return this.getSnapshot();
      }

      const csrfBody: SuccessEnvelope<{ csrf_token: string }> = await csrfRes.json();
      if (!csrfBody.data?.csrf_token) {
        this.clearCredentials("anonymous");
        return this.getSnapshot();
      }
      this.csrfToken = csrfBody.data.csrf_token;

      // 步骤 2：用 Cookie 与 CSRF 令牌刷新会话
      await this.refresh();
      return this.getSnapshot();
    } catch {
      this.clearCredentials("anonymous");
      return this.getSnapshot();
    }
  }

  /**
   * 用户登录
   */
  async login(credentials: LoginCredentials): Promise<UserPrincipal> {
    const loginUrl = `${this.baseUrl}/v1/auth/login`;
    const requestBody: Record<string, string> = {
      password: credentials.password,
      audience: this.audience,
    };
    if ("email" in credentials) requestBody.email = credentials.email;
    else requestBody.username = credentials.username;
    if (credentials.tenant_id) requestBody.tenant_id = credentials.tenant_id;
    if (credentials.captcha_id && credentials.captcha_code) {
      requestBody.captcha_id = credentials.captcha_id;
      requestBody.captcha_code = credentials.captcha_code;
    }
    const res = await fetch(loginUrl, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      },
      body: JSON.stringify(requestBody),
    });

    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new ApiError({
        httpStatus: res.status,
        code: body.code || "auth.invalid_credentials",
        message_key: body.message_key,
        details: body.details,
        request_id: body.request_id,
        trace_id: body.trace_id,
      });
    }

    const sessionData = body.data;
    this.sessionEpoch += 1; // 切换主体，纪元升级
    this.accessToken = sessionData.access_token;
    this.csrfToken = sessionData.csrf_token;
    this.principal = sessionData.principal;
    this.accessExpiresAt = Date.now() + (sessionData.expires_in || 900) * 1000;
    this.status = "authenticated";

    this.notify();
    return this.principal!;
  }

  async switchTenant(tenantId: string): Promise<UserPrincipal> {
    if (this.audience !== "admin" || !this.accessToken || !this.csrfToken) {
      throw new Error("An active Admin session is required to switch tenant");
    }
    const res = await fetch(`${this.baseUrl}/v1/auth/switch-tenant`, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        Authorization: `Bearer ${this.accessToken}`,
        "X-CSRF-Token": this.csrfToken,
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      },
      body: JSON.stringify({ tenant_id: tenantId }),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new ApiError({
        httpStatus: res.status,
        code: body.code || "auth.forbidden",
        message_key: body.message_key,
        details: body.details,
        request_id: body.request_id,
        trace_id: body.trace_id,
      });
    }
    const sessionData = body.data;
    if (!sessionData?.access_token || !sessionData?.csrf_token || !sessionData?.principal) {
      throw new Error("Invalid tenant-switch response");
    }
    this.sessionEpoch += 1;
    this.accessToken = sessionData.access_token;
    this.csrfToken = sessionData.csrf_token;
    this.principal = sessionData.principal;
    this.accessExpiresAt = Date.now() + (sessionData.expires_in || 900) * 1000;
    this.status = "authenticated";
    this.notify();
    return this.principal!;
  }

  /**
   * 创作者自主注册 (Creator Register)
   */
  async register(data: RegisterData): Promise<UserPrincipal> {
    const registerUrl = `${this.baseUrl}/v1/auth/register`;
    const res = await fetch(registerUrl, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      },
      body: JSON.stringify({
        email: data.email,
        password: data.password,
        pen_name: data.pen_name,
      }),
    });

    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new ApiError({
        httpStatus: res.status,
        code: body.code || "auth.account_unavailable",
        message_key: body.message_key,
        details: body.details,
        request_id: body.request_id,
        trace_id: body.trace_id,
      });
    }

    const sessionData = body.data;
    this.sessionEpoch += 1;
    this.accessToken = sessionData.access_token;
    this.csrfToken = sessionData.csrf_token;
    this.principal = sessionData.principal;
    this.accessExpiresAt = Date.now() + (sessionData.expires_in || 900) * 1000;
    this.status = "authenticated";

    this.notify();
    return this.principal!;
  }

  /**
   * 忘记密码申请 (Forgot Password Request)
   */
  async forgotPassword(email: string): Promise<{ sent: boolean; dev_token?: string }> {
    const url = `${this.baseUrl}/v1/auth/forgot-password`;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      },
      body: JSON.stringify({ email }),
    });

    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new ApiError({
        httpStatus: res.status,
        code: body.code || "platform.validation_failed",
        message_key: body.message_key,
        details: body.details,
        request_id: body.request_id,
        trace_id: body.trace_id,
      });
    }

    return body.data || { sent: false };
  }

  /**
   * 凭重置令牌更新密码 (Reset Password with Token)
   */
  async resetPassword(token: string, newPassword: string): Promise<void> {
    const url = `${this.baseUrl}/v1/auth/reset-password`;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      },
      body: JSON.stringify({ token, new_password: newPassword }),
    });

    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new ApiError({
        httpStatus: res.status,
        code: body.code || "auth.invalid_token",
        message_key: body.message_key,
        details: body.details,
        request_id: body.request_id,
        trace_id: body.trace_id,
      });
    }
  }

  /**
   * 单飞刷新 (Single-Flight Refresh)
   * 并发 401 请求同时到达时，共享唯一的刷新 Promise。
   */
  async refresh(): Promise<string> {
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = this.executeRefresh()
      .then((token) => token)
      .finally(() => {
        this.refreshPromise = null;
      });

    return this.refreshPromise;
  }

  private async executeRefresh(): Promise<string> {
    try {
      const refreshUrl = `${this.baseUrl}/v1/auth/refresh`;
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      };

      if (this.csrfToken) {
        headers["X-CSRF-Token"] = this.csrfToken;
      }

      const res = await fetch(refreshUrl, {
        method: "POST",
        credentials: "same-origin",
        headers,
        body: JSON.stringify({ audience: this.audience }),
      });

      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        this.clearCredentials("expired");
        if (this.onSessionInvalidCallback) {
          this.onSessionInvalidCallback();
        }
        throw new ApiError({
          httpStatus: res.status,
          code: body.code || "auth.unauthenticated",
          message_key: body.message_key,
          details: body.details,
          request_id: body.request_id,
          trace_id: body.trace_id,
        });
      }

      const data = body.data;
      this.accessToken = data.access_token;
      if (data.csrf_token) {
        this.csrfToken = data.csrf_token;
      }
      if (data.principal) {
        this.principal = data.principal;
      }
      this.accessExpiresAt = Date.now() + (data.expires_in || 900) * 1000;
      this.status = "authenticated";

      this.notify();
      return this.accessToken!;
    } catch (err) {
      this.clearCredentials("expired");
      throw err;
    }
  }

  /**
   * 用户登出
   */
  async logout(): Promise<void> {
    try {
      const logoutUrl = `${this.baseUrl}/v1/auth/logout`;
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-Request-ID": typeof crypto !== "undefined" && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()),
      };
      if (this.csrfToken) {
        headers["X-CSRF-Token"] = this.csrfToken;
      }

      await fetch(logoutUrl, {
        method: "POST",
        credentials: "same-origin",
        headers,
        body: JSON.stringify({ audience: this.audience }),
      });
    } finally {
      // 无论后端是否返回 200，前端立即清除内存凭据并推进纪元
      this.clearCredentials("anonymous");
    }
  }

  /**
   * 手动注入/更新会话凭据（用于运维鉴权或开发调试）
   */
  setSession(
    accessToken: string,
    principal: UserPrincipal,
    csrfToken?: string,
    expiresAt?: number
  ): void {
    this.sessionEpoch += 1;
    this.accessToken = accessToken;
    this.principal = principal;
    this.csrfToken = csrfToken || null;
    this.accessExpiresAt = expiresAt || Date.now() + 3600 * 1000;
    this.status = "authenticated";
    this.notify();
  }

  /** 清除内存凭据并触发纪元更新 */
  private clearCredentials(newStatus: "anonymous" | "expired"): void {
    this.sessionEpoch += 1;
    this.accessToken = null;
    this.csrfToken = null;
    this.principal = null;
    this.accessExpiresAt = null;
    this.status = newStatus;
    this.notify();
  }
}
