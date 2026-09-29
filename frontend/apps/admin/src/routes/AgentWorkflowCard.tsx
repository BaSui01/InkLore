import { Workflow, Power, Edit2, Trash2, Eye } from "lucide-react";
import { Card, Badge } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { AgentWorkflowView } from "../api/adminApi";

interface WorkflowCardProps {
  wf: AgentWorkflowView;
  onToggleEnabled: (wf: AgentWorkflowView) => void;
  onEdit: (wf: AgentWorkflowView) => void;
  onDelete: (wf: AgentWorkflowView) => void;
  onInspect: (wf: AgentWorkflowView) => void;
}

export function WorkflowCard({
  wf,
  onToggleEnabled,
  onEdit,
  onDelete,
  onInspect,
}: WorkflowCardProps) {
  const { t } = useTranslation();
  return (
    <Card
      key={wf.id}
      className="p-6 bg-[var(--bg-surface)] border-[var(--border-decorative)] space-y-5 hover:shadow-md transition-all flex flex-col justify-between"
    >
      <div className="space-y-4">
        {/* Header */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-3 min-w-0">
            <div className="p-2.5 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] shrink-0">
              <Workflow className="w-5 h-5 text-[var(--status-success)]" />
            </div>
            <div className="min-w-0">
              <div className="font-bold text-base text-[var(--text-primary)] truncate">
                {wf.title}
              </div>
              <div className="text-sm text-[var(--text-secondary)] font-medium">
                {t("admin.agents.workflow_configured")}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <Badge variant={wf.enabled ? "success" : "secondary"} size="md">
              {t("admin.agents.workflow_version_label", wf.version)}
            </Badge>
          </div>
        </div>

        {/* DAG Node Pipeline Visualizer */}
        <div className="space-y-2.5">
          <div className="text-sm font-bold text-[var(--text-secondary)]">
            <span>{t("admin.agents.topology_nodes", wf.manifest?.nodes?.length ?? 0)}</span>
          </div>

          <div className="p-4 rounded-xl bg-[var(--bg-paper)] border border-[var(--border-decorative)] space-y-2.5">
            {wf.manifest?.nodes?.map((node, nIdx) => (
              <div
                key={node.node_key || nIdx}
                className="flex items-center justify-between p-3 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)]/70 text-sm"
              >
                <div className="flex items-center gap-2.5 min-w-0">
                  <span className="w-6 h-6 rounded-md bg-[var(--bg-surface)] border border-[var(--border-decorative)] flex items-center justify-center text-xs font-bold text-[var(--text-secondary)] shrink-0">
                    {nIdx + 1}
                  </span>
                  <span className="text-sm font-bold text-[var(--text-primary)] truncate">
                    {t("admin.agents.node_number", nIdx + 1)}
                  </span>
                  <Badge variant="secondary" size="sm">
                    {t(["supervisor", "structure", "writer", "polisher", "reviewer", "tool"].includes(node.role) ? `admin.agents.node_role.${node.role}` : "admin.agents.node_role.other")}
                  </Badge>
                </div>
                <div className="flex items-center gap-2 text-xs text-[var(--text-secondary)] shrink-0">
                  {node.depends_on && node.depends_on.length > 0 && (
                    <span className="text-xs text-[var(--text-muted)] font-medium">
                      {t("admin.agents.dependency_label")}{" "}
                      {node.depends_on.map((key) => {
                        const index = wf.manifest.nodes.findIndex((item) => item.node_key === key);
                        return index < 0 ? t("admin.agents.node_role.other") : String(index + 1);
                      }).join(", ")}
                    </span>
                  )}
                  {node.required && (
                    <span className="px-2 py-0.5 rounded bg-[var(--status-danger)]/15 text-[var(--status-danger)] border border-[var(--status-danger)]/30 font-semibold">
                      {t("admin.agents.required_node")}
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="pt-4 border-t border-[var(--border-decorative)] flex items-center justify-end text-sm text-[var(--text-secondary)]">
        <div className="flex items-center gap-3">
          <button
            onClick={() => onToggleEnabled(wf)}
            className={`cursor-pointer font-medium flex items-center gap-1 ${
              wf.enabled ? "text-[var(--status-success)] hover:opacity-80" : "text-[var(--text-muted)] hover:text-[var(--text-primary)]"
            }`}
            title={t("admin.agents.toggle_workflow_status")}
          >
            <Power className="w-3.5 h-3.5" />
            <span>{t(wf.enabled ? "admin.agents.workflow_enabled" : "admin.agents.workflow_disabled")}</span>
          </button>
          <button
            onClick={() => onEdit(wf)}
            className="text-[var(--text-secondary)] hover:text-[var(--text-primary)] p-1 cursor-pointer"
            title={t("admin.agents.edit_workflow")}
          >
            <Edit2 className="w-4 h-4" />
          </button>
          <button
            onClick={() => onDelete(wf)}
            className="flex items-center gap-1 text-[var(--status-danger)] hover:opacity-80 p-1 cursor-pointer"
            title={t("admin.agents.delete_workflow")}
          >
            <Trash2 className="w-4 h-4" />
            <span>{t("common.delete")}</span>
          </button>
          <button
            onClick={() => onInspect(wf)}
            className="flex items-center gap-1.5 text-[var(--action-primary)] hover:opacity-80 font-sans cursor-pointer font-medium"
          >
            <Eye className="w-4 h-4" />
            <span>{t("admin.agents.inspect_workflow")}</span>
          </button>
        </div>
      </div>
    </Card>
  );
}
