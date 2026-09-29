/**
 * @inklore/admin-console - 模型配置与能力治理路由组件
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 模型一级类型 (文本模型/图片模型/嵌入/重排/视频/语音) 与能力体系解耦
 * 文本模型清晰划分视觉识别能力 (Vision)
 */

import { useState, useEffect, useCallback, useMemo } from "react";
import { Cpu, Plus, RefreshCw, Trash2, Edit3, Check, Eye } from "lucide-react";
import { Button, Modal, Input, ConfirmDialog, Badge } from "@inklore/ui";
import { safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { AdminNotice } from "../components/AdminNotice";
import { useAdminSession } from "../context/AdminSessionContext";
import { AdminHeader } from "../components/AdminHeader";
import { SearchFilterBar } from "../components/SearchFilterBar";
import { AdminPagination } from "../components/AdminPagination";
import { StatusBadge } from "../components/StatusBadge";
import { EmptyState } from "../components/EmptyState";
import {
  LlmModel,
  LlmProvider,
  LlmChannel,
  fetchLlmModels,
  fetchLlmProviders,
  fetchLlmChannels,
  createLlmModel,
  updateLlmModel,
  deleteLlmModel,
  createLlmProvider,
} from "../api/adminApi";
import {
  ModelType,
  MODEL_TYPE_OPTIONS,
  resolveModelClassification,
  composeModelCapabilities,
} from "../utils/modelCapabilities";
import { navigateAdmin } from "../navigation";

export function ModelConfigRoute() {
  const { t } = useTranslation();
  const { isAuthenticated } = useAdminSession();

  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const [noticeSuccess, setNoticeSuccess] = useState<string | null>(null);
  const dismissNotice = useCallback(() => {
    setLoadError(null);
    setActionError(null);
    setNoticeSuccess(null);
  }, []);
  const [submitting, setSubmitting] = useState(false);

  const [models, setModels] = useState<LlmModel[]>([]);
  const [providers, setProviders] = useState<LlmProvider[]>([]);
  const [channels, setChannels] = useState<LlmChannel[]>([]);

  // 搜索与过滤状态
  const [modelSearchQuery, setModelSearchQuery] = useState("");
  const [selectedProviderFilter, setSelectedProviderFilter] = useState("all");
  const [selectedTypeFilter, setSelectedTypeFilter] = useState("all");
  const [selectedVisionFilter, setSelectedVisionFilter] = useState("all");
  const [selectedStatusFilter, setSelectedStatusFilter] = useState("all");
  const [selectedChannelFilter, setSelectedChannelFilter] = useState("all");

  // 分页状态
  const [modelPage, setModelPage] = useState(1);
  const [modelPageSize, setModelPageSize] = useState(10);

  // 模型编辑/创建弹窗状态（通信协议严格归属渠道，模型页面绝不配置协议）
  const [modelModalOpen, setModelModalOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<LlmModel | null>(null);
  const [formModelProviderId, setFormModelProviderId] = useState<string>("");
  const [formModelUpstreamName, setFormModelUpstreamName] = useState("");
  const [formModelTitle, setFormModelTitle] = useState("");
  const [titleManuallyChanged, setTitleManuallyChanged] = useState(false);

  // 类型与能力解耦状态
  const [formModelPrimaryType, setFormModelPrimaryType] = useState<ModelType>("text");
  const [formModelSupportsVision, setFormModelSupportsVision] = useState(false);

  const [formModelContextWindow, setFormModelContextWindow] = useState("128000");
  const [formModelMaxOutput, setFormModelMaxOutput] = useState("8192");
  const [formModelEnabled, setFormModelEnabled] = useState(true);
  const [modelFormError, setModelFormError] = useState<string | null>(null);

  // 模型删除状态
  const [deleteModelConfirmOpen, setDeleteModelConfirmOpen] = useState(false);
  const [modelToDelete, setModelToDelete] = useState<LlmModel | null>(null);
  const [deletingModel, setDeletingModel] = useState(false);
  const [deleteModelError, setDeleteModelError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [m, p, c] = await Promise.all([fetchLlmModels(), fetchLlmProviders(), fetchLlmChannels()]);
      setModels(m);
      setProviders(p);
      setChannels(c);
      setLoadError(null);
    } catch (err) {
      setNoticeSuccess(null);
      setLoadError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isAuthenticated) {
      loadData();
    }
  }, [loadData, isAuthenticated]);

  const providerMap = useMemo(() => {
    const map = new Map<string, LlmProvider>();
    providers.forEach((p) => map.set(p.id, p));
    return map;
  }, [providers]);

  const modelChannelsMap = useMemo(() => {
    const map = new Map<string, LlmChannel[]>();
    channels.forEach((ch) => {
      const targetModelIds = new Set<string>();
      if (ch.model_id) targetModelIds.add(ch.model_id);
      if (ch.model_ids) ch.model_ids.forEach((id) => targetModelIds.add(id));
      if (ch.models) ch.models.forEach((bm) => targetModelIds.add(bm.model_id));

      targetModelIds.forEach((mid) => {
        const list = map.get(mid) || [];
        list.push(ch);
        map.set(mid, list);
      });
    });
    return map;
  }, [channels]);

  // 打开创建模型弹窗
  const handleOpenCreateModelModal = () => {
    setEditingModel(null);
    setModelFormError(null);
    setFormModelProviderId(providers[0]?.id || "");
    setFormModelUpstreamName("");
    setFormModelTitle("");
    setTitleManuallyChanged(false);
    setFormModelPrimaryType("text");
    setFormModelSupportsVision(false);
    setFormModelContextWindow("128000");
    setFormModelMaxOutput("8192");
    setFormModelEnabled(true);
    setModelModalOpen(true);
  };

  // 打开编辑模型弹窗
  const handleOpenEditModelModal = (m: LlmModel) => {
    setEditingModel(m);
    setModelFormError(null);
    setFormModelProviderId(m.provider_id);
    setFormModelUpstreamName(m.upstream_name);
    setFormModelTitle(m.title);
    setTitleManuallyChanged(true);

    const parsed = resolveModelClassification(m.capabilities);
    setFormModelPrimaryType(parsed.primaryType);
    setFormModelSupportsVision(parsed.supportsVision);

    setFormModelContextWindow(String(m.context_window || 128000));
    setFormModelMaxOutput(String(m.max_output_tokens || 8192));
    setFormModelEnabled(m.enabled);
    setModelModalOpen(true);
  };

  // 上游真实模型名输入联动
  const handleUpstreamNameChange = (val: string) => {
    setFormModelUpstreamName(val);
    if (!editingModel && !titleManuallyChanged) {
      setFormModelTitle(val);
    }
  };

  // 保存模型提交逻辑（严格遵循模型无协议原则，协议由渠道统一配置）
  const handleSubmitModel = async () => {
    if (!formModelUpstreamName.trim()) {
      setModelFormError(t("admin.models.err_upstream_required"));
      return;
    }
    if (!formModelTitle.trim()) {
      setModelFormError(t("admin.models.err_title_required"));
      return;
    }

    setModelFormError(null);
    setActionError(null);
    setNoticeSuccess(null);
    setSubmitting(true);

    try {
      let targetProviderId = formModelProviderId;
      if (!targetProviderId) {
        if (providers.length > 0) {
          targetProviderId = providers[0].id;
        } else {
          const defaultProv = await createLlmProvider({
            code: "default",
            title: "默认厂商",
            protocol: "openai",
            enabled: true,
          });
          targetProviderId = defaultProv.id;
        }
      }

      const capabilities = composeModelCapabilities(
        formModelPrimaryType,
        formModelSupportsVision
      );
      const isEmbedding = formModelPrimaryType === "embedding";

      const basePayload = {
        provider_id: targetProviderId,
        upstream_name: formModelUpstreamName.trim(),
        title: formModelTitle.trim(),
        capabilities,
        context_window: parseInt(formModelContextWindow, 10) || 128000,
        max_output_tokens: parseInt(formModelMaxOutput, 10) || 8192,
        enabled: formModelEnabled,
        embedding_dimensions: isEmbedding ? 1536 : null,
      };

      if (editingModel) {
        await updateLlmModel(editingModel.id, {
          expected_revision: editingModel.revision,
          ...basePayload,
        });
      } else {
        await createLlmModel(basePayload);
      }
      setModelModalOpen(false);
      setNoticeSuccess(t("common.success"));
      window.dispatchEvent(new CustomEvent("inklore:llm_catalog_changed"));
      await loadData();
    } catch (err: unknown) {
      setActionError(err ?? safeErrorText(err, t));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteModelClick = (m: LlmModel) => {
    setModelToDelete(m);
    setDeleteModelError(null);
    setDeleteModelConfirmOpen(true);
  };

  const handleConfirmDeleteModel = async () => {
    if (!modelToDelete) return;
    setDeletingModel(true);
    setDeleteModelError(null);
    try {
      await deleteLlmModel(modelToDelete.id, modelToDelete.revision);
      setDeleteModelConfirmOpen(false);
      setModelToDelete(null);
      setNoticeSuccess(t("common.success"));
      window.dispatchEvent(new CustomEvent("inklore:llm_catalog_changed"));
      await loadData();
    } catch (err: unknown) {
      setDeleteModelError(safeErrorText(err, t));
    } finally {
      setDeletingModel(false);
    }
  };

  // 过滤模型
  const filteredModels = useMemo(() => {
    return models.filter((m) => {
      const prov = providerMap.get(m.provider_id);
      const channelTitles = (modelChannelsMap.get(m.id) || []).map((c) => c.title.toLowerCase());
      const parsed = resolveModelClassification(m.capabilities);

      if (modelSearchQuery.trim()) {
        const q = modelSearchQuery.toLowerCase();
        const matches =
          m.title.toLowerCase().includes(q) ||
          m.upstream_name.toLowerCase().includes(q) ||
          prov?.title.toLowerCase().includes(q) ||
          prov?.code.toLowerCase().includes(q) ||
          channelTitles.some((t) => t.includes(q));
        if (!matches) return false;
      }

      if (selectedProviderFilter !== "all" && m.provider_id !== selectedProviderFilter) {
        return false;
      }

      // 按模型主类型过滤
      if (selectedTypeFilter !== "all" && parsed.primaryType !== selectedTypeFilter) {
        return false;
      }

      // 按视觉识别支持状态过滤
      if (selectedVisionFilter !== "all") {
        if (selectedVisionFilter === "vision" && !parsed.supportsVision) return false;
        if (selectedVisionFilter === "text_only" && (parsed.primaryType !== "text" || parsed.supportsVision)) return false;
      }

      if (selectedStatusFilter !== "all") {
        if (selectedStatusFilter === "enabled" && !m.enabled) return false;
        if (selectedStatusFilter === "disabled" && m.enabled) return false;
      }

      if (selectedChannelFilter !== "all") {
        const count = modelChannelsMap.get(m.id)?.length || 0;
        if (selectedChannelFilter === "bound" && count === 0) return false;
        if (selectedChannelFilter === "unbound" && count > 0) return false;
      }

      return true;
    });
  }, [
    models,
    modelSearchQuery,
    selectedProviderFilter,
    selectedTypeFilter,
    selectedVisionFilter,
    selectedStatusFilter,
    selectedChannelFilter,
    providerMap,
    modelChannelsMap,
  ]);

  const paginatedModels = useMemo(() => {
    const start = (modelPage - 1) * modelPageSize;
    return filteredModels.slice(start, start + modelPageSize);
  }, [filteredModels, modelPage, modelPageSize]);

  // 筛选器选项
  const providerFilterOptions = useMemo(() => {
    const opts = [{ label: t("admin.components.filter_provider"), value: "all" }];
    providers.forEach((p) => {
      opts.push({ label: p.title || p.code, value: p.id });
    });
    return opts;
  }, [providers, t]);

  const typeFilterOptions = useMemo(() => {
    const opts = [{ label: t("admin.models.filter_type") + " (全部)", value: "all" }];
    MODEL_TYPE_OPTIONS.forEach((tp) => {
      opts.push({ label: t(tp.labelKey), value: tp.id });
    });
    return opts;
  }, [t]);

  const visionFilterOptions = useMemo(() => [
    { label: t("admin.models.filter_vision_all"), value: "all" },
    { label: t("admin.models.filter_vision_yes"), value: "vision" },
    { label: t("admin.models.filter_vision_no"), value: "text_only" },
  ], [t]);

  const statusFilterOptions = useMemo(() => [
    { label: t("admin.components.filter_status"), value: "all" },
    { label: t("admin.components.status_enabled"), value: "enabled" },
    { label: t("admin.components.status_disabled"), value: "disabled" },
  ], [t]);

  const channelFilterOptions = useMemo(() => [
    { label: t("admin.models.filter_channels_all"), value: "all" },
    { label: t("admin.models.filter_channels_bound"), value: "bound" },
    { label: t("admin.models.filter_channels_unbound"), value: "unbound" },
  ], [t]);

  const hasModelFilters =
    modelSearchQuery !== "" ||
    selectedProviderFilter !== "all" ||
    selectedTypeFilter !== "all" ||
    selectedVisionFilter !== "all" ||
    selectedStatusFilter !== "all" ||
    selectedChannelFilter !== "all";

  const handleResetModelFilters = () => {
    setModelSearchQuery("");
    setSelectedProviderFilter("all");
    setSelectedTypeFilter("all");
    setSelectedVisionFilter("all");
    setSelectedStatusFilter("all");
    setSelectedChannelFilter("all");
    setModelPage(1);
  };

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminNotice
        error={actionError ?? loadError}
        success={noticeSuccess}
        onRetry={actionError == null && loadError != null ? () => void loadData() : undefined}
        onDismiss={dismissNotice}
      />
      {/* Header */}
      <AdminHeader
        title={t("admin.models.title")}
        actions={
          <div className="flex items-center gap-2 flex-wrap">
            <Button
              size="sm"
              variant="outline"
              icon={<RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} />}
              onClick={loadData}
              title={t("common.refresh")}
              aria-label={t("common.refresh")}
            >
              {t("common.refresh")}
            </Button>
            <Button
              size="sm"
              variant="primary"
              icon={<Plus className="w-3.5 h-3.5" />}
              onClick={handleOpenCreateModelModal}
            >
              {t("admin.models.add_model_btn")}
            </Button>
          </div>
        }
        className="px-6 py-3.5 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)] shrink-0"
      />

      {/* Search & Filter Bar */}
      <SearchFilterBar
        searchQuery={modelSearchQuery}
        onSearchChange={(val) => {
          setModelSearchQuery(val);
          setModelPage(1);
        }}
        filters={[
          {
            id: "provider",
            value: selectedProviderFilter,
            options: providerFilterOptions,
            onChange: (val) => {
              setSelectedProviderFilter(val);
              setModelPage(1);
            },
          },
          {
            id: "type",
            value: selectedTypeFilter,
            options: typeFilterOptions,
            onChange: (val) => {
              setSelectedTypeFilter(val);
              setModelPage(1);
            },
          },
          {
            id: "vision",
            value: selectedVisionFilter,
            options: visionFilterOptions,
            onChange: (val) => {
              setSelectedVisionFilter(val);
              setModelPage(1);
            },
          },
          {
            id: "status",
            value: selectedStatusFilter,
            options: statusFilterOptions,
            onChange: (val) => {
              setSelectedStatusFilter(val);
              setModelPage(1);
            },
          },
          {
            id: "channels",
            value: selectedChannelFilter,
            options: channelFilterOptions,
            onChange: (val) => {
              setSelectedChannelFilter(val);
              setModelPage(1);
            },
          },
        ]}
        showReset={hasModelFilters}
        onReset={handleResetModelFilters}
        className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 rounded-none border-x-0 border-t-0 shadow-none"
      />

      {/* Models Table */}
      <div className="flex-1 min-h-0 flex flex-col bg-[var(--bg-surface)] overflow-hidden">
        {paginatedModels.length === 0 ? (
          <div className="flex-1 min-h-0 flex items-center justify-center p-8 sm:p-12">
            <EmptyState
              icon={<Cpu className="w-6 h-6 text-[var(--action-primary)]" />}
              action={
                hasModelFilters ? (
                  <Button size="sm" variant="outline" onClick={handleResetModelFilters}>
                    {t("common.reset_filters")}
                  </Button>
                ) : (
                  <Button size="sm" variant="primary" icon={<Plus className="w-3.5 h-3.5" />} onClick={handleOpenCreateModelModal}>
                    {t("admin.models.add_model_btn")}
                  </Button>
                )
              }
            />
          </div>
        ) : (
          <div className="flex-1 min-h-0 overflow-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead className="sticky top-0 bg-[var(--bg-subtle)] border-b border-[var(--border-decorative)] text-[var(--text-secondary)] font-semibold z-10">
                <tr>
                  <th className="p-3.5 whitespace-nowrap min-w-[170px]">{t("admin.models.col_title")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[170px]">{t("admin.models.col_upstream")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[130px]">{t("admin.models.col_provider")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[100px]">{t("admin.models.col_channels")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[110px]">{t("admin.models.col_type")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[110px]">{t("admin.models.col_capabilities")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[90px]">{t("admin.models.col_context_window")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[90px]">{t("admin.models.col_max_output")}</th>
                  <th className="p-3.5 whitespace-nowrap min-w-[80px]">{t("admin.models.col_status")}</th>
                  <th className="p-3.5 text-right whitespace-nowrap min-w-[90px]">{t("admin.models.col_actions")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-decorative)]">
                {paginatedModels.map((m) => {
                  const prov = providerMap.get(m.provider_id);
                  const boundChannels = modelChannelsMap.get(m.id) || [];
                  const parsed = resolveModelClassification(m.capabilities);
                  const typeOpt = MODEL_TYPE_OPTIONS.find((t) => t.id === parsed.primaryType);

                  return (
                    <tr
                      key={m.id}
                      className="hover:bg-[var(--bg-subtle)]/50 transition-colors group"
                    >
                      <td className="p-3.5 font-medium text-[var(--text-primary)]">
                        <div className="flex items-center gap-2">
                          <Cpu className="w-3.5 h-3.5 text-[var(--action-primary)] shrink-0" />
                          <span>{m.title}</span>
                        </div>
                      </td>
                      <td className="p-3.5 font-mono text-[var(--text-secondary)]">
                        {m.upstream_name}
                      </td>
                      <td className="p-3.5 whitespace-nowrap">
                        <span className="font-medium text-[var(--text-primary)]">
                          {prov ? prov.title || prov.code : "-"}
                        </span>
                      </td>
                      <td className="p-3.5">
                        {boundChannels.length > 0 ? (
                          <button
                            type="button"
                            onClick={() => navigateAdmin("channels")}
                            className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-medium bg-[var(--action-primary)]/10 text-[var(--action-primary)] border border-[var(--action-primary)]/20 hover:bg-[var(--action-primary)]/20 transition-colors cursor-pointer"
                            title={boundChannels.map((c) => c.title).join(", ")}
                          >
                            <span>{t("admin.models.channels_count", String(boundChannels.length))}</span>
                          </button>
                        ) : (
                          <button
                            type="button"
                            onClick={() => navigateAdmin("channels")}
                            className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-medium bg-[var(--status-warning)]/10 text-[var(--status-warning)] border border-[var(--status-warning)]/30 hover:bg-[var(--status-warning)]/20 transition-colors cursor-pointer"
                            title={t("admin.models.no_channels")}
                          >
                            <span>{t("admin.models.no_channels")}</span>
                          </button>
                        )}
                      </td>
                      {/* 模型类型列 */}
                      <td className="p-3.5 whitespace-nowrap">
                        <Badge variant="default" size="sm">
                          {typeOpt ? t(typeOpt.labelKey) : parsed.primaryType}
                        </Badge>
                      </td>
                      {/* 能力与视觉列 */}
                      <td className="p-3.5 whitespace-nowrap">
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {parsed.primaryType === "text" ? (
                            <Badge
                              variant={parsed.supportsVision ? "success" : "secondary"}
                              size="sm"
                              className="gap-1 font-semibold"
                            >
                              {parsed.supportsVision && <Eye className="w-3 h-3" />}
                              <span>
                                {parsed.supportsVision
                                  ? t("admin.models.vision_supported")
                                  : t("admin.models.vision_unsupported")}
                              </span>
                            </Badge>
                          ) : (
                            <span className="text-[11px] text-[var(--text-muted)]">-</span>
                          )}
                        </div>
                      </td>
                      <td className="p-3.5 font-mono whitespace-nowrap">
                        {(m.context_window / 1024).toFixed(0)}K
                      </td>
                      <td className="p-3.5 font-mono whitespace-nowrap">
                        {(m.max_output_tokens / 1024).toFixed(0)}K
                      </td>
                      <td className="p-3.5 whitespace-nowrap">
                        <StatusBadge status={m.enabled ? "enabled" : "disabled"} />
                      </td>
                      <td className="p-3.5 text-right whitespace-nowrap">
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            size="icon"
                            variant="ghost"
                            onClick={() => handleOpenEditModelModal(m)}
                            title={t("common.edit")}
                            aria-label={t("common.edit")}
                          >
                            <Edit3 className="w-3.5 h-3.5" />
                          </Button>
                          <Button
                            size="icon"
                            variant="ghost"
                            onClick={() => handleDeleteModelClick(m)}
                            title={t("common.delete")}
                            aria-label={t("common.delete")}
                            className="hover:text-[var(--status-danger)] hover:bg-[var(--status-danger)]/10"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        <AdminPagination
          page={modelPage}
          pageSize={modelPageSize}
          total={filteredModels.length}
          onPageChange={setModelPage}
          onPageSizeChange={(size) => {
            setModelPageSize(size);
            setModelPage(1);
          }}
          className="border-t border-[var(--border-decorative)] bg-[var(--bg-surface)] shrink-0"
        />
      </div>

      {/* Model Create/Edit Modal */}
      <Modal
        open={modelModalOpen}
        onClose={() => !submitting && setModelModalOpen(false)}
        title={editingModel ? t("admin.models.modal_edit_title") : t("admin.models.modal_create_title")}
        maxWidth="2xl"
      >
        <div className="space-y-4 text-xs text-[var(--text-primary)]">
          {modelFormError && (
            <div className="p-3 rounded-xl bg-[var(--status-danger)]/10 border border-[var(--status-danger)]/30 text-[var(--status-danger)] font-medium">
              {modelFormError}
            </div>
          )}

          {/* 上游真实模型名 */}
          <div className="space-y-1">
            <label className="font-semibold text-[var(--text-secondary)]">
              {t("admin.models.modal_upstream_name")}
            </label>
            <Input
              value={formModelUpstreamName}
              onChange={(e) => handleUpstreamNameChange(e.target.value)}
              placeholder={t("admin.models.modal_upstream_name_ph")}
            />
          </div>

          {/* 展示名称 */}
          <div className="space-y-1">
            <label className="font-semibold text-[var(--text-secondary)]">
              {t("admin.models.modal_title_label")}
            </label>
            <Input
              value={formModelTitle}
              onChange={(e) => {
                setFormModelTitle(e.target.value);
                setTitleManuallyChanged(true);
              }}
              placeholder={t("admin.models.modal_title_ph")}
            />
          </div>

          {/* 所属厂商 (仅当存在多个在册供应商时提供选择) */}
          {providers.length > 1 && (
            <div className="space-y-1">
              <label className="font-semibold text-[var(--text-secondary)]">
                {t("admin.models.col_provider")}
              </label>
              <select
                value={formModelProviderId}
                onChange={(e) => setFormModelProviderId(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] transition-colors cursor-pointer text-xs"
              >
                {providers.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.title || p.code}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* 模块 1: 模型类型 (单选卡片组) */}
          <div className="space-y-1.5">
            <label className="font-semibold text-[var(--text-secondary)]">
              {t("admin.models.section_type")}
            </label>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {MODEL_TYPE_OPTIONS.map((tp) => {
                const isSelected = formModelPrimaryType === tp.id;
                return (
                  <button
                    key={tp.id}
                    type="button"
                    onClick={() => {
                      setFormModelPrimaryType(tp.id);
                      if (tp.id === "embedding") {
                        setFormModelContextWindow("8192");
                        setFormModelMaxOutput("2048");
                      }
                    }}
                    className={`p-2.5 rounded-xl border text-left transition-all cursor-pointer ${
                      isSelected
                        ? "bg-[var(--action-primary)]/10 border-[var(--action-primary)] text-[var(--action-primary)] shadow-xs"
                        : "bg-[var(--bg-canvas)] border-[var(--border-decorative)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--border-strong)]"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs">{t(tp.labelKey)}</span>
                      {isSelected && <Check className="w-3.5 h-3.5 text-[var(--action-primary)]" />}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* 模块 2: 核心分水岭能力 (图片识别) */}
          {formModelPrimaryType === "text" && (
            <div className="space-y-1.5">
              <label className="font-semibold text-[var(--text-secondary)]">
                {t("admin.models.section_core_capabilities")}
              </label>
              <div
                onClick={() => setFormModelSupportsVision(!formModelSupportsVision)}
                className={`p-3 rounded-xl border transition-all cursor-pointer flex items-center justify-between ${
                  formModelSupportsVision
                    ? "bg-emerald-500/10 border-emerald-500/40 shadow-xs"
                    : "bg-[var(--bg-canvas)] border-[var(--border-decorative)] hover:bg-[var(--bg-subtle)]/40 hover:border-[var(--border-strong)]"
                }`}
              >
                <div className="flex items-center gap-2.5">
                  <div className={`w-6 h-6 rounded-lg flex items-center justify-center ${formModelSupportsVision ? "bg-emerald-500/20 text-emerald-600 dark:text-emerald-400" : "bg-[var(--bg-subtle)] text-[var(--text-muted)]"}`}>
                    <Eye className="w-3.5 h-3.5" />
                  </div>
                  <span className={`font-semibold text-xs ${formModelSupportsVision ? "text-emerald-600 dark:text-emerald-400" : "text-[var(--text-primary)]"}`}>
                    {t("admin.models.vision_toggle_label")}
                  </span>
                </div>
                <input
                  type="checkbox"
                  checked={formModelSupportsVision}
                  onChange={() => {}}
                  className="rounded accent-emerald-500 cursor-pointer h-4 w-4 pointer-events-none"
                />
              </div>
            </div>
          )}

          {/* Context Window & Max Output */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="font-semibold text-[var(--text-secondary)]">
                {t("admin.models.modal_context_window")}
              </label>
              <Input
                type="number"
                value={formModelContextWindow}
                onChange={(e) => setFormModelContextWindow(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <label className="font-semibold text-[var(--text-secondary)]">
                {t("admin.models.modal_max_output")}
              </label>
              <Input
                type="number"
                value={formModelMaxOutput}
                onChange={(e) => setFormModelMaxOutput(e.target.value)}
              />
            </div>
          </div>

          {/* Enabled Checkbox */}
          <div className="flex items-center gap-2 pt-2">
            <input
              type="checkbox"
              id="model_enabled"
              checked={formModelEnabled}
              onChange={(e) => setFormModelEnabled(e.target.checked)}
              className="rounded accent-[var(--action-primary)] cursor-pointer h-3.5 w-3.5"
            />
            <label htmlFor="model_enabled" className="cursor-pointer font-medium text-xs">
              {t("admin.models.modal_enabled")}
            </label>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-2 pt-4 border-t border-[var(--border-decorative)]">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setModelModalOpen(false)}
              disabled={submitting}
            >
              {t("common.cancel")}
            </Button>
            <Button
              type="button"
              variant="primary"
              size="sm"
              onClick={handleSubmitModel}
              disabled={submitting}
            >
              {submitting ? t("common.saving") : t("common.save")}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Model Delete Confirmation */}
      <ConfirmDialog
        open={deleteModelConfirmOpen}
        title={t("admin.models.modal_edit_title")}
        description={
          <div className="space-y-2">
            <p>{t("admin.models.confirm_delete", modelToDelete?.title || "")}</p>
            <p className="text-xs text-[var(--status-danger)]">
              {t("admin.models.confirm_delete_cascade")}
            </p>
            {deleteModelError && (
              <p className="text-xs text-[var(--status-danger)] font-medium">
                {deleteModelError}
              </p>
            )}
          </div>
        }
        confirmText={t("common.delete")}
        cancelText={t("common.cancel")}
        variant="danger"
        loading={deletingModel}
        onConfirm={handleConfirmDeleteModel}
        onClose={() => !deletingModel && setDeleteModelConfirmOpen(false)}
      />
    </div>
  );
}
