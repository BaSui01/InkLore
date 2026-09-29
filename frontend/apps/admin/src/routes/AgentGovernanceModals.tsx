import { Cpu, Workflow, Copy, Plus, Check } from "lucide-react";
import { Modal, Button, Input, ErrorPresenter } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";

export interface StandardTaskKindOption {
  value: string;
}

// The input enum is fixed by the Agent contract; the displayed labels come from i18n.
export const STANDARD_TASK_KINDS: StandardTaskKindOption[] = [
  { value: "chapter_draft" },
  { value: "chapter_rewrite" },
  { value: "outline_suggest" },
  { value: "worldview_suggest" },
  { value: "inspiration_analyze" },
  { value: "compliance_audit" },
  { value: "compliance_rewrite" },
];

export interface InspectModalProps {
  open: boolean;
  onClose: () => void;
  inspectItem: { title: string; type: "definition" | "workflow"; data: any } | null;
  copiedId: string | null;
  onCopy: (text: string, id: string) => void;
}

export function AgentInspectModal({ open, onClose, inspectItem, copiedId, onCopy }: InspectModalProps) {
  const { t } = useTranslation();
  const definitionDisplayFields = ["id", "revision", "definition_key", "title", "enabled", "task_kinds", "tool_version_ids", "current_version_id", "version", "publication_status", "system_instruction", "created_at", "updated_at"];
  const safeData = inspectItem?.type === "definition" && inspectItem.data
    ? Object.fromEntries(definitionDisplayFields.filter((key) => key in inspectItem.data).map((key) => [key, inspectItem.data[key]]))
    : inspectItem?.data;
  return (
    <Modal
      open={open}
      onClose={onClose}
      maxWidth="2xl"
      title={<div className="flex items-center gap-2.5">
        {inspectItem?.type === "definition" && <Cpu className="w-5 h-5 text-[var(--action-primary)]" />}
        {inspectItem?.type === "workflow" && <Workflow className="w-5 h-5 text-[var(--status-success)]" />}
        <span>{inspectItem?.title || t("admin.agents.contract_details")}</span>
      </div>}
      footer={<div className="flex items-center justify-between w-full">
        <Button variant="outline" size="md" icon={<Copy className="w-4 h-4" />} onClick={() => safeData && onCopy(JSON.stringify(safeData, null, 2), "modal-copied")}>
          {copiedId === "modal-copied" ? t("admin.agents.copied") : t("admin.agents.copy_config")}
        </Button>
        <Button variant="primary" size="md" onClick={onClose}>{t("common.close")}</Button>
      </div>}
    >
      <div className="space-y-4 text-sm">
        {inspectItem?.type === "definition" && inspectItem.data?.system_instruction && (
          <div className="space-y-1.5">
            <span className="font-semibold text-xs text-[var(--text-secondary)]">{t("admin.agents.instruction_admin_only")}</span>
            <div className="p-3.5 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] text-xs text-[var(--text-primary)] leading-relaxed whitespace-pre-wrap max-h-48 overflow-y-auto">
              {inspectItem.data.system_instruction}
            </div>
          </div>
        )}
        <p className="text-[var(--text-secondary)] text-xs">{t("admin.agents.safe_projection_notice")}</p>
        <pre className="p-4 rounded-xl bg-[var(--bg-paper)] border border-[var(--border-decorative)] text-[var(--text-primary)] overflow-x-auto text-xs font-mono leading-relaxed max-h-80">
          {JSON.stringify(safeData, null, 2)}
        </pre>
      </div>
    </Modal>
  );
}

export interface EditModalProps {
  open: boolean;
  onClose: () => void;
  activeTab: "definitions" | "workflows";
  editingItem: any | null;
  submitting: boolean;
  modalFormError: unknown;
  formTitle: string;
  setFormTitle: (v: string) => void;
  formKey: string;
  setFormKey: (v: string) => void;
  formEnabled: boolean;
  setFormEnabled: (v: boolean) => void;
  formInstruction: string;
  setFormInstruction: (v: string) => void;
  formTaskKinds: string[];
  setFormTaskKinds: (v: string[]) => void;
  formWorkflowManifest: string;
  setFormWorkflowManifest: (v: string) => void;
  onSave: () => void;
}

const editorClass = "w-full rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-paper)] p-3 text-sm text-[var(--text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)] leading-relaxed font-mono";

