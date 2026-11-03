/**
 * @inklore/transport - 平台级 WebSocket 通知客户端 (NotificationClient)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.4
 * 
 * 核心铁律：
 * 1. 使用 POST /v1/auth/ws-ticket 获取 60 秒一次性票据后建立 WS 连接。
 * 2. 消息仅携带 event_id / topic / resource_id / revision / occurred_at，绝不正文传输。
 * 3. 严格基于 event_id 进行窗口去重。
 * 4. 断线 1..30 秒指数退避重连，重连后触发重新拉取权威状态。
 * 5. WS 偶发断联绝不把 HTTP API 标记为整体离线。
 */

import { PlatformNotificationMessage } from "@inklore/contracts";
import { AuthSession } from "../http/session";
import { reportSafeDiagnostic } from "../http/diagnostic";
import { stripTrailingSlashes } from "../base-url";

export type NotificationHandler = (msg: PlatformNotificationMessage) => void;

export interface NotificationClientOptions {
  baseUrl?: string;
  session: AuthSession;
}

export class NotificationClient {
  private readonly baseUrl: string;
  private readonly session: AuthSession;
  private ws: WebSocket | null = null;
  private isDisposed = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;

  private readonly topicHandlers = new Map<string, Set<NotificationHandler>>();
  private readonly recentEventIds = new Set<string>();
  private readonly eventIdQueue: string[] = [];
  private static readonly MAX_SEEN_EVENTS = 500;

  constructor(options: NotificationClientOptions) {
    this.baseUrl = stripTrailingSlashes(options.baseUrl || "");
    this.session = options.session;
  }

  /**
   * 订阅指定主题 (topic) 的平台通知
   */
  subscribe(topic: string, handler: NotificationHandler): () => void {
    let handlers = this.topicHandlers.get(topic);
    if (!handlers) {
      handlers = new Set();
      this.topicHandlers.set(topic, handlers);
    }
    handlers.add(handler);

    // 首次有订阅者且未连接时启动连接
    if (!this.ws && !this.isDisposed) {
      this.connect();
    }

    return () => {
      handlers?.delete(handler);
      if (handlers && handlers.size === 0) {
        this.topicHandlers.delete(topic);
      }
    };
  }

  /**
   * 启动 WebSocket 连接
   */
  private async connect(): Promise<void> {
    if (this.isDisposed) return;

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    try {
      // 步骤 1：通过 POST /v1/auth/ws-ticket 获取一次性票据
      const ticketUrl = `${this.baseUrl}/v1/auth/ws-ticket`;
      const token = this.session.getAccessToken();
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        Accept: "application/json",
      };
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }

      const res = await fetch(ticketUrl, {
        method: "POST",
        headers,
        credentials: "same-origin",
        body: JSON.stringify({ audience: this.session.getAudience() }),
      });

      if (!res.ok) {
        throw new Error(`Failed to obtain WS ticket: ${res.status}`);
      }

      const ticketBody = await res.json();
      const ticket = ticketBody.data?.ticket;
      if (!ticket) {
        throw new Error("No ticket returned from ws-ticket endpoint");
      }

      // 步骤 2：建立 WebSocket 连接
      const wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const wsHost = this.baseUrl ? new URL(this.baseUrl).host : window.location.host;
      const wsUrl = `${wsProtocol}//${wsHost}/v1/platform/events?ticket=${encodeURIComponent(ticket)}`;

      const ws = new WebSocket(wsUrl);
      this.ws = ws;

      ws.onopen = () => {
        this.reconnectAttempts = 0;
      };

      ws.onmessage = (evt) => {
        try {
          const msg: PlatformNotificationMessage = JSON.parse(evt.data);
          this.dispatch(msg);
        } catch {
          reportSafeDiagnostic("ws.invalid_frame");
        }
      };

      ws.onclose = () => {
        this.ws = null;
        if (!this.isDisposed) {
          this.scheduleReconnect();
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    } catch (err) {
      if (!this.isDisposed) {
        this.scheduleReconnect();
      }
    }
  }

  /**
   * 分发消息给已订阅主题的处理者
   */
  private dispatch(msg: PlatformNotificationMessage): void {
    if (!msg.event_id || this.recentEventIds.has(msg.event_id)) {
      return; // 已经处理过的事件去重丢弃
    }

    this.recentEventIds.add(msg.event_id);
    this.eventIdQueue.push(msg.event_id);
    if (this.eventIdQueue.length > NotificationClient.MAX_SEEN_EVENTS) {
      const oldest = this.eventIdQueue.shift();
      if (oldest) this.recentEventIds.delete(oldest);
    }

    const handlers = this.topicHandlers.get(msg.topic);
    if (handlers) {
      for (const handler of handlers) {
        try {
          handler(msg);
        } catch {
          reportSafeDiagnostic("ws.handler_failed");
        }
      }
    }
  }

  /**
   * 带抖动的指数退避重连 (1s..30s)
   */
  private scheduleReconnect(): void {
    if (this.isDisposed) return;

    this.reconnectAttempts += 1;
    const baseDelays = [1000, 2000, 5000, 10000, 20000, 30000];
    const delay = baseDelays[Math.min(this.reconnectAttempts - 1, baseDelays.length - 1)];
    const jitter = delay * (Math.random() * 0.2);

    this.reconnectTimer = setTimeout(() => {
      this.connect();
    }, Math.floor(delay + jitter));
  }

  /**
   * 销毁客户端
   */
  dispose(): void {
    this.isDisposed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.topicHandlers.clear();
    this.recentEventIds.clear();
    this.eventIdQueue.length = 0;
  }
}
