/**
 * @inklore/admin-console - 渠道治理路由组件 (Channels & Keys)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 接入真实后端 /v1/llm/channels 与 /v1/llm/keys 接口
 */

import { useState, useEffect, useCallback, useMemo } from "react";
import {
  Layers,
  Plus,
  RefreshCw,
  Trash2,
  Edit3,
  KeyRound,
  LayoutGrid,
  List,
  Copy,
  Check,
  Activity,
  Cpu,
  Key,
} from "lucide-react";
import { Button, ConfirmDialog } from "@inklore/ui";
import { safeErrorText } from "@inklore/transport";
import { useTranslation } from "@inklore/i18n";
import { AdminNotice } from "../components/AdminNotice";
import { AdminHeader } from "../components/AdminHeader";
import { useAdminSession } from "../context/AdminSessionContext";
import { SearchFilterBar } from "../components/SearchFilterBar";
import { AdminPagination } from "../components/AdminPagination";
import { StatusBadge } from "../components/StatusBadge";
import { EmptyState } from "../components/EmptyState";
import { ChannelModal } from "./ChannelModal";
import { ChannelKeysModal } from "./ChannelKeysModal";
import {
  LlmChannel,
  LlmKey,
  LlmModel,
  LlmProvider,
  fetchLlmChannels,
  fetchLlmKeys,
  fetchLlmModels,
  fetchLlmProviders,
  deleteLlmChannel,
} from "../api/adminApi";

