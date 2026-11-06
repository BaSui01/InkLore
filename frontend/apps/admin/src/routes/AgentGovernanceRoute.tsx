/**
 * @inklore/admin-console - 7 业务域智能体矩阵 (TAO 状态机治理)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 杜绝小字（全界面最小字体规范），前端 CRUD + 分页一次性通用复用
 * 数据 100% 来源于后端真实 REST API
 */

import { useState, useEffect, useCallback, useMemo } from "react";
import {
  Cpu,
  Workflow,
  RefreshCw,
  Wrench,
  Search,
  Eye,
  Plus,
  Edit2,
  Trash2,
  Power,
} from "lucide-react";
import { Card, Badge, Button, Input, ConfirmDialog } from "@inklore/ui";
import { safeErrorText } from "@inklore/transport";
import { AdminHeader } from "../components/AdminHeader";
import { AdminPagination } from "../components/AdminPagination";
import { AdminNotice } from "../components/AdminNotice";
import { useTranslation } from "@inklore/i18n";
import { useAdminSession } from "../context/AdminSessionContext";
import { AgentInspectModal, AgentEditModal, STANDARD_TASK_KINDS } from "./AgentGovernanceModals";
import { WorkflowCard } from "./AgentWorkflowCard";
import { AgentTaskPolicies } from "./AgentTaskPolicies";
import { AgentRunAudit } from "./AgentRunAudit";
import { AgentToolAvailability } from "./AgentToolAvailability";
import {
  AgentDefinitionView,
  AgentWorkflowView,
  AgentBuiltinToolView,
  fetchAgentDefinitions,
  fetchAgentWorkflows,
  fetchAgentBuiltinTools,
  createAgentDefinition,
  updateAgentDefinition,
  deleteAgentDefinition,
  createAgentWorkflow,
  updateAgentWorkflow,
  deleteAgentWorkflow,
} from "../api/adminApi";

type ActiveTab = "definitions" | "workflows" | "tools" | "audit" | "policies";

