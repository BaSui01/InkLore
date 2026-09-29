/**
 * @inklore/transport - 统一 SSE 客户端 (StreamClient)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.4
 * 
 * 核心契约与红线：
 * 1. 基于 fetch ReadableStream 实现，支持 Authorization Bearer 头（严禁将 JWT 放 URL query）。
 * 2. UTF-8 跨 chunk 解码与多行 data: 规范支持。
 * 3. 严格递增 seq 与 Last-Event-ID 游标断线重连 (1s/2s/4s/8s/16s/30s 带抖动)。
 * 4. 15 秒心跳保活检测：若 45 秒内无任何心跳/数据包，自动断开重连。
 * 5. 遇到 run.succeeded / run.failed / run.cancelled 终态标记完成并停止重连；EOF 缺少终态绝不推断成功。
 * 6. 401 触发单飞刷新重试，403 停止，410 游标失效通知上层 GetRun。
 * 7. unsubscribe 仅关闭客户端网络流，绝不静默调用 CancelRun。
 */

import {
  RunEventEnvelope,
  RunEventType,
  Seq,
  isTerminalEvent,
  parseSeq,
  ApiError,
} from "@inklore/contracts";
import { AuthSession } from "../http/session";
import { RunEventBuffer } from "./buffer";
import { reportSafeDiagnostic } from "../http/diagnostic";

export type ConnectionState = "idle" | "connecting" | "open" | "reconnecting" | "closed";

export interface StreamSubscribeOptions {
  runId: string;
  lastSeq?: Seq | number;
  onEvent?: (event: RunEventEnvelope) => void;
  onConnection?: (state: ConnectionState) => void;
  onError?: (error: Error) => void;
  onTerminal?: (terminalEvent: RunEventType, envelope: RunEventEnvelope) => void;
  onCursorExpired?: () => void;
  signal?: AbortSignal;
}

export interface StreamSubscription {
  unsubscribe: () => void;
  getCursor: () => Seq | 0;
  getBuffer: () => RunEventBuffer;
}

export interface StreamClientOptions {
  baseUrl?: string;
  session: AuthSession;
  heartbeatTimeoutMs?: number; // 默认 45000ms (3 * 15s)
}

export class StreamClient {
  private readonly baseUrl: string;
  private readonly session: AuthSession;
  private readonly heartbeatTimeoutMs: number;

  constructor(options: StreamClientOptions) {
    this.baseUrl = (options.baseUrl || "").replace(/\/+$/, "");
    this.session = options.session;
    this.heartbeatTimeoutMs = options.heartbeatTimeoutMs || 45000;
  }

