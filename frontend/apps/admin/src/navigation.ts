/**
 * @inklore/admin-console - HTML5 History API 统一路径路由系统
 * 严格遵循无 # 号普通路径契约与平滑自动迁移
 */

export type AdminView = "dashboard" | "channels" | "models" | "users" | "agents" | "audit" | "project-logs" | "mail-settings";
export type AdminAuthView = "login" | "forgot-password";
export type AnyAdminView = AdminView | AdminAuthView;

export const VALID_ADMIN_VIEWS: AdminView[] = [
  "dashboard",
  "channels",
  "models",
  "users",
  "agents",
  "audit",
  "project-logs",
  "mail-settings",
];

export const AUTH_ADMIN_VIEWS: AdminAuthView[] = ["login", "forgot-password"];

const NAVIGATION_EVENT = "admin:navigate";

export function adminPath(view: AnyAdminView): string {
  return `/${view}`;
}

export function viewAt(rawPath: string): AnyAdminView | undefined {
  const clean = rawPath.replace(/\/+$/, "") || "/";
  if (clean === "/" || clean === "/admin.html") return "dashboard";

  const segment = clean.replace(/^\//, "");
  if (AUTH_ADMIN_VIEWS.includes(segment as AdminAuthView)) {
    return segment as AdminAuthView;
  }
  if (VALID_ADMIN_VIEWS.includes(segment as AdminView)) {
    return segment as AdminView;
  }
  return undefined;
}

export function safeReturnTo(value: string | null): AdminView | undefined {
  if (!value) return undefined;
  const clean = value.replace(/^\//, "");
  if (VALID_ADMIN_VIEWS.includes(clean as AdminView)) {
    return clean as AdminView;
  }
  return undefined;
}

export function readAdminLocation(): {
  view: AnyAdminView;
  returnTo?: AdminView;
} {
  const pathname = window.location.pathname;
  const view = viewAt(pathname) || "dashboard";
  const params = new URLSearchParams(window.location.search);
  const returnTo = safeReturnTo(params.get("returnTo"));
  return { view, returnTo };
}

export function navigateAdmin(target: string | AnyAdminView, returnTo?: AdminView, replace = false) {
  let targetPath: string;
  if (typeof target === "string" && target.startsWith("/")) {
    targetPath = target;
  } else {
    const viewName = target as AnyAdminView;
    targetPath = adminPath(viewName);
    if (AUTH_ADMIN_VIEWS.includes(viewName as AdminAuthView) && returnTo) {
      targetPath += `?returnTo=${encodeURIComponent(returnTo)}`;
    }
  }

  const currentFull = window.location.pathname + window.location.search;
  if (currentFull === targetPath) return;

  if (replace) {
    window.history.replaceState(null, "", targetPath);
  } else {
    window.history.pushState(null, "", targetPath);
  }
  window.dispatchEvent(new Event(NAVIGATION_EVENT));
}

export function subscribeAdminLocation(listener: () => void): () => void {
  window.addEventListener("popstate", listener);
  window.addEventListener(NAVIGATION_EVENT, listener);
  return () => {
    window.removeEventListener("popstate", listener);
    window.removeEventListener(NAVIGATION_EVENT, listener);
  };
}

/**
 * 自动规范化旧 Hash 链接或根路径：
 * 将 http://.../#/dashboard 或 http://.../#/users 自动转换为 http://.../dashboard 或 http://.../users
 */
export function normalizeAdminLocation(): void {
  if (window.location.hash.startsWith("#/")) {
    const [hashPath, search = ""] = window.location.hash.slice(1).split("?");
    const view = viewAt(hashPath);
    if (view) {
      const searchParams = new URLSearchParams(search);
      const queryStr = searchParams.toString();
      const targetUrl = adminPath(view) + (queryStr ? `?${queryStr}` : "");
      window.history.replaceState(null, "", targetUrl);
      window.dispatchEvent(new Event(NAVIGATION_EVENT));
      return;
    }
  }

  // 如果直接访问根路径 / 或 /admin.html，规范化为 /dashboard
  const currentPath = window.location.pathname;
  if (currentPath === "/" || currentPath === "/admin.html") {
    const search = window.location.search;
    window.history.replaceState(null, "", "/dashboard" + search);
    window.dispatchEvent(new Event(NAVIGATION_EVENT));
  }
}