export function AgentGovernanceRoute() {
  const { t } = useTranslation();
  const { isAuthenticated } = useAdminSession();

  const [activeTab, setActiveTab] = useState<ActiveTab>("definitions");
  const [loading, setLoading] = useState(true);
  const [loadedOnce, setLoadedOnce] = useState(false);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [toolLoading, setToolLoading] = useState(true);
  const [toolLoadedOnce, setToolLoadedOnce] = useState(false);
  const [toolLoadError, setToolLoadError] = useState<unknown>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [definitions, setDefinitions] = useState<AgentDefinitionView[]>([]);
  const [workflows, setWorkflows] = useState<AgentWorkflowView[]>([]);
  const [tools, setTools] = useState<AgentBuiltinToolView[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Pagination state (reusable for each tab)
  const [defPage, setDefPage] = useState(1);
  const [defPageSize, setDefPageSize] = useState(6);

  const [wfPage, setWfPage] = useState(1);
  const [wfPageSize, setWfPageSize] = useState(6);

  // Inspector Modal State
  const [inspectModalOpen, setInspectModalOpen] = useState(false);
  const [inspectItem, setInspectItem] = useState<{
    title: string;
    type: "definition" | "workflow";
    data: any;
  } | null>(null);

  // Create / Edit Modal State
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<any | null>(null);
  const [modalFormError, setModalFormError] = useState<unknown>(null);

  // Form Fields
  const [formKey, setFormKey] = useState("");
  const [formTitle, setFormTitle] = useState("");
  const [formEnabled, setFormEnabled] = useState(false);
  const [formInstruction, setFormInstruction] = useState("");
  const [formTaskKinds, setFormTaskKinds] = useState<string[]>([]);
  const [formToolVersionIds, setFormToolVersionIds] = useState<string[]>([]);
  const [formWorkflowManifest, setFormWorkflowManifest] = useState("");

  const loadData = useCallback(async () => {
    setLoading(true);
    setToolLoading(true);
    await Promise.all([
      (async () => {
        try {
          const [defs, wfs] = await Promise.all([fetchAgentDefinitions(), fetchAgentWorkflows()]);
          setDefinitions(defs);
          setWorkflows(wfs);
          setLoadedOnce(true);
          setLoadError(null);
        } catch (err) {
          setLoadError(err);
        } finally {
          setLoading(false);
        }
      })(),
      (async () => {
        try {
          const builtinTools = await fetchAgentBuiltinTools();
          setTools(builtinTools);
          setToolLoadedOnce(true);
          setToolLoadError(null);
        } catch (err) {
          setToolLoadError(err);
          setToolLoadedOnce(false);
          setTools([]);
        } finally {
          setToolLoading(false);
        }
      })(),
    ]);
  }, []);

  useEffect(() => {
    if (isAuthenticated) {
      loadData();
    }
  }, [loadData, isAuthenticated]);

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  // Filtered lists
  const filteredDefinitions = useMemo(() => {
    if (!searchQuery.trim()) return definitions;
    const q = searchQuery.toLowerCase();
    return definitions.filter(
      (d) =>
        d.title.toLowerCase().includes(q) ||
        d.definition_key.toLowerCase().includes(q) ||
        d.task_kinds.some((k) => k.toLowerCase().includes(q))
    );
  }, [definitions, searchQuery]);

  const filteredWorkflows = useMemo(() => {
    if (!searchQuery.trim()) return workflows;
    const q = searchQuery.toLowerCase();
    return workflows.filter(
      (w) =>
        w.title.toLowerCase().includes(q) ||
        w.workflow_key.toLowerCase().includes(q)
    );
  }, [workflows, searchQuery]);

  // Paginated items
  const paginatedDefinitions = useMemo(() => {
    const start = (defPage - 1) * defPageSize;
    return filteredDefinitions.slice(start, start + defPageSize);
  }, [filteredDefinitions, defPage, defPageSize]);

  const paginatedWorkflows = useMemo(() => {
    const start = (wfPage - 1) * wfPageSize;
    return filteredWorkflows.slice(start, start + wfPageSize);
  }, [filteredWorkflows, wfPage, wfPageSize]);

  // --- CRUD Handlers ---

  const handleOpenCreateModal = () => {
    if (activeTab === "tools" || activeTab === "audit" || activeTab === "policies") return;
    setEditingItem(null);
    setModalFormError(null);
    setFormTitle("");
    setFormEnabled(false);

    if (activeTab === "definitions") {
      setFormKey("");
      setFormInstruction("");
      setFormTaskKinds(["outline_suggest"]);
      setFormToolVersionIds([]);
    } else if (activeTab === "workflows") {
      setFormKey("");
      setFormWorkflowManifest(JSON.stringify({ nodes: [] }, null, 2));
    }
    setCreateModalOpen(true);
  };

  const handleOpenEditModal = (item: any) => {
    if (activeTab === "tools" || activeTab === "audit" || activeTab === "policies") return;
    setEditingItem(item);
    setModalFormError(null);
    setFormTitle(item.title);
    setFormEnabled(item.enabled);

    if (activeTab === "definitions") {
      setFormKey(item.definition_key);
      setFormInstruction(item.system_instruction || "");
      setFormTaskKinds(item.task_kinds || []);
      setFormToolVersionIds(item.tool_version_ids || []);
    } else if (activeTab === "workflows") {
      setFormKey(item.workflow_key);
      setFormWorkflowManifest(JSON.stringify(item.manifest ?? { nodes: [] }, null, 2));
    }
    setCreateModalOpen(true);
  };

  const handleToggleEnabled = async (item: any, type: ActiveTab) => {
    try {
      setActionError(null);
      setActionSuccess(null);
      if (type === "definitions") {
        await updateAgentDefinition(item.id, {
          definition_key: item.definition_key,
          title: item.title,
          enabled: !item.enabled,
          system_instruction: item.system_instruction,
          tool_version_ids: item.tool_version_ids || [],
          task_kinds: item.task_kinds || [],
          expected_revision: item.revision,
        });
      } else if (type === "workflows") {
        await updateAgentWorkflow(item.id, {
          workflow_key: item.workflow_key,
          title: item.title,
          enabled: !item.enabled,
          manifest: item.manifest,
          expected_revision: item.revision,
        });
      }
      await loadData();
      setActionSuccess(t("common.success"));
    } catch (err: any) {
      setActionError(err);
    }
  };

  // Delete confirm state
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [itemToDelete, setItemToDelete] = useState<{ item: any; type: ActiveTab } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const handleDeleteItem = (item: any, type: ActiveTab) => {
    setItemToDelete({ item, type });
    setDeleteError(null);
    setDeleteConfirmOpen(true);
  };

  const executeDelete = async () => {
    if (!itemToDelete) return;
    setDeleting(true);
    setActionError(null);
    setActionSuccess(null);
    setDeleteError(null);
    try {
      const { item, type } = itemToDelete;
      if (type === "definitions") {
        await deleteAgentDefinition(item.id, item.revision);
      } else if (type === "workflows") {
        await deleteAgentWorkflow(item.id, item.revision);
      }
      setDeleteConfirmOpen(false);
      setItemToDelete(null);
      setDeleteError(null);
      await loadData();
      setActionSuccess(t("common.success"));
    } catch (err: any) {
      setDeleteError(safeErrorText(err, t));
      setActionError(err);
    } finally {
      setDeleting(false);
    }
  };

  const handleSaveModal = async () => {
    if (!formTitle.trim() || !formKey.trim()) {
      setModalFormError(t("admin.agents.required_fields"));
      return;
    }

    setSubmitting(true);
    setModalFormError(null);
    setActionError(null);
    setActionSuccess(null);

    try {
      if (activeTab === "definitions") {
        const kinds = formTaskKinds.map((k) => k.trim()).filter(Boolean);
        if (kinds.length === 0) {
          setModalFormError(t("admin.agents.task_kinds_required"));
          return;
        }
        if (!formInstruction.trim()) {
          setModalFormError(t("admin.agents.instruction_required"));
          return;
        }
        if (formInstruction.trim().length > 8000) {
          setModalFormError(t("admin.agents.instruction_required"));
          return;
        }
        if (!editingItem && !/^[a-z][a-z0-9_]*$/.test(formKey.trim())) {
          setModalFormError(t("admin.agents.key_format_invalid"));
          return;
        }
        if (editingItem) {
          await updateAgentDefinition(editingItem.id, {
            definition_key: formKey.trim(),
            title: formTitle.trim(),
            enabled: formEnabled,
            system_instruction: formInstruction.trim(),
            tool_version_ids: formToolVersionIds,
            task_kinds: kinds,
            expected_revision: editingItem.revision,
          });
        } else {
          await createAgentDefinition({
            definition_key: formKey.trim(),
            title: formTitle.trim(),
            enabled: formEnabled,
            system_instruction: formInstruction.trim(),
            tool_version_ids: formToolVersionIds,
            task_kinds: kinds,
          });
        }
      } else if (activeTab === "workflows") {
        let manifest: AgentWorkflowView["manifest"];
        try {
          manifest = JSON.parse(formWorkflowManifest);
        } catch {
          throw new Error(t("admin.agents.invalid_manifest_json"));
        }
        if (!manifest || !Array.isArray(manifest.nodes)) {
          throw new Error(t("admin.agents.manifest_nodes_required"));
        }
        const payload = {
          workflow_key: formKey.trim(),
          title: formTitle.trim(),
          enabled: formEnabled,
          manifest,
        };
        if (editingItem) {
          await updateAgentWorkflow(editingItem.id, { ...payload, expected_revision: editingItem.revision });
        } else {
          await createAgentWorkflow(payload);
        }
      }
      setCreateModalOpen(false);
      await loadData();
      setActionSuccess(t("common.success"));
    } catch (err: unknown) {
      const localValidationMessages = [
        t("admin.agents.invalid_manifest_json"),
        t("admin.agents.manifest_nodes_required"),
      ];
      if (err instanceof Error && localValidationMessages.includes(err.message)) {
        setModalFormError(err.message);
      } else {
        setModalFormError(safeErrorText(err, t));
        setActionError(err);
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminNotice
        error={actionError ?? (activeTab === "tools" ? toolLoadError : loadError)}
        success={actionSuccess}
        onRetry={actionError == null && (activeTab === "tools" ? toolLoadError : loadError) != null ? () => void loadData() : undefined}
        onDismiss={() => { setActionError(null); setLoadError(null); setToolLoadError(null); setActionSuccess(null); }}
      />
      <AdminHeader
        title={t("admin.sidebar.nav_agents")}
        actions={
          <div className="flex items-center gap-2 flex-wrap">
            <Button
              size="sm"
              variant="outline"
              icon={<RefreshCw className={`w-3.5 h-3.5 ${loading || toolLoading ? "animate-spin" : ""}`} />}
              onClick={loadData}
              title={t("common.refresh")}
            >
              {t("common.refresh")}
            </Button>
            {(activeTab === "definitions" || activeTab === "workflows") && <Button
              size="sm"
              variant="primary"
              icon={<Plus className="w-3.5 h-3.5" />}
              disabled={!loadedOnce || loadError != null}
              onClick={handleOpenCreateModal}
            >
              {t(activeTab === "definitions" ? "admin.agents.new_definition" : "admin.agents.new_workflow")}
            </Button>}
          </div>
        }
        className="px-6 py-3.5 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)] shrink-0"
      />

      {/* Tabs navigation & Search Toolbar */}
      <div className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 flex flex-col min-[900px]:flex-row items-start min-[900px]:items-center justify-between gap-3">
        {/* Tab Buttons */}
        <div className="flex flex-wrap items-center gap-2 p-1.5 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)]">
          <button
            onClick={() => { setActiveTab("definitions"); setSearchQuery(""); }}
            className={`flex shrink-0 items-center gap-2.5 px-4 py-2 rounded-lg text-sm font-semibold transition-all cursor-pointer ${
              activeTab === "definitions"
                ? "bg-[var(--bg-surface)] text-[var(--action-primary)] shadow-sm"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            <Cpu className="w-4 h-4" />
            <span>{t("admin.agents.tab_definitions")}</span>
            <span className="ml-1.5 px-2 py-0.5 rounded-full text-xs font-mono font-bold bg-[var(--bg-subtle)] text-[var(--text-primary)] border border-[var(--border-decorative)]">
              {loadedOnce ? definitions.length : "—"}
            </span>
          </button>

          <button
            onClick={() => { setActiveTab("workflows"); setSearchQuery(""); }}
            className={`flex shrink-0 items-center gap-2.5 px-4 py-2 rounded-lg text-sm font-semibold transition-all cursor-pointer ${
              activeTab === "workflows"
                ? "bg-[var(--bg-surface)] text-[var(--action-primary)] shadow-sm"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            <Workflow className="w-4 h-4" />
            <span>{t("admin.agents.tab_workflows")}</span>
            <span className="ml-1.5 px-2 py-0.5 rounded-full text-xs font-mono font-bold bg-[var(--bg-subtle)] text-[var(--text-primary)] border border-[var(--border-decorative)]">
              {loadedOnce ? workflows.length : "—"}
            </span>
          </button>

          <button
            onClick={() => { setActiveTab("tools"); setSearchQuery(""); }}
            className={`flex shrink-0 items-center gap-2.5 px-4 py-2 rounded-lg text-sm font-semibold transition-all cursor-pointer ${
              activeTab === "tools"
                ? "bg-[var(--bg-surface)] text-[var(--action-primary)] shadow-sm"
                : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            }`}
          >
            <Wrench className="w-4 h-4" />
            <span>{t("admin.agents.tab_tools")}</span>
          </button>
          <button onClick={() => { setActiveTab("policies"); setSearchQuery(""); }} className={`px-4 py-2 rounded-lg text-sm font-semibold ${activeTab === "policies" ? "bg-[var(--bg-surface)] text-[var(--action-primary)]" : "text-[var(--text-secondary)]"}`}>{t("agentPolicy.title")}</button>
          <button onClick={() => { setActiveTab("audit"); setSearchQuery(""); }} className={`px-4 py-2 rounded-lg text-sm font-semibold ${activeTab === "audit" ? "bg-[var(--bg-surface)] text-[var(--action-primary)]" : "text-[var(--text-secondary)]"}`}>{t("agentAudit.title")}</button>
        </div>

        {/* Search Input */}
        {activeTab !== "audit" && activeTab !== "policies" && <div className="w-full min-[900px]:w-80">
          <Input
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t("admin.agents.search_placeholder")}
            prefixIcon={<Search className="w-4 h-4" />}
          />
        </div>}
      </div>

      {activeTab === "audit" && <AgentRunAudit />}
      <div className={activeTab === "policies" ? "flex-1 min-h-0 flex flex-col" : "hidden"} aria-hidden={activeTab !== "policies"}><AgentTaskPolicies /></div>

      {/* Tab 1: Agent Definitions */}
      {activeTab === "definitions" && (
        <div className="flex-1 min-h-0 flex flex-col">
          <div className="flex-1 min-h-0 overflow-auto p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {paginatedDefinitions.length === 0 ? (
              <div className="col-span-full p-16 text-center bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-2xl space-y-4">
                <div className="w-14 h-14 mx-auto rounded-2xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-[var(--action-primary)]">
                  <Cpu className="w-7 h-7" />
                </div>
                <div className="space-y-1.5">
                  <h3 className="text-base font-bold text-[var(--text-primary)]">
{loading ? t("common.loading") : !loadedOnce ? t("admin.agents.load_unavailable") : searchQuery ? t("admin.agents.search_empty") : t("admin.agents.empty_definitions")}
                  </h3>
                  <p className="text-sm text-[var(--text-secondary)] max-w-md mx-auto leading-relaxed">
                    {!loadedOnce ? t("admin.agents.load_retry_hint") : searchQuery ? t("admin.agents.search_hint") : t("admin.agents.empty_definition_hint")}
                  </p>
                </div>
                <div>
                  {searchQuery ? (
                    <Button size="md" variant="outline" onClick={() => setSearchQuery("")}>
                      {t("admin.agents.clear_search")}
                    </Button>
                  ) : (
                    <Button
                      size="md"
                      variant="primary"
                      icon={<Plus className="w-4 h-4" />}
                      disabled={!loadedOnce || loadError != null}
                      onClick={handleOpenCreateModal}
                    >
                      {t("admin.agents.create_first_definition")}
                    </Button>
                  )}
                </div>
              </div>
            ) : (
              paginatedDefinitions.map((def) => (
                  <Card
                    key={def.id}
                    className="p-5 bg-[var(--bg-surface)] border-[var(--border-decorative)] rounded-2xl hover:shadow-md transition-all flex flex-col justify-between gap-4"
                  >
                    <div className="space-y-3.5">
                      {/* Header */}
                      <div className="flex items-start gap-3">
                        <div className="p-2.5 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] shrink-0 mt-0.5">
                          <Cpu className="w-5 h-5 text-[var(--action-primary)]" />
                        </div>
                        <div className="min-w-0 flex-1 space-y-1.5">
                          <div className="font-bold text-base text-[var(--text-primary)] leading-snug truncate" title={def.title}>
                            {def.title}
                          </div>
                          <div className="flex items-center justify-end gap-1.5 text-xs">
                            <div className="flex items-center gap-1.5 shrink-0">
                              <Badge variant={def.publication_status === "published" ? "success" : "secondary"} size="sm">
                                {def.publication_status === "published"
                                  ? t("admin.agents.def_status_published")
                                  : t("admin.agents.def_status_draft")}
                              </Badge>
                              <Badge variant="default" size="sm">
                                {t("admin.agents.version_label", def.version)}
                              </Badge>
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* Task kinds are display labels here; machine keys remain available in the config inspector. */}
                      <div className="flex flex-wrap gap-1.5">
                        {def.task_kinds.map((taskKind) => {
                          const known = STANDARD_TASK_KINDS.some((item) => item.value === taskKind);
                          return (
                            <span
                              key={taskKind}
                              className="px-2 py-0.5 rounded-md text-xs font-medium bg-[var(--bg-subtle)] text-[var(--text-primary)] border border-[var(--border-decorative)]"
                            >
                              {t(known ? `admin.agents.task_kind.${taskKind}` : "admin.agents.unknown_task_kind")}
                            </span>
                          );
                        })}
                      </div>

                      {/* System Instruction Preview */}
                      {def.system_instruction && (
                        <p className="text-xs text-[var(--text-secondary)] line-clamp-2 bg-[var(--bg-subtle)]/50 p-2.5 rounded-xl border border-[var(--border-decorative)]/60 leading-relaxed font-sans" title={def.system_instruction}>
                          {def.system_instruction}
                        </p>
                      )}
                    </div>

                    {/* Card Actions & Meta */}
                    <div className="pt-3 border-t border-[var(--border-decorative)] space-y-2.5">
                      {/* Meta line: bound tool versions & power toggle */}
                      <div className="flex items-center justify-between text-xs text-[var(--text-secondary)]">
                        <div className="flex items-center gap-1">
                          <span>{t("admin.agents.def_tools")}:</span>
                          <span className="font-semibold text-[var(--text-primary)]">
                            {def.tool_version_ids?.length
                              ? t("admin.agents.bound_tool_versions", def.tool_version_ids.length)
                              : t("admin.agents.no_tools_bound")}
                          </span>
                        </div>
                        <button
                          onClick={() => handleToggleEnabled(def, "definitions")}
                          className={`cursor-pointer font-medium flex items-center gap-1.5 px-2.5 py-1 rounded-lg transition-colors border text-xs ${
                            def.enabled
                              ? "bg-[var(--status-success)]/10 text-[var(--status-success)] border-[var(--status-success)]/30 hover:bg-[var(--status-success)]/20"
                              : "bg-[var(--bg-subtle)] text-[var(--text-muted)] border-[var(--border-control)] hover:text-[var(--text-primary)]"
                          }`}
                          title={t("admin.agents.toggle_status")}
                        >
                          <Power className="w-3 h-3" />
                          <span>{t(def.enabled ? "admin.agents.enabled_status" : "admin.agents.disabled_status")}</span>
                        </button>
                      </div>

                      {/* Action buttons toolbar */}
                      <div className="flex items-center justify-between gap-2 pt-1 border-t border-[var(--border-decorative)]/40">
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            className="whitespace-nowrap shrink-0"
                            icon={<Edit2 className="w-3.5 h-3.5" />}
                            onClick={() => handleOpenEditModal(def)}
                          >
                            {t("admin.agents.edit")}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            className="whitespace-nowrap shrink-0 text-[var(--status-danger)] hover:bg-[var(--status-danger)]/10 border-[var(--border-decorative)] hover:border-[var(--status-danger)]/30"
                            icon={<Trash2 className="w-3.5 h-3.5" />}
                            onClick={() => handleDeleteItem(def, "definitions")}
                          >
                            {t("common.delete")}
                          </Button>
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="whitespace-nowrap shrink-0 text-[var(--action-primary)] hover:bg-[var(--action-primary)]/10"
                          icon={<Eye className="w-3.5 h-3.5 text-[var(--action-primary)]" />}
                          onClick={() => {
                            setInspectItem({
                              title: def.title,
                              type: "definition",
                              data: def,
                            });
                            setInspectModalOpen(true);
                          }}
                        >
                          {t("common.details")}
                        </Button>
                      </div>
                    </div>
                  </Card>
              ))
            )}
            </div>
          </div>

          {/* 统一底部分页组件 */}
          <AdminPagination
            total={filteredDefinitions.length}
            page={defPage}
            pageSize={defPageSize}
            onPageChange={setDefPage}
            onPageSizeChange={(size) => { setDefPageSize(size); setDefPage(1); }}
            pageSizeOptions={[6, 12, 24, 48]}
            className="px-6 py-2 bg-[var(--bg-surface)] border-t border-[var(--border-decorative)] shrink-0"
          />
        </div>
      )}

      {/* Tab 2: Workflows */}
      {activeTab === "workflows" && (
        <div className="flex-1 min-h-0 flex flex-col">
          <div className="flex-1 min-h-0 overflow-auto p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {paginatedWorkflows.length === 0 ? (
              <div className="col-span-full p-16 text-center bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-2xl space-y-4">
                <div className="w-14 h-14 mx-auto rounded-2xl bg-[var(--bg-subtle)] border border-[var(--border-decorative)] flex items-center justify-center text-[var(--status-success)]">
                  <Workflow className="w-7 h-7" />
                </div>
                <div className="space-y-1.5">
                  <h3 className="text-base font-bold text-[var(--text-primary)]">
{loading ? t("common.loading") : !loadedOnce ? t("admin.agents.load_unavailable") : searchQuery ? t("admin.agents.search_empty") : t("admin.agents.empty_workflows")}
                  </h3>
                  <p className="text-sm text-[var(--text-secondary)] max-w-md mx-auto leading-relaxed">
                    {!loadedOnce ? t("admin.agents.load_retry_hint") : searchQuery ? t("admin.agents.search_hint") : t("admin.agents.empty_workflow_hint")}
                  </p>
                </div>
                <div>
                  {searchQuery ? (
                    <Button size="md" variant="outline" onClick={() => setSearchQuery("")}>
                      {t("admin.agents.clear_search")}
                    </Button>
                  ) : (
                    <Button
                      size="md"
                      variant="primary"
                      icon={<Plus className="w-4 h-4" />}
                      disabled={!loadedOnce || loadError != null}
                      onClick={handleOpenCreateModal}
                    >
                      {t("admin.agents.create_first_workflow")}
                    </Button>
                  )}
                </div>
              </div>
            ) : (
              paginatedWorkflows.map((wf) => (
                <WorkflowCard
                  key={wf.id}
                  wf={wf}
                  onToggleEnabled={(item) => handleToggleEnabled(item, "workflows")}
                  onEdit={handleOpenEditModal}
                  onDelete={(item) => handleDeleteItem(item, "workflows")}
                  onInspect={(item) => {
                    setInspectItem({
                      title: item.title,
                      type: "workflow",
                      data: item,
                    });
                    setInspectModalOpen(true);
                  }}
                />
              ))
            )}
            </div>
          </div>

          {/* 统一底部分页组件 */}
          <AdminPagination
            total={filteredWorkflows.length}
            page={wfPage}
            pageSize={wfPageSize}
            onPageChange={setWfPage}
            onPageSizeChange={(size) => { setWfPageSize(size); setWfPage(1); }}
            pageSizeOptions={[6, 12, 24, 48]}
            className="px-6 py-2 bg-[var(--bg-surface)] border-t border-[var(--border-decorative)] shrink-0"
          />
        </div>
      )}

      {/* The server handler registry is the only source for the read-only tool catalog. */}
      {activeTab === "tools" && (
        <AgentToolAvailability tools={tools} loadedOnce={toolLoadedOnce} loading={toolLoading} searchQuery={searchQuery} />
      )}

      {/* Detail Contract Inspector Modal */}
      <AgentInspectModal
        open={inspectModalOpen}
        onClose={() => setInspectModalOpen(false)}
        inspectItem={inspectItem}
        copiedId={copiedId}
        onCopy={copyToClipboard}
      />

      {/* Create / Edit Definition or Workflow Modal */}
      {(activeTab === "definitions" || activeTab === "workflows") && <AgentEditModal
        open={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        activeTab={activeTab}
        editingItem={editingItem}
        submitting={submitting}
        modalFormError={modalFormError}
        formTitle={formTitle}
        setFormTitle={setFormTitle}
        formKey={formKey}
        setFormKey={setFormKey}
        formEnabled={formEnabled}
        setFormEnabled={setFormEnabled}
        formInstruction={formInstruction}
        setFormInstruction={setFormInstruction}
        formTaskKinds={formTaskKinds}
        setFormTaskKinds={setFormTaskKinds}
        formWorkflowManifest={formWorkflowManifest}
        setFormWorkflowManifest={setFormWorkflowManifest}
        onSave={handleSaveModal}
      />}

      {/* Reusable Centered Custom Confirm Dialog */}
      <ConfirmDialog
        open={deleteConfirmOpen}
        onClose={() => { if (!deleting) { setDeleteConfirmOpen(false); setDeleteError(null); } }}
        onConfirm={executeDelete}
        loading={deleting}
        title={t("admin.agents.confirm_delete_title", itemToDelete?.item?.title ?? "")}
        description={
          <div className="space-y-2">
            <p>{t("admin.agents.confirm_archive_desc")}</p>
            <p>{t("admin.agents.confirm_revision", itemToDelete?.item?.revision ?? "—")}</p>
            {deleteError && (
              <div role="alert" className="p-3 rounded-lg bg-[var(--status-danger)]/10 border border-[var(--status-danger)]/30 text-[var(--status-danger)] text-xs font-medium">
                {deleteError}
              </div>
            )}
          </div>
        }
        confirmText={t("admin.agents.confirm_delete")}
        cancelText={t("admin.agents.cancel")}
        variant="danger"
      />
    </div>
  );
}
