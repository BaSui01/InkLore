/**
 * @inklore/recovery - 安全 LocalStorage 封装
 * 仅用于非敏感本地偏好（字号、展开状态等），绝不存正文与访问令牌
 */

export class SafeLocalStorage {
  /** 安全读取 JSON */
  static getItem<T>(key: string, fallback: T): T {
    if (typeof localStorage === "undefined") {
      return fallback;
    }
    try {
      const item = localStorage.getItem(key);
      if (item === null) return fallback;
      return JSON.parse(item) as T;
    } catch {
      return fallback;
    }
  }

  /** 安全写入 JSON */
  static setItem<T>(key: string, value: T): boolean {
    if (typeof localStorage === "undefined") {
      return false;
    }
    try {
      localStorage.setItem(key, JSON.stringify(value));
      return true;
    } catch {
      console.warn("InkLore diagnostic", { event: "recovery.preference_write_failed" });
      return false;
    }
  }

  /** 移除项 */
  static removeItem(key: string): void {
    if (typeof localStorage === "undefined") return;
    try {
      localStorage.removeItem(key);
    } catch {}
  }
}
