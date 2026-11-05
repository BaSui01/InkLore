/**
 * @inklore/run-ui - 智能体运行检查器 (RunInspector)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.5
 * 
 * 核心契约与红线：
 * 1. Props 采用严格判别联合：mode="owner" (创作者) vs mode="redacted-summary" (管理端审计)。
 * 2. 管理端仅接收脱敏元数据（node_count, duration_ms, cost_micros, error_code），绝不传递作者完整正文/节点细节。
 * 3. 创作者端呈现节点状态机、思考摘要、逐字增量正文与审批流。
 * 4. 纯净排版与零 Emoji 铁律。
 */

import React, { useState } from "react";
import {
  AgentRun,
  UsageUpdatedPayload,
  NodeUpdatedPayload,
} from "@inklore/contracts";
import {
  useTranslation,
  ModelIcon as Cpu,
  MetricsIcon as Activity,
  TerminalIcon as Terminal,
  StatusSuccessIcon,
  StatusDangerIcon,
  StatusWarningIcon,
  LoadingSpinner,
} from "@inklore/i18n";

export interface OwnerRunInspectorProps {
  mode: "owner";
  runId: string;
  status: AgentRun["status"];
  textBuffer: string;
  reasoningSummary: string;
  nodes: NodeUpdatedPayload[];
  usage: UsageUpdatedPayload | null;
  onCancel?: () => void;
}

export interface AdminRunInspectorProps {
  mode: "redacted-summary";
  runId: string;
  taskKind: string;
  status: string;
  nodeCount: number;
  durationMs: number;
  costMicros?: number;
  errorCode?: string;
  onRefresh?: () => void;
}

export type RunInspectorProps = OwnerRunInspectorProps | AdminRunInspectorProps;

