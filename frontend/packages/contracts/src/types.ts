/**
 * @inklore/contracts - 核心品牌类型与值对象契约
 * 严格遵循 ADR-0016 / 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md
 * 杜绝裸类型滥用，全平台 Revision / Seq 采用严格 number 安全整数。
 */

/** 小写带连字符的 UUIDv4 品牌类型 */
export type UUID = string & { readonly __brand: "UUID" };

const UUIDV4_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isValidUUID(val: unknown): val is UUID {
  return typeof val === "string" && UUIDV4_REGEX.test(val);
}

export function parseUUID(val: string): UUID {
  if (!isValidUUID(val)) {
    throw new TypeError(`Invalid UUIDv4: "${val}"`);
  }
  return val;
}

/** 生成随机 UUIDv4 */
export function generateUUID(): UUID {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID() as UUID;
  }
  // 浏览器降级随机生成器
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  }) as UUID;
}

/** 协议与事实版本号 (安全整数 1..9007199254740991) */
export type Revision = number & { readonly __brand: "Revision" };

export const MIN_SAFE_REVISION = 1;
export const MAX_SAFE_REVISION = Number.MAX_SAFE_INTEGER; // 9007199254740991

export function isValidRevision(val: unknown): val is Revision {
  return typeof val === "number" && Number.isSafeInteger(val) && val >= MIN_SAFE_REVISION && val <= MAX_SAFE_REVISION;
}

export function parseRevision(val: unknown): Revision {
  if (!isValidRevision(val)) {
    throw new TypeError(`Invalid Revision (must be safe integer between ${MIN_SAFE_REVISION} and ${MAX_SAFE_REVISION}): ${val}`);
  }
  return val;
}

/** 流事件单调递增游标 (安全整数 1..9007199254740991) */
export type Seq = number & { readonly __brand: "Seq" };

export function isValidSeq(val: unknown): val is Seq {
  return typeof val === "number" && Number.isSafeInteger(val) && val >= 1 && val <= MAX_SAFE_REVISION;
}

export function parseSeq(val: unknown): Seq {
  if (!isValidSeq(val)) {
    throw new TypeError(`Invalid Seq (must be safe integer between 1 and ${MAX_SAFE_REVISION}): ${val}`);
  }
  return val;
}

/** ISO-8601 UTC 时间字符串品牌类型 */
export type Timestamp = string & { readonly __brand: "Timestamp" };

export function isValidTimestamp(val: unknown): val is Timestamp {
  if (typeof val !== "string") return false;
  const d = new Date(val);
  return !Number.isNaN(d.getTime());
}

/** 前端应用端别：Creator (端口 3000) 与 Admin (端口 3001) 物理隔离 */
export type Audience = "creator" | "admin";

/** 作用域键：用于隔离租户、用户与会话，防止跨租户/跨主体数据污染 */
export interface ScopeKey {
  tenantId: string;
  userId: string;
  audience: Audience;
  sessionEpoch: number;
  permissionRevision?: number;
  novelId?: string;
}

export function isSameScope(a: ScopeKey | null | undefined, b: ScopeKey | null | undefined): boolean {
  if (!a || !b) return false;
  return (
    a.tenantId === b.tenantId &&
    a.userId === b.userId &&
    a.audience === b.audience &&
    a.sessionEpoch === b.sessionEpoch &&
    a.novelId === b.novelId
  );
}

/** 资源唯一坐标 */
export interface ResourceKey {
  domain: string;
  type: string;
  id: string;
}

/** 认证主体信息 */
export interface UserPrincipal {
  user_id: string;
  tenant_id: string;
  display_name: string;
  roles: string[];
  global_roles: string[];
  audience: Audience;
  session_id: string;
  user_revision: number;
  membership_revision?: number;
}

export interface PlatformTenantSummary {
  id: UUID;
  name: string;
}

/** 会话快照状态机 */
export type SessionStatus = "bootstrapping" | "authenticated" | "anonymous" | "expired";

export interface SessionSnapshot {
  status: SessionStatus;
  principal: UserPrincipal | null;
  scope: ScopeKey | null;
  accessExpiresAt: number | null;
  sessionEpoch: number;
}
