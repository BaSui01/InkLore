import type { NavView } from "./components/NavigationRail";

export type CreatorView = NavView | "agent-guide" | "login" | "register" | "forgot-password";
export const AUTH_VIEWS: CreatorView[] = ["login", "register", "forgot-password"];
const VIEWS: CreatorView[] = ["home", "editor", "agent-guide", "outline", "worldview", "analysis", ...AUTH_VIEWS];
const NAVIGATION_EVENT = "creator:navigate";

export function creatorPath(view: CreatorView): string {
  return view === "home" ? "/" : `/${view}`;
}

function viewAt(path: string): CreatorView | undefined {
  if (path === "/" || path === "/home") return "home";
  return VIEWS.find(view => path === creatorPath(view));
}

/** Only existing, same-origin content routes can be authentication return targets. */
export function safeReturnTo(value: string | null): string | undefined {
  if (!value?.startsWith("/") || value.startsWith("//") || value.includes("\\")) return;
  try {
    const url = new URL(value, window.location.origin);
    const view = viewAt(url.pathname);
    if (url.origin !== window.location.origin || !view || AUTH_VIEWS.includes(view)) return;
    return creatorPath(view) + url.search;
  } catch { return; }
}

export function readCreatorLocation() {
  const view = viewAt(window.location.pathname) ?? "home";
  const query = new URLSearchParams(window.location.search);
  return {
    view,
    returnTo: safeReturnTo(query.get("returnTo")),
    novelId: view === "editor" ? query.get("novelId") || "" : "",
  };
}

export function authPath(view: CreatorView, returnTo?: string): string {
  const target = safeReturnTo(returnTo ?? null);
  return creatorPath(view) + (target ? `?${new URLSearchParams({ returnTo: target })}` : "");
}

export function navigateCreator(path: string, replace = false) {
  if (window.location.pathname + window.location.search + window.location.hash === path) return;
  window.history[replace ? "replaceState" : "pushState"](null, "", path);
  window.dispatchEvent(new Event(NAVIGATION_EVENT));
}

export function subscribeCreatorLocation(listener: () => void) {
  window.addEventListener("popstate", listener);
  window.addEventListener(NAVIGATION_EVENT, listener);
  return () => {
    window.removeEventListener("popstate", listener);
    window.removeEventListener(NAVIGATION_EVENT, listener);
  };
}

/** Convert saved Hash links once; normal navigation uses History exclusively. */
export function normalizeCreatorLocation() {
  if (window.location.hash.startsWith("#/")) {
    const [path, search = ""] = window.location.hash.slice(1).split("?");
    const view = viewAt(path);
    if (view) {
      const query = new URLSearchParams(search);
      const previous = query.get("returnTo");
      if (previous && VIEWS.includes(previous as CreatorView)) {
        query.set("returnTo", creatorPath(previous as CreatorView));
      }
      const suffix = query.toString();
      window.history.replaceState(null, "", creatorPath(view) + (suffix ? `?${suffix}` : ""));
    }
  }
  if (window.location.pathname === "/home") window.history.replaceState(null, "", "/" + window.location.search);
}
