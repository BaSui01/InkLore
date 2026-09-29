/**
 * @inklore/admin-console - 渠道治理路由组件 (Channels & Keys)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 接入真实后端 /v1/llm/channels 与 /v1/llm/keys 接口
 */

import { useState, useEffect, useCallback, useMemo } from "react";
import { Layers, Plus, RefreshCw, Trash2, Edit3, KeyRound } from "lucide-react";
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

  // Pagination state
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

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
        ) : (
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
                        className="px-4 py-3.5 font-mono text-[var(--text-muted)] max-w-xs truncate"
                        title={ch.base_url}
                      >
                        <span className="truncate">{ch.base_url}</span>
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
          pageSizeOptions={[10, 20, 50, 100]}
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