export function RunInspector(props: RunInspectorProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<"text" | "reasoning" | "nodes">("text");

  // 管理端脱敏展示模式
  if (props.mode === "redacted-summary") {
    return (
      <div
        className="run-inspector-admin break-words"
        style={{
          border: "1px solid var(--border-decorative)",
          borderRadius: "8px",
          padding: "16px",
          backgroundColor: "var(--bg-surface)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "12px" }}>
          <Cpu size={18} color="var(--action-primary)" />
          <span style={{ fontWeight: 600 }}>{t("agentAudit.title")} ({props.runId})</span>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))", gap: "12px", fontSize: "13px" }}>
          <div>
            <span style={{ color: "var(--text-muted)" }}>{t("agentAudit.taskKind")}:</span> {props.taskKind}
          </div>
          <div>
            <span style={{ color: "var(--text-muted)" }}>{t("agentAudit.status")}:</span> {t(`agentState.${props.status}`)}
          </div>
          <div>
            <span style={{ color: "var(--text-muted)" }}>{t("agentAudit.nodes")}:</span> {props.nodeCount}
          </div>
          <div>
            <span style={{ color: "var(--text-muted)" }}>{t("agentAudit.duration")}:</span> {props.durationMs}ms
          </div>
          {props.costMicros !== undefined && (
            <div>
              <span style={{ color: "var(--text-muted)" }}>{t("agentAudit.cost")}:</span> {props.costMicros}
            </div>
          )}
          {props.errorCode && (
            <div>
              <span style={{ color: "var(--status-danger)" }}>{t("agentAudit.errorCode")}:</span> {props.errorCode}
            </div>
          )}
        </div>
      </div>
    );
  }

  // 创作者端完整模式
  const isRunning = props.status === "running";

  return (
    <div
      className="run-inspector-owner"
      style={{
        border: "1px solid var(--border-decorative)",
        borderRadius: "8px",
        backgroundColor: "var(--bg-surface)",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "10px 16px",
          backgroundColor: "var(--bg-subtle)",
          borderBottom: "1px solid var(--border-decorative)",
        }}
      >
        <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: "8px", fontSize: "14px", fontWeight: 600 }}>
          {isRunning ? <LoadingSpinner size={16} /> : <Terminal size={16} />}
          <span style={{ overflowWrap: "anywhere", minWidth: 0 }}>{props.runId}</span>
          <span
            style={{
              fontSize: "12px",
              padding: "2px 8px",
              borderRadius: "4px",
              backgroundColor: props.status === "succeeded" ? "var(--status-success)" : "var(--bg-canvas)",
              color: props.status === "succeeded" ? "#ffffff" : "var(--text-secondary)",
            }}
          >
            {t(`agentState.${props.status}`)}
          </span>
        </div>

        {isRunning && props.onCancel && (
          <button
            type="button"
            onClick={props.onCancel}
            style={{
              border: "1px solid var(--status-danger)",
              backgroundColor: "transparent",
              color: "var(--status-danger)",
              padding: "4px 10px",
              borderRadius: "4px",
              fontSize: "12px",
              cursor: "pointer",
            }}
          >
            {t("creator.stop_generation")}
          </button>
        )}
      </div>

      <div style={{ display: "flex", borderBottom: "1px solid var(--border-decorative)", fontSize: "13px" }}>
        <button
          type="button"
          onClick={() => setActiveTab("text")}
          style={{
            flex: 1,
            padding: "8px",
            border: "none",
            backgroundColor: activeTab === "text" ? "var(--bg-surface)" : "var(--bg-subtle)",
            fontWeight: activeTab === "text" ? 600 : 400,
            cursor: "pointer",
          }}
        >
          {t("creator.chapter_body")}
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("reasoning")}
          style={{
            flex: 1,
            padding: "8px",
            border: "none",
            backgroundColor: activeTab === "reasoning" ? "var(--bg-surface)" : "var(--bg-subtle)",
            fontWeight: activeTab === "reasoning" ? 600 : 400,
            cursor: "pointer",
          }}
        >
          {t("creator.ai_flavor_detection")}
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("nodes")}
          style={{
            flex: 1,
            padding: "8px",
            border: "none",
            backgroundColor: activeTab === "nodes" ? "var(--bg-surface)" : "var(--bg-subtle)",
            fontWeight: activeTab === "nodes" ? 600 : 400,
            cursor: "pointer",
          }}
        >
          {t("admin.workflows")} ({props.nodes.length})
        </button>
      </div>

      <div style={{ padding: "16px", minHeight: "200px", maxHeight: "400px", overflowY: "auto", fontSize: "14px" }}>
        {activeTab === "text" && (
          <div style={{ whiteSpace: "pre-wrap", lineHeight: 1.6 }}>
            {props.textBuffer || (isRunning ? t("creator.generate_stream") : t("common.empty"))}
          </div>
        )}

        {activeTab === "reasoning" && (
          <div style={{ color: "var(--text-secondary)", fontStyle: "italic", whiteSpace: "pre-wrap" }}>
            {props.reasoningSummary || t("common.empty")}
          </div>
        )}

        {activeTab === "nodes" && (
          <div>
            {props.nodes.map((node) => (
              <div
                key={node.node_id}
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  padding: "6px 0",
                  borderBottom: "1px solid var(--border-decorative)",
                  fontSize: "13px",
                }}
              >
                <span>{node.node_id}</span>
                <span style={{ color: node.status === "succeeded" ? "var(--status-success)" : "var(--text-muted)" }}>
                  {node.status}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {props.usage && (
        <div
          style={{
            padding: "8px 16px",
            backgroundColor: "var(--bg-subtle)",
            borderTop: "1px solid var(--border-decorative)",
            fontSize: "12px",
            color: "var(--text-muted)",
            display: "flex",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: "8px",
          }}
        >
          <span>{t("agentAudit.inputTokens")}: {props.usage.input_tokens}</span>
          <span>{t("agentAudit.outputTokens")}: {props.usage.output_tokens}</span>
          <span>{t("agentAudit.cost")}: {props.usage.cost_micros} {props.usage.currency} / 1,000,000 {props.usage.estimated ? t("agentAudit.estimated") : ""}</span>
        </div>
      )}
    </div>
  );
}