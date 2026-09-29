/**
 * @inklore/run-ui - 人工审批卡片组件 (ApprovalCard)
 * 严格遵循 20-FRONTEND-REUSE-AND-STATE-CONTRACTS.md §3.10
 * 
 * 核心契约与红线：
 * 1. 仅创作者端本人 (actorId === owner_user_id) 具备真实审批权限。
 * 2. 必须完整呈现 run/node/tool_name/risk_level/arguments_summary/expires_at。
 * 3. 严格禁止出现任何 Unicode Emoji。
 */

import React from "react";
import { ApprovalRequiredPayload } from "@inklore/contracts";
import { useTranslation, ShieldIcon, ActionCheckIcon, ActionCloseIcon } from "@inklore/i18n";

export interface ApprovalCardProps {
  approval: ApprovalRequiredPayload;
  actorId: string;
  canDecide: boolean;
  onApprove: (approvalId: string) => void;
  onReject: (approvalId: string, reason?: string) => void;
  disabled?: boolean;
}

export function ApprovalCard({
  approval,
  actorId,
  canDecide,
  onApprove,
  onReject,
  disabled = false,
}: ApprovalCardProps) {
  const { t } = useTranslation();

  const isHighRisk = approval.risk_level === "high" || approval.risk_level === "critical";

  return (
    <div
      role="region"
      aria-label="Agent Tool Approval Request"
      className="approval-card"
      style={{
        border: `1px solid ${isHighRisk ? "var(--status-danger)" : "var(--status-warning)"}`,
        borderRadius: "8px",
        padding: "16px",
        backgroundColor: "var(--bg-surface)",
        margin: "12px 0",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: "10px", marginBottom: "12px" }}>
        <ShieldIcon size={20} color={isHighRisk ? "var(--status-danger)" : "var(--status-warning)"} />
        <span style={{ fontWeight: 600, fontSize: "15px" }}>
          {t("errors.agent.approval_required")}
        </span>
        <span
          style={{
            fontSize: "12px",
            padding: "2px 8px",
            borderRadius: "4px",
            backgroundColor: isHighRisk ? "var(--status-danger)" : "var(--status-warning)",
            color: "#ffffff",
            fontWeight: 500,
          }}
        >
          {approval.risk_level.toUpperCase()}
        </span>
      </div>

      <div style={{ fontSize: "13px", color: "var(--text-secondary)", marginBottom: "8px" }}>
        <div>
          <strong>Tool:</strong> {approval.tool_name}
        </div>
        <div>
          <strong>Node ID:</strong> {approval.node_id}
        </div>
        <div>
          <strong>Expires At:</strong> {approval.expires_at}
        </div>
      </div>

      {approval.arguments_summary && Object.keys(approval.arguments_summary).length > 0 && (
        <div
          style={{
            backgroundColor: "var(--bg-subtle)",
            padding: "8px 12px",
            borderRadius: "6px",
            fontSize: "12px",
            marginBottom: "12px",
            fontFamily: "monospace",
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: "4px" }}>Arguments Summary:</div>
          <pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>
            {JSON.stringify(approval.arguments_summary, null, 2)}
          </pre>
        </div>
      )}

      {canDecide ? (
        <div style={{ display: "flex", gap: "10px", justifyContent: "flex-end" }}>
          <button
            type="button"
            disabled={disabled}
            onClick={() => onReject(approval.approval_id)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: "6px",
              backgroundColor: "var(--bg-subtle)",
              color: "var(--status-danger)",
              border: "1px solid var(--status-danger)",
              padding: "6px 14px",
              borderRadius: "6px",
              cursor: disabled ? "not-allowed" : "pointer",
              fontSize: "13px",
            }}
          >
            <ActionCloseIcon size={14} />
            {t("common.cancel")}
          </button>

          <button
            type="button"
            disabled={disabled}
            onClick={() => onApprove(approval.approval_id)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: "6px",
              backgroundColor: "var(--action-primary)",
              color: "var(--action-on-primary)",
              border: "none",
              padding: "6px 16px",
              borderRadius: "6px",
              cursor: disabled ? "not-allowed" : "pointer",
              fontSize: "13px",
            }}
          >
            <ActionCheckIcon size={14} />
            {t("common.confirm")}
          </button>
        </div>
      ) : (
        <div style={{ fontSize: "12px", color: "var(--text-muted)", textAlign: "right" }}>
          {t("auth.forbidden")}
        </div>
      )}
    </div>
  );
}