export function AgentEditModal(props: EditModalProps) {
  const { t } = useTranslation();
  const {
    open,
    onClose,
    activeTab,
    editingItem,
    submitting,
    modalFormError,
    formTitle,
    setFormTitle,
    formKey,
    setFormKey,
    formEnabled,
    setFormEnabled,
    formInstruction,
    setFormInstruction,
    formTaskKinds,
    setFormTaskKinds,
    formWorkflowManifest,
    setFormWorkflowManifest,
    onSave,
  } = props;

  let previewNodes: Array<{ node_key?: string; role?: string; depends_on?: string[] }> = [];
  if (activeTab === "workflows") {
    try {
      const parsed = JSON.parse(formWorkflowManifest);
      if (Array.isArray(parsed?.nodes)) previewNodes = parsed.nodes;
    } catch { /* JSON parse feedback is presented on save; do not replace server DAG validation. */ }
  }

  const titleKey = activeTab === "definitions" ? "admin.agents.tab_definitions" : "admin.agents.tab_workflows";

  const toggleTaskKind = (val: string) => {
    if (formTaskKinds.includes(val)) {
      setFormTaskKinds(formTaskKinds.filter((k) => k !== val));
    } else {
      if (formTaskKinds.length >= 7) return;
      setFormTaskKinds([...formTaskKinds, val]);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      maxWidth="2xl"
      title={<div className="flex items-center gap-2.5"><Plus className="w-5 h-5 text-[var(--action-primary)]" /><span>{t(editingItem ? "admin.agents.edit" : "admin.agents.create")} {t(titleKey)}</span></div>}
      footer={<div className="flex items-center justify-end gap-3 w-full"><Button variant="outline" size="md" onClick={onClose}>{t("common.cancel")}</Button><Button variant="primary" size="md" loading={submitting} onClick={onSave}>{submitting ? t("admin.agents.saving") : t("common.save")}</Button></div>}
    >
      <div className="space-y-4 py-2 text-sm">
        {modalFormError != null && (typeof modalFormError === "string" ? <p role="alert" className="text-sm text-[var(--status-danger)] p-2.5 rounded-lg bg-[var(--status-danger)]/10 border border-[var(--status-danger)]/30 font-medium">{modalFormError}</p> : <ErrorPresenter error={modalFormError} variant="inline" />)}

        <label className="block font-semibold text-[var(--text-secondary)]">
          {t("admin.agents.title_label")}
          <Input value={formTitle} onChange={(e) => setFormTitle(e.target.value)} placeholder={t("admin.agents.title_placeholder")} />
        </label>
        <label className="block font-semibold text-[var(--text-secondary)]">
          {t("admin.agents.key_label")}
          <Input value={formKey} onChange={(e) => setFormKey(e.target.value)} disabled={!!editingItem} placeholder={t("admin.agents.key_placeholder")} />
        </label>

        {activeTab === "definitions" && (
          <>
            <p className="text-[var(--text-secondary)] text-xs">{t("admin.agents.future_runs_only")}</p>

            {/* 任务类型选择器 (支持多选 1~7 项，清晰展示合法创作类型) */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="font-semibold text-[var(--text-secondary)]">{t("admin.agents.task_kinds_select")}</label>
                <span className="text-xs text-[var(--text-muted)]">{t("admin.agents.selected_task_kinds", formTaskKinds.length, STANDARD_TASK_KINDS.length)}</span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {STANDARD_TASK_KINDS.map((opt) => {
                  const checked = formTaskKinds.includes(opt.value);
                  return (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => toggleTaskKind(opt.value)}
                      className={`flex items-start gap-2.5 p-2.5 rounded-xl border text-left transition-all cursor-pointer ${
                        checked
                          ? "border-[var(--action-primary)] bg-[var(--action-primary)]/10 text-[var(--text-primary)]"
                          : "border-[var(--border-decorative)] bg-[var(--bg-paper)] text-[var(--text-secondary)] hover:border-[var(--border-control)]"
                      }`}
                    >
                      <div className={`w-4 h-4 rounded mt-0.5 flex items-center justify-center shrink-0 border ${
                        checked ? "bg-[var(--action-primary)] border-[var(--action-primary)] text-white" : "border-[var(--border-control)] bg-[var(--bg-canvas)]"
                      }`}>
                        {checked && <Check className="w-3 h-3 stroke-[3]" />}
                      </div>
                      <div className="min-w-0 flex-1">
                        <span className="font-semibold text-sm text-[var(--text-primary)]">{t(`admin.agents.task_kind.${opt.value}`)}</span>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>

            <p className="text-xs text-[var(--text-muted)]">{t("admin.agents.definition_tool_binding_readonly")}</p>

            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <label className="font-semibold text-[var(--text-secondary)]">{t("admin.agents.instruction_admin_only")}</label>
                <span className={`text-xs font-mono ${formInstruction.length > 8000 ? "text-[var(--status-danger)] font-bold" : "text-[var(--text-muted)]"}`}>
                  {t("admin.agents.instruction_length", formInstruction.length, 8000)}
                </span>
              </div>
              <textarea
                value={formInstruction}
                onChange={(e) => setFormInstruction(e.target.value)}
                rows={6}
                placeholder={t("admin.agents.instruction_placeholder")}
                className={editorClass}
              />
            </div>
          </>
        )}

        {activeTab === "workflows" && (
          <>
            <p className="text-[var(--text-secondary)]">{t("admin.agents.future_runs_only")}</p>
            <label className="block font-semibold text-[var(--text-secondary)]">
              {t("admin.agents.workflow_manifest_json")}
              <textarea value={formWorkflowManifest} onChange={(e) => setFormWorkflowManifest(e.target.value)} rows={14} spellCheck={false} className={editorClass} />
            </label>
            <div className="rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-subtle)] p-3 space-y-2">
              <strong>{t("admin.agents.workflow_preview")}</strong>
              {previewNodes.length === 0 ? (
                <p className="text-[var(--text-secondary)]">{t("admin.agents.no_preview_nodes")}</p>
              ) : (
                previewNodes.map((node, index) => (
                  <div key={`${node.node_key ?? "node"}-${index}`} className="text-xs font-mono">
                    <span>{node.node_key ?? "?"} · {node.role ?? "?"} ← {(node.depends_on ?? []).join(", ") || "—"}</span>
                  </div>
                ))
              )}
            </div>
            <p className="text-[var(--text-secondary)]">{t("admin.agents.server_dag_validation")}</p>
          </>
        )}

        <label className="flex items-center gap-2 pt-1 cursor-pointer select-none">
          <input type="checkbox" checked={formEnabled} onChange={(e) => setFormEnabled(e.target.checked)} />
          <span className="font-semibold text-[var(--text-primary)]">{t("admin.agents.enabled")}</span>
        </label>
      </div>
    </Modal>
  );
}