export function ChannelsRoute() {
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
  const [channels, setChannels] = useState<LlmChannel[]>([]);
  const [keys, setKeys] = useState<LlmKey[]>([]);
  const [providers, setProviders] = useState<LlmProvider[]>([]);
  const [models, setModels] = useState<LlmModel[]>([]);

  // Search & Filters state
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedProviderFilter, setSelectedProviderFilter] = useState("all");
  const [selectedStatusFilter, setSelectedStatusFilter] = useState("all");

  // 视图模式与交互状态
  const [viewMode, setViewMode] = useState<"grid" | "table">(() => {
    return (localStorage.getItem("inklore_admin_channels_view") as "grid" | "table") || "grid";
  });
  const [copiedChannelId, setCopiedChannelId] = useState<string | null>(null);

  const handleCopyEndpoint = (id: string, url: string) => {
    navigator.clipboard.writeText(url).then(() => {
      setCopiedChannelId(id);
      setTimeout(() => setCopiedChannelId(null), 1500);
    });
  };

  const handleToggleViewMode = (mode: "grid" | "table") => {
    setViewMode(mode);
    localStorage.setItem("inklore_admin_channels_view", mode);
  };

  // Pagination state
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(12);

  // Modal State (Create / Edit via ChannelModal)
  const [modalOpen, setModalOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<LlmChannel | null>(null);

  // Delete Confirm Dialog State
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [channelToDelete, setChannelToDelete] = useState<LlmChannel | null>(null);
  const [channelForKeys, setChannelForKeys] = useState<LlmChannel | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [c, k, p, m] = await Promise.all([
        fetchLlmChannels(),
        fetchLlmKeys(),
        fetchLlmProviders(),
        fetchLlmModels(),
      ]);
      setChannels(c);
      setKeys(k);
      setProviders(p);
      setModels(m);
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

  useEffect(() => {
    const handleCatalogChanged = () => {
      void loadData();
    };
    window.addEventListener("inklore:llm_catalog_changed", handleCatalogChanged);
    return () => {
      window.removeEventListener("inklore:llm_catalog_changed", handleCatalogChanged);
    };
  }, [loadData]);

  const providerMap = useMemo(() => {
    const map = new Map<string, LlmProvider>();
    providers.forEach((p) => map.set(p.id, p));
    return map;
  }, [providers]);

  const modelMap = useMemo(() => {
    const map = new Map<string, LlmModel>();
    models.forEach((m) => map.set(m.id, m));
    return map;
  }, [models]);

  const keyMap = useMemo(() => {
    const map = new Map<string, LlmKey[]>();
    keys.forEach((k) => {
      const existing = map.get(k.channel_id) || [];
      existing.push(k);
      map.set(k.channel_id, existing);
    });
    return map;
  }, [keys]);

  // 渠道统计概览
  const channelStats = useMemo(() => {
    const total = channels.length;
    const enabled = channels.filter((c) => c.enabled).length;
    const allBoundModelNames = new Set<string>();
    channels.forEach((c) => {
      if (c.models) c.models.forEach((m) => allBoundModelNames.add(m.model_id));
      if (c.model_ids) c.model_ids.forEach((id) => allBoundModelNames.add(id));
      if (c.model_id) allBoundModelNames.add(c.model_id);
    });
    const totalKeys = keys.length;
    return { total, enabled, mappedModels: allBoundModelNames.size, totalKeys };
  }, [channels, keys]);

  // 打开创建弹窗
  const handleOpenCreateModal = () => {
    setEditingChannel(null);
    setModalOpen(true);
  };

  // 打开编辑弹窗
  const handleOpenEditModal = (ch: LlmChannel) => {
    setEditingChannel(ch);
    setModalOpen(true);
  };

  // 删除渠道
  const handleDeleteClick = (ch: LlmChannel) => {
    setChannelToDelete(ch);
    setDeleteError(null);
    setActionError(null);
    setNoticeSuccess(null);
    setDeleteConfirmOpen(true);
  };

  const handleManageKeys = (channel: LlmChannel) => setChannelForKeys(channel);

  const handleExecuteDelete = async () => {
    if (!channelToDelete) return;
    setDeleteError(null);
    setActionError(null);
    setDeleting(true);
    try {
      await deleteLlmChannel(channelToDelete.id, channelToDelete.revision);
      setDeleteConfirmOpen(false);
      setChannelToDelete(null);
      setNoticeSuccess(t("common.success"));
      window.dispatchEvent(new CustomEvent("inklore:llm_catalog_changed"));
      await loadData();
    } catch (err: any) {
      setDeleteError(safeErrorText(err, t));
      setActionError(err ?? safeErrorText(err, t));
    } finally {
      setDeleting(false);
    }
  };

  // 搜索与过滤
  const filteredChannels = useMemo(() => {
    return channels.filter((ch) => {
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const prov = providerMap.get(ch.provider_id);
        const mdl = ch.model_id ? modelMap.get(ch.model_id) : undefined;
        const matches =
          ch.title.toLowerCase().includes(q) ||
          (ch.upstream_model ? ch.upstream_model.toLowerCase().includes(q) : false) ||
          (ch.models ? ch.models.some((bm) => bm.upstream_model?.toLowerCase().includes(q)) : false) ||
          ch.base_url.toLowerCase().includes(q) ||
          (prov ? prov.title.toLowerCase().includes(q) : false) ||
          (mdl ? mdl.title.toLowerCase().includes(q) : false);
        if (!matches) return false;
      }
      if (selectedProviderFilter !== "all" && ch.provider_id !== selectedProviderFilter) {
        return false;
      }
      if (selectedStatusFilter !== "all") {
        if (selectedStatusFilter === "enabled" && !ch.enabled) return false;
        if (selectedStatusFilter === "disabled" && ch.enabled) return false;
        if (
          selectedStatusFilter === "cooling" &&
          ch.breaker_state !== "open" &&
          ch.breaker_state !== "cooling"
        ) {
          return false;
        }
      }
      return true;
    });
  }, [channels, searchQuery, selectedProviderFilter, selectedStatusFilter, providerMap, modelMap]);

  // 分页
  const paginatedChannels = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredChannels.slice(start, start + pageSize);
  }, [filteredChannels, page, pageSize]);

  // 筛选选项
  const providerFilterOptions = useMemo(() => {
    const opts = [{ label: t("admin.components.filter_provider"), value: "all" }];
    providers.forEach((p) => {
      opts.push({ label: p.title || p.code, value: p.id });
    });
    return opts;
  }, [providers, t]);

  const statusFilterOptions = useMemo(
    () => [
      { label: t("admin.components.filter_status"), value: "all" },
      { label: t("admin.components.status_enabled"), value: "enabled" },
      { label: t("admin.components.status_cooling"), value: "cooling" },
      { label: t("admin.components.status_disabled"), value: "disabled" },
    ],
    [t]
  );

  const hasActiveFilters =
    searchQuery !== "" || selectedProviderFilter !== "all" || selectedStatusFilter !== "all";

  const handleResetFilters = () => {
    setSearchQuery("");
    setSelectedProviderFilter("all");
    setSelectedStatusFilter("all");
    setPage(1);
  };

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminNotice
        error={actionError ?? loadError}
        success={noticeSuccess}
        onRetry={actionError == null && loadError != null ? () => void loadData() : undefined}
        onDismiss={dismissNotice}
      />
      {/* 顶部标题栏与全局操作 */}
      <AdminHeader
        title={t("admin.channels.title")}
        actions={
          <div className="flex items-center gap-2 flex-wrap">
            {/* 视图模式切换 */}
            <div className="flex items-center p-0.5 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] mr-1">
              <button
                type="button"
                onClick={() => handleToggleViewMode("grid")}
                className={`p-1.5 rounded-md transition-colors cursor-pointer ${
                  viewMode === "grid"
                    ? "bg-[var(--bg-canvas)] text-[var(--action-primary)] shadow-xs font-semibold"
                    : "text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                }`}
                title="卡片视图"
                aria-label="卡片视图"
              >
                <LayoutGrid className="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                onClick={() => handleToggleViewMode("table")}
                className={`p-1.5 rounded-md transition-colors cursor-pointer ${
                  viewMode === "table"
                    ? "bg-[var(--bg-canvas)] text-[var(--action-primary)] shadow-xs font-semibold"
                    : "text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                }`}
                title="表格视图"
                aria-label="表格视图"
              >
                <List className="w-3.5 h-3.5" />
              </button>
            </div>
            <Button
              size="sm"
              variant="outline"
              icon={<RefreshCw className={"w-3.5 h-3.5 " + (loading ? "animate-spin" : "")} />}
              onClick={loadData}
              title={t("common.refresh")}
            >
              {t("common.refresh")}
            </Button>
            <Button
              size="sm"
              variant="primary"
              icon={<Plus className="w-3.5 h-3.5 text-[var(--action-on-primary)]" />}
              onClick={handleOpenCreateModal}
            >
              {t("admin.channels.add_btn")}
            </Button>
          </div>
        }
        className="px-6 py-3.5 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)] shrink-0"
      />

      {/* 渠道概览统计指标条 */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 px-6 py-3 bg-[var(--bg-canvas)]/60 border-b border-[var(--border-decorative)] shrink-0">
        <div className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/70">
          <div className="w-8 h-8 rounded-lg bg-[var(--action-primary)]/10 text-[var(--action-primary)] flex items-center justify-center shrink-0">
            <Layers className="w-4 h-4" />
          </div>
          <div>
            <span className="text-[11px] text-[var(--text-muted)] block">接入渠道</span>
            <span className="font-semibold text-sm text-[var(--text-primary)] font-mono">{channelStats.total}</span>
          </div>
        </div>
        <div className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/70">
          <div className="w-8 h-8 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shrink-0">
            <Activity className="w-4 h-4" />
          </div>
          <div>
            <span className="text-[11px] text-[var(--text-muted)] block">正常启用</span>
            <span className="font-semibold text-sm text-[var(--text-primary)] font-mono">{channelStats.enabled}</span>
          </div>
        </div>
        <div className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/70">
          <div className="w-8 h-8 rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400 flex items-center justify-center shrink-0">
            <Cpu className="w-4 h-4" />
          </div>
          <div>
            <span className="text-[11px] text-[var(--text-muted)] block">绑定模型</span>
            <span className="font-semibold text-sm text-[var(--text-primary)] font-mono">{channelStats.mappedModels}</span>
          </div>
        </div>
        <div className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/70">
          <div className="w-8 h-8 rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center justify-center shrink-0">
            <Key className="w-4 h-4" />
          </div>
          <div>
            <span className="text-[11px] text-[var(--text-muted)] block">配置密钥</span>
            <span className="font-semibold text-sm text-[var(--text-primary)] font-mono">{channelStats.totalKeys}</span>
          </div>
        </div>
      </div>

      {/* 搜索与筛选控制栏 */}
      <SearchFilterBar
        searchQuery={searchQuery}
        onSearchChange={(val) => {
          setSearchQuery(val);
          setPage(1);
        }}
        filters={[
          {
            id: "provider",
            value: selectedProviderFilter,
            options: providerFilterOptions,
            onChange: (val) => {
              setSelectedProviderFilter(val);
              setPage(1);
            },
          },
          {
            id: "status",
            value: selectedStatusFilter,
            options: statusFilterOptions,
            onChange: (val) => {
              setSelectedStatusFilter(val);
              setPage(1);
            },
          },
        ]}
        showReset={hasActiveFilters}
        onReset={handleResetFilters}
        className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 rounded-none border-x-0 border-t-0 shadow-none"
      />

      {/* 渠道列表大盘 */}
      <div className="flex-1 min-h-0 flex flex-col bg-[var(--bg-surface)] overflow-hidden">
        {paginatedChannels.length === 0 ? (
          <div className="flex-1 min-h-0 flex items-center justify-center p-8 sm:p-12">
            <EmptyState
              icon={<Layers className="w-6 h-6 text-[var(--action-primary)]" />}
              action={
                hasActiveFilters ? (
                  <Button size="sm" variant="outline" onClick={handleResetFilters}>
                    {t("admin.components.reset_filters")}
                  </Button>
                ) : (
                  <Button size="sm" variant="primary" onClick={handleOpenCreateModal}>
                    {t("admin.channels.add_btn")}
                  </Button>
                )
              }
            />
          </div>
        ) : viewMode === "grid" ? (
          /* 卡片网格视图 */
          <div className="flex-1 min-h-0 overflow-y-auto p-4 sm:p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {paginatedChannels.map((ch) => {
                const prov = providerMap.get(ch.provider_id);
                const boundKeys = keyMap.get(ch.id) || [];
                const isCooling = ch.breaker_state === "open" || ch.breaker_state === "cooling";
                const breakerKey = "admin.dashboard.breaker_" + (ch.breaker_state || "closed");
                const isCopied = copiedChannelId === ch.id;

                const modelBadges: { id: string; name: string }[] = [];
                if (ch.models && ch.models.length > 0) {
                  ch.models.forEach((bm) => {
                    const m = modelMap.get(bm.model_id);
                    modelBadges.push({
                      id: bm.id || bm.model_id,
                      name: m?.title || bm.upstream_model || bm.model_id,
                    });
                  });
                } else if (ch.model_ids && ch.model_ids.length > 0) {
                  ch.model_ids.forEach((mid) => {
                    const m = modelMap.get(mid);
                    modelBadges.push({ id: mid, name: m?.title || mid });
                  });
                } else if (ch.upstream_model || ch.model_id) {
                  const m = ch.model_id ? modelMap.get(ch.model_id) : undefined;
                  modelBadges.push({
                    id: ch.model_id || "single",
                    name: m?.title || ch.upstream_model || "-",
                  });
                }

                return (
                  <div
                    key={ch.id}
                    className="group rounded-2xl bg-[var(--bg-canvas)] border border-[var(--border-decorative)] hover:border-[var(--action-primary)]/50 shadow-xs hover:shadow-md transition-all duration-200 flex flex-col justify-between overflow-hidden"
                  >
                    {/* 卡片头部 */}
                    <div className="p-4 sm:p-5 border-b border-[var(--border-decorative)]/60 bg-[var(--bg-surface)] flex items-start justify-between gap-3">
                      <div className="flex items-start gap-3 min-w-0">
                        <div className="w-10 h-10 rounded-xl bg-[var(--action-primary)]/10 text-[var(--action-primary)] flex items-center justify-center shrink-0">
                          <Layers className="w-5 h-5" />
                        </div>
                        <div className="min-w-0">
                          <h3 className="font-semibold text-sm text-[var(--text-primary)] truncate" title={ch.title}>
                            {ch.title}
                          </h3>
                          <p className="text-[11px] text-[var(--text-muted)] truncate mt-0.5">
                            {prov?.title || prov?.code || ch.provider_id}
                          </p>
                        </div>
                      </div>
                      <div className="flex flex-col items-end gap-1.5 shrink-0">
                        <StatusBadge status={ch.enabled ? "enabled" : "disabled"} />
                        <StatusBadge
                          status={isCooling ? "cooling" : "healthy"}
                          label={t(breakerKey)}
                        />
                      </div>
                    </div>

                    {/* 卡片主体 */}
                    <div className="p-4 sm:p-5 space-y-3.5 flex-1 text-xs">
                      {/* 网关端点 */}
                      <div className="space-y-1">
                        <span className="text-[11px] font-medium text-[var(--text-muted)] block">
                          {t("admin.channels.col_endpoint")}
                        </span>
                        <div className="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] font-mono text-[11px] text-[var(--text-secondary)]">
                          <span className="truncate" title={ch.base_url}>{ch.base_url}</span>
                          <button
                            type="button"
                            onClick={() => handleCopyEndpoint(ch.id, ch.base_url)}
                            className="p-1 hover:text-[var(--action-primary)] text-[var(--text-muted)] transition-colors cursor-pointer shrink-0"
                            title={isCopied ? "已复制" : "复制端点"}
                          >
                            {isCopied ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
                          </button>
                        </div>
                      </div>

                      {/* 绑定模型 */}
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between text-[11px] text-[var(--text-muted)] font-medium">
                          <span>{t("admin.channels.col_model")}</span>
                          <span className="font-mono">{modelBadges.length} 个模型</span>
                        </div>
                        <div className="flex flex-wrap gap-1.5 max-h-20 overflow-y-auto pr-1">
                          {modelBadges.length === 0 ? (
                            <span className="text-[11px] text-[var(--text-muted)] italic">未绑定模型</span>
                          ) : (
                            modelBadges.slice(0, 4).map((b) => (
                              <span
                                key={b.id}
                                className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-mono bg-[var(--action-primary)]/10 text-[var(--action-primary)] border border-[var(--action-primary)]/20"
                              >
                                {b.name}
                              </span>
                            ))
                          )}
                          {modelBadges.length > 4 && (
                            <span className="inline-flex items-center px-1.5 py-0.5 rounded-md text-[10px] font-mono bg-[var(--bg-subtle)] text-[var(--text-muted)] border border-[var(--border-decorative)]">
                              +{modelBadges.length - 4}
                            </span>
                          )}
                        </div>
                      </div>

                      {/* 调度指标微仪表 */}
                      <div className="grid grid-cols-3 gap-2 pt-2 border-t border-[var(--border-decorative)]/50 text-center">
                        <div className="p-2 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/40">
                          <span className="text-[10px] text-[var(--text-muted)] block">{t("admin.channels.col_weight")}</span>
                          <span className="font-mono font-semibold text-xs text-[var(--text-primary)]">{ch.weight}</span>
                        </div>
                        <div className="p-2 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/40">
                          <span className="text-[10px] text-[var(--text-muted)] block">{t("admin.channels.col_rpm")}</span>
                          <span className="font-mono font-semibold text-xs text-[var(--text-primary)]">{ch.rpm}</span>
                        </div>
                        <div className="p-2 rounded-xl bg-[var(--bg-surface)] border border-[var(--border-decorative)]/40">
                          <span className="text-[10px] text-[var(--text-muted)] block">{t("admin.channels.col_concurrency")}</span>
                          <span className="font-mono font-semibold text-xs text-[var(--text-primary)]">{ch.concurrency}</span>
                        </div>
                      </div>
                    </div>

                    {/* 卡片底部操作栏 */}
                    <div className="p-3 px-4 bg-[var(--bg-subtle)]/40 border-t border-[var(--border-decorative)] flex items-center justify-between gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        icon={<KeyRound className="w-3.5 h-3.5 text-[var(--action-primary)]" />}
                        onClick={() => handleManageKeys(ch)}
                        className="text-xs"
                      >
                        <span>{t("admin.channels.keys_manage")}</span>
                        {boundKeys.length > 0 && (
                          <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-[var(--action-primary)]/10 text-[var(--action-primary)] font-bold">
                            {boundKeys.length}
                          </span>
                        )}
                      </Button>
                      <div className="flex items-center gap-1.5">
                        <Button
                          size="sm"
                          variant="outline"
                          icon={<Edit3 className="w-3.5 h-3.5" />}
                          onClick={() => handleOpenEditModal(ch)}
                        >
                          {t("admin.channels.edit_btn")}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          icon={<Trash2 className="w-3.5 h-3.5 text-[var(--status-danger)]" />}
                          onClick={() => handleDeleteClick(ch)}
                          className="hover:bg-[var(--status-danger)]/10 text-[var(--status-danger)]"
                        />
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ) : (
          /* 表格视图 */
          <div className="flex-1 min-h-0 overflow-auto">
            <table className="w-full min-w-[1080px] text-xs text-left">
              <thead className="bg-[var(--bg-subtle)]/80 text-[var(--text-secondary)] border-b border-[var(--border-decorative)] font-mono uppercase tracking-wider sticky top-0 z-10">
                <tr>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_provider")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_title")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_model")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_endpoint")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_weight")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_rpm")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_concurrency")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_breaker")}</th>
                  <th className="px-4 py-3 whitespace-nowrap">{t("admin.channels.col_status")}</th>
                  <th className="px-4 py-3 text-right whitespace-nowrap">{t("admin.channels.col_actions")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-decorative)] text-[var(--text-secondary)]">
                {paginatedChannels.map((ch) => {
                  const prov = providerMap.get(ch.provider_id);
                  const mdl = ch.model_id ? modelMap.get(ch.model_id) : undefined;
                  const boundKeys = keyMap.get(ch.id) || [];
                  const isCooling = ch.breaker_state === "open" || ch.breaker_state === "cooling";
                  const breakerKey = "admin.dashboard.breaker_" + (ch.breaker_state || "closed");
                  const isCopied = copiedChannelId === ch.id;

                  return (
                    <tr key={ch.id} className="hover:bg-[var(--bg-subtle)]/60 transition-colors">
                      {/* 供应商 */}
                      <td className="px-4 py-3.5 font-semibold text-[var(--text-primary)]">
                        {prov?.title || prov?.code || ch.provider_id}
                      </td>

                      {/* 渠道名称与绑定的 Key 徽章 */}
                      <td className="px-4 py-3.5">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-[var(--text-primary)]">{ch.title}</span>
                          {boundKeys.length > 0 && (
                            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-muted)]">
                              <KeyRound className="w-2.5 h-2.5 text-[var(--action-primary)]" />
                              {boundKeys.length}
                            </span>
                          )}
                        </div>
                      </td>

                      {/* 支持的模型列表 */}
                      <td className="px-4 py-3.5">
                        {ch.models && ch.models.length > 0 ? (
                          <div className="flex flex-wrap gap-1 max-w-xs">
                            {ch.models.slice(0, 3).map((bm) => {
                              const m = modelMap.get(bm.model_id);
                              return (
                                <span
                                  key={bm.id || bm.model_id}
                                  className="px-1.5 py-0.5 rounded text-[11px] font-mono bg-[var(--action-primary)]/10 text-[var(--action-primary)] border border-[var(--action-primary)]/20"
                                  title={bm.upstream_model ? `上游: ${bm.upstream_model}` : undefined}
                                >
                                  {m?.title || bm.upstream_model || bm.model_id}
                                </span>
                              );
                            })}
                            {ch.models.length > 3 && (
                              <span className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-muted)]">
                                +{ch.models.length - 3}
                              </span>
                            )}
                          </div>
                        ) : ch.model_ids && ch.model_ids.length > 0 ? (
                          <div className="flex flex-wrap gap-1 max-w-xs">
                            {ch.model_ids.slice(0, 3).map((mid) => {
                              const m = modelMap.get(mid);
                              return (
                                <span
                                  key={mid}
                                  className="px-1.5 py-0.5 rounded text-[11px] font-mono bg-[var(--action-primary)]/10 text-[var(--action-primary)] border border-[var(--action-primary)]/20"
                                >
                                  {m?.title || mid}
                                </span>
                              );
                            })}
                            {ch.model_ids.length > 3 && (
                              <span className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-muted)]">
                                +{ch.model_ids.length - 3}
                              </span>
                            )}
                          </div>
                        ) : (
                          <div className="font-mono">
                            <div className="text-[var(--text-primary)] font-medium">
                              {mdl?.title || ch.upstream_model || "-"}
                            </div>
                            {ch.upstream_model && (
                              <div className="text-[10px] text-[var(--text-muted)]">
                                {ch.upstream_model}
                              </div>
                            )}
                          </div>
                        )}
                      </td>

                      {/* Base URL */}
                      <td
                        className="px-4 py-3.5 font-mono text-[var(--text-muted)] max-w-xs"
                        title={ch.base_url}
                      >
                        <div className="flex items-center gap-1.5">
                          <span className="truncate">{ch.base_url}</span>
                          <button
                            type="button"
                            onClick={() => handleCopyEndpoint(ch.id, ch.base_url)}
                            className="p-1 hover:text-[var(--action-primary)] text-[var(--text-muted)] transition-colors cursor-pointer shrink-0"
                            title={isCopied ? "已复制" : "复制端点"}
                          >
                            {isCopied ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
                          </button>
                        </div>
                      </td>

                      {/* 权重 */}
                      <td className="px-4 py-3.5 font-mono font-semibold text-[var(--text-primary)]">
                        {ch.weight}
                      </td>

                      {/* RPM 限流 */}
                      <td className="px-4 py-3.5 font-mono">{ch.rpm}</td>

                      {/* 最大并发数 */}
                      <td className="px-4 py-3.5 font-mono">{ch.concurrency}</td>

                      {/* 熔断状态 */}
                      <td className="px-4 py-3.5">
                        <StatusBadge
                          status={isCooling ? "cooling" : "healthy"}
                          label={t(breakerKey)}
                        />
                      </td>

                      {/* 启停状态 */}
                      <td className="px-4 py-3.5">
                        <StatusBadge status={ch.enabled ? "enabled" : "disabled"} />
                      </td>

                      {/* 操作 */}
                      <td className="px-4 py-3.5 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            onClick={() => handleManageKeys(ch)}
                            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--action-primary)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer"
                            title={t("admin.channels.keys_manage")}
                            aria-label={t("admin.channels.keys_manage")}
                          >
                            <KeyRound className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleOpenEditModal(ch)}
                            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--action-primary)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer"
                            title={t("admin.channels.edit_btn")}
                          >
                            <Edit3 className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleDeleteClick(ch)}
                            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--status-danger)] hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer"
                            title={t("admin.channels.delete_btn")}
                            aria-label={t("admin.channels.delete_btn")}
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* 分页组件 - 始终吸底沉在卡片底部 */}
        <AdminPagination
          total={filteredChannels.length}
          page={page}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={(newSize) => {
            setPageSize(newSize);
            setPage(1);
          }}
          pageSizeOptions={[12, 24, 48, 96]}
        />
      </div>

      {/* 现代模型渠道配置与批量注册弹窗 */}
      <ChannelModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        editingChannel={editingChannel}
        providers={providers}
        models={models}
        onSuccess={loadData}
      />

      <ChannelKeysModal
        open={channelForKeys !== null}
        channel={channelForKeys}
        keys={keys}
        onClose={() => setChannelForKeys(null)}
        onSaved={loadData}
      />

      {/* 删除确认对话框 */}
      <ConfirmDialog
        open={deleteConfirmOpen}
        title={t("admin.channels.action_delete")}
        description={<div className="space-y-2"><p>{t("admin.channels.confirm_delete", channelToDelete?.title || "")}</p>{channelToDelete && (keyMap.get(channelToDelete.id)?.length || 0) > 0 && <div role="alert" className="space-y-2 rounded-md border border-[var(--status-warning)]/30 bg-[var(--status-warning)]/10 p-3 text-left"><p>{t("admin.channels.keys_block_delete", String(keyMap.get(channelToDelete.id)?.length || 0))}</p><Button size="sm" variant="outline" onClick={() => { setDeleteConfirmOpen(false); setChannelForKeys(channelToDelete); }}>{t("admin.channels.keys_manage")}</Button></div>}{deleteError && <p role="alert" className="text-[var(--status-danger)]">{deleteError}</p>}</div>}
        confirmText={t("admin.channels.action_delete")}
        cancelText={t("admin.channels.modal_cancel")}
        variant="danger"
        onConfirm={handleExecuteDelete}
        onClose={() => { setDeleteConfirmOpen(false); setDeleteError(null); }}
        loading={deleting}
        confirmDisabled={channelToDelete !== null && (keyMap.get(channelToDelete.id)?.length || 0) > 0}
      />
    </div>
  );
}