  /**
   * 订阅智能体运行事件流
   */
  subscribe(options: StreamSubscribeOptions): StreamSubscription {
    const { runId } = options;
    const buffer = new RunEventBuffer(runId);

    let isDisposed = false;
    let abortController: AbortController | null = null;
    let keepaliveTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectAttempts = 0;
    let connectionState: ConnectionState = "idle";
    let currentSeq: Seq | 0 = options.lastSeq ? (parseSeq(options.lastSeq) as Seq) : 0;

    const setConnectionState = (state: ConnectionState) => {
      connectionState = state;
      options.onConnection?.(state);
    };

    const resetKeepaliveTimer = () => {
      if (keepaliveTimer) {
        clearTimeout(keepaliveTimer);
      }
      keepaliveTimer = setTimeout(() => {
        // 超过 45s 无心跳，判定网络死锁，强制断开重连
        if (!isDisposed && connectionState === "open") {
          reportSafeDiagnostic("sse.keepalive_timeout");
          abortController?.abort();
        }
      }, this.heartbeatTimeoutMs);
    };

    const clearTimers = () => {
      if (keepaliveTimer) {
        clearTimeout(keepaliveTimer);
        keepaliveTimer = null;
      }
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    };

    const connect = async () => {
      if (isDisposed) return;

      clearTimers();
      abortController = new AbortController();

      if (options.signal) {
        options.signal.addEventListener("abort", () => {
          cleanup();
        }, { once: true });
      }

      setConnectionState(reconnectAttempts === 0 ? "connecting" : "reconnecting");

      try {
        const url = `${this.baseUrl}/v1/agents/runs/${encodeURIComponent(runId)}/events`;
        const headers: Record<string, string> = {
          Accept: "text/event-stream",
          "Cache-Control": "no-cache",
        };

        const token = this.session.getAccessToken();
        if (token) {
          headers["Authorization"] = `Bearer ${token}`;
        }

        if (currentSeq > 0) {
          headers["Last-Event-ID"] = String(currentSeq);
        }

        const res = await fetch(url, {
          method: "GET",
          headers,
          credentials: "same-origin",
          signal: abortController.signal,
        });

        // 401 触发单飞刷新一次
        if (res.status === 401) {
          try {
            await this.session.refresh();
            if (!isDisposed) {
              return connect();
            }
          } catch (refreshErr) {
            setConnectionState("closed");
            options.onError?.(new ApiError({
              httpStatus: 401,
              code: "auth.unauthenticated",
              message_key: "errors.auth.unauthenticated",
            }));
            return;
          }
        }

        // 403 禁止访问
        if (res.status === 403) {
          setConnectionState("closed");
          options.onError?.(new ApiError({
            httpStatus: 403,
            code: "auth.forbidden",
            message_key: "errors.auth.forbidden",
          }));
          return;
        }

        // 410 事件游标已过期
        if (res.status === 410) {
          setConnectionState("closed");
          options.onCursorExpired?.();
          return;
        }

        if (!res.ok) {
          throw new Error(`SSE connect failed with status ${res.status}`);
        }

        if (!res.body) {
          throw new Error("Response body is not a ReadableStream");
        }

        setConnectionState("open");
        reconnectAttempts = 0; // 连接成功重置退避计数
        resetKeepaliveTimer();

        // 开始流式解析
        await this.readEventStream(
          res.body,
          (envelope) => {
            resetKeepaliveTimer();
            if (envelope.seq > currentSeq) {
              currentSeq = envelope.seq;
            }

            const pushed = buffer.push(envelope);
            if (pushed) {
              options.onEvent?.(envelope);
            }

            // 终态检测
            if (isTerminalEvent(envelope.event)) {
              cleanup();
              options.onTerminal?.(envelope.event, envelope);
            }
          },
          () => {
            // 收到心跳注释行
            resetKeepaliveTimer();
          },
          abortController.signal
        );

        // 流正常读完 (EOF) 但未收到终态事件 -> 判定为 disconnected 并重连
        if (!isDisposed && !buffer.getAggregatedState().isCompleted) {
          scheduleReconnect();
        }
      } catch (err: any) {
        if (isDisposed || abortController.signal.aborted) {
          return;
        }

        options.onError?.(err);
        scheduleReconnect();
      }
    };

    /**
     * 带抖动的指数退避重连 (1s, 2s, 4s, 8s, 16s, 30s)
     */
    const scheduleReconnect = () => {
      if (isDisposed || buffer.getAggregatedState().isCompleted) {
        return;
      }

      setConnectionState("reconnecting");
      reconnectAttempts += 1;

      const baseDelays = [1000, 2000, 4000, 8000, 16000, 30000];
      const delayIndex = Math.min(reconnectAttempts - 1, baseDelays.length - 1);
      const baseDelay = baseDelays[delayIndex];
      // 增加 0 ~ 30% 抖动
      const jitter = baseDelay * (Math.random() * 0.3);
      const actualDelay = Math.floor(baseDelay + jitter);

      reconnectTimer = setTimeout(() => {
        connect();
      }, actualDelay);
    };

    const cleanup = () => {
      isDisposed = true;
      clearTimers();
      if (abortController) {
        abortController.abort();
        abortController = null;
      }
      setConnectionState("closed");
    };

    // 立即发起首次连接
    connect();

    return {
      unsubscribe: cleanup,
      getCursor: () => currentSeq,
      getBuffer: () => buffer,
    };
  }

  /**
   * 基于 ReadableStream 的 UTF-8 跨 chunk SSE 行解析器
   */
  private async readEventStream(
    stream: ReadableStream<Uint8Array>,
    onEvent: (event: RunEventEnvelope) => void,
    onHeartbeat: () => void,
    signal: AbortSignal
  ): Promise<void> {
    const reader = stream.getReader();
    const decoder = new TextDecoder("utf-8");

    let buffer = "";
    let currentEventName = "message";
    let currentDataLines: string[] = [];
    let currentId: string | null = null;

    try {
      while (!signal.aborted) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split(/\r\n|\r|\n/);
        // 最后一个未换行的片段保留在 buffer
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (line.startsWith(":")) {
            // 注释行/心跳保活
            onHeartbeat();
            continue;
          }

          if (line === "") {
            // 空行：分发当前 SSE 事件
            if (currentDataLines.length > 0) {
              const fullData = currentDataLines.join("\n");
              try {
                const parsed = JSON.parse(fullData);
                // 兼容外包格式与标准封套
                const envelope: RunEventEnvelope = {
                  run_id: parsed.run_id || "",
                  seq: parseSeq(parsed.seq ?? Number(currentId || "1")),
                  occurred_at: parsed.occurred_at || new Date().toISOString(),
                  event: (parsed.event || currentEventName) as RunEventType,
                  payload: parsed.payload !== undefined ? parsed.payload : parsed,
                };
                onEvent(envelope);
              } catch {
                reportSafeDiagnostic("sse.invalid_frame");
              }
            }

            // 重置状态
            currentEventName = "message";
            currentDataLines = [];
            currentId = null;
            continue;
          }

          if (line.startsWith("event:")) {
            currentEventName = line.slice(6).trim();
          } else if (line.startsWith("data:")) {
            currentDataLines.push(line.slice(5).trimStart());
          } else if (line.startsWith("id:")) {
            currentId = line.slice(3).trim();
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }
}
