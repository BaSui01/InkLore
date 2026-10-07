/**
 * @inklore/admin-console - 大模型调度网关监控大盘
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 * 数据全部来自真实后端 REST API，全面国际化
 */

import { useEffect, useState, useCallback, useMemo } from "react";
import { Activity, Cpu, Database, Layers, Clock, Box } from "lucide-react";
import { Card, Badge } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { useAdminSession } from "../context/AdminSessionContext";
import { AdminHeader } from "../components/AdminHeader";
import { AdminNotice } from "../components/AdminNotice";
import {
  LlmBudget,
  LlmChannel,
  LlmModel,
  LlmProvider,
  LlmUsage,
  fetchLlmBudget,
  fetchLlmChannels,
  fetchLlmModels,
  fetchLlmProviders,
  fetchLlmUsage,
} from "../api/adminApi";

export function LlmDashboardRoute() {
  const { t } = useTranslation();
  const { isAuthenticated } = useAdminSession();

  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [hasLoaded, setHasLoaded] = useState(false);
  const [budget, setBudget] = useState<LlmBudget | null>(null);
  const [channels, setChannels] = useState<LlmChannel[]>([]);
  const [models, setModels] = useState<LlmModel[]>([]);
  const [providers, setProviders] = useState<LlmProvider[]>([]);
  const [usages, setUsages] = useState<LlmUsage[]>([]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [b, c, m, p, u] = await Promise.all([
        fetchLlmBudget(),
        fetchLlmChannels(),
        fetchLlmModels(),
        fetchLlmProviders(),
        fetchLlmUsage(20),
      ]);
      setBudget(b);
      setChannels(c);
      setModels(m);
      setProviders(p);
      setUsages(u);
      setHasLoaded(true);
      setLoadError(null);
    } catch (err) {
      setLoadError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData, isAuthenticated]);

  // 计算动态统计值
  const activeChannelsCount = channels.filter((c) => c.enabled && c.breaker_state !== "open").length;
  const totalChannelsCount = channels.length;

  const totalInputTokens = usages.reduce((acc, cur) => acc + (cur.input_tokens || 0), 0);
  const totalOutputTokens = usages.reduce((acc, cur) => acc + (cur.output_tokens || 0), 0);
  const totalTokensConsumed = totalInputTokens + totalOutputTokens;

  const totalCalls = usages.length;
  const avgDuration = totalCalls > 0 ? Math.round(usages.reduce((acc, cur) => acc + (cur.duration_ms || 0), 0) / totalCalls) : 0;
  const healthRate = totalChannelsCount > 0 ? Math.round((activeChannelsCount / totalChannelsCount) * 100) : 100;

  const enabledChannelsCount = channels.filter((c) => c.enabled).length;
  const currency = budget?.currency || "CNY";
  const totalCost = budget ? budget.spent_micros / 1000000 : 0;
  const dailyLimit = budget ? budget.daily_limit_micros / 1000000 : 0;
  const remainingBudget = budget ? Math.max(0, (budget.daily_limit_micros - budget.spent_micros) / 1000000) : 0;

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

  // 模型能力聚合集合
  const uniqueCapabilities = useMemo(() => {
    const set = new Set<string>();
    models.forEach((m) => {
      if (Array.isArray(m.capabilities)) {
        m.capabilities.forEach((c) => set.add(c));
      }
    });
    return Array.from(set);
  }, [models]);

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminNotice error={loadError} onRetry={() => void loadData()} onDismiss={() => setLoadError(null)} />

      {/* 头部标题栏 */}
      <AdminHeader title={t("admin.dashboard.title")} />

      {loading || hasLoaded ? <>
      {/* 实时网关状态指示条 */}
      <div className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse" />
          <span className="font-semibold text-xs text-[var(--text-primary)]">
            {t("admin.dashboard.realtime_overview")}
          </span>
        </div>
        <div className="flex flex-wrap items-center gap-6 text-xs font-mono">
          <div className="flex items-center gap-2">
            <span className="text-[var(--text-secondary)]">{t("admin.dashboard.health_rate")}:</span>
            <span className="font-bold text-emerald-600 dark:text-emerald-400">{healthRate}%</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-[var(--text-secondary)]">{t("admin.dashboard.avg_latency")}:</span>
            <span className="font-bold text-[var(--text-primary)]">{avgDuration}ms</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-[var(--text-secondary)]">{t("admin.dashboard.total_calls")}:</span>
            <span className="font-bold text-[var(--action-primary)]">{totalCalls}</span>
          </div>
        </div>
      </div>

      {/* 4 大核心 KPI 仪表卡 */}
      <div className="shrink-0 px-6 py-3.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/20 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
        {/* Token Consumption */}
        <Card className="group p-3.5 bg-[var(--bg-surface)] border-[var(--border-decorative)] space-y-2 rounded-lg shadow-xs transition-all">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-[var(--text-secondary)]">
              {t("admin.dashboard.stat_tokens_today")}
            </span>
            <div className="w-7 h-7 rounded-lg bg-[var(--action-primary)]/10 text-[var(--action-primary)] flex items-center justify-center">
              <Activity className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl font-bold font-mono tracking-tight text-[var(--text-primary)]">
            {totalTokensConsumed.toLocaleString()}
          </div>
          <div className="flex items-center justify-between text-[11px] font-mono text-[var(--text-muted)] pt-1.5 border-t border-[var(--border-decorative)]">
            <span>{t("admin.dashboard.col_input_tokens")}: {totalInputTokens.toLocaleString()}</span>
            <span>{t("admin.dashboard.col_output_tokens")}: {totalOutputTokens.toLocaleString()}</span>
          </div>
        </Card>

        {/* Active Channels */}
        <Card className="group p-3.5 bg-[var(--bg-surface)] border-[var(--border-decorative)] space-y-2 rounded-lg shadow-xs transition-all">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-[var(--text-secondary)]">
              {t("admin.dashboard.stat_active_channels")}
            </span>
            <div className="w-7 h-7 rounded-lg bg-emerald-500/10 text-emerald-500 flex items-center justify-center">
              <Layers className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl font-bold font-mono tracking-tight text-[var(--text-primary)] flex items-baseline gap-1.5">
            <span>{enabledChannelsCount}</span>
            <span className="text-xs text-[var(--text-muted)] font-normal">/ {channels.length}</span>
          </div>
          <div className="w-full bg-[var(--bg-subtle)] h-1 rounded-full overflow-hidden">
            <div
              className="bg-emerald-500 h-full transition-all duration-500"
              style={{ width: `${channels.length ? (enabledChannelsCount / channels.length) * 100 : 0}%` }}
            />
          </div>
        </Card>

        {/* Budget Spend */}
        <Card className="group p-3.5 bg-[var(--bg-surface)] border-[var(--border-decorative)] space-y-2 rounded-lg shadow-xs transition-all">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-[var(--text-secondary)]">
              {t("admin.dashboard.stat_budget_spent")}
            </span>
            <div className="w-7 h-7 rounded-lg bg-amber-500/10 text-amber-500 flex items-center justify-center">
              <Database className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl font-bold font-mono tracking-tight text-[var(--text-primary)]">
            {currency} {totalCost.toFixed(4)}
          </div>
          <div className="flex items-center justify-between text-[11px] font-mono text-[var(--text-muted)] pt-1.5 border-t border-[var(--border-decorative)]">
            <span>{t("admin.dashboard.stat_budget_limit")}: {currency} {dailyLimit.toFixed(2)}</span>
            <span>{t("admin.dashboard.stat_budget_remaining")}: {currency} {remainingBudget.toFixed(2)}</span>
          </div>
        </Card>

        {/* Registered Models */}
        <Card className="group p-3.5 bg-[var(--bg-surface)] border-[var(--border-decorative)] space-y-2 rounded-lg shadow-xs transition-all">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-[var(--text-secondary)]">
              {t("admin.dashboard.stat_total_models")}
            </span>
            <div className="w-7 h-7 rounded-lg bg-indigo-500/10 text-indigo-500 flex items-center justify-center">
              <Box className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl font-bold font-mono tracking-tight text-[var(--text-primary)]">
            {models.length}
          </div>
          <div className="flex items-center gap-1.5 overflow-hidden text-[11px] pt-1.5 border-t border-[var(--border-decorative)]">
            {uniqueCapabilities.map((cap) => (
              <Badge key={cap} variant="secondary" size="sm">
                {cap}
              </Badge>
            ))}
          </div>
        </Card>
      </div>

      {/* 下方双栏占满表格区：拓扑健康度与最新调用记录 */}
      <div className="flex-1 min-h-0 grid grid-cols-1 xl:grid-cols-2 divide-y xl:divide-y-0 xl:divide-x divide-[var(--border-decorative)] bg-[var(--bg-surface)]">
        {/* Provider & Channel Topology Table */}
        <div className="flex-1 min-h-0 flex flex-col bg-[var(--bg-surface)] overflow-hidden">
          <div className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/30 flex items-center justify-between">
            <div className="font-semibold text-xs text-[var(--text-primary)] flex items-center gap-2">
              <Cpu className="w-3.5 h-3.5 text-[var(--action-primary)]" />
              <span>{t("admin.dashboard.topology_title")}</span>
            </div>
            <Badge variant="secondary" size="sm">
              {channels.length}
            </Badge>
          </div>

          <div className="flex-1 min-h-0 overflow-auto">
            <table className="w-full text-xs text-left">
              <thead className="bg-[var(--bg-subtle)]/90 text-[var(--text-secondary)] border-b border-[var(--border-decorative)] font-mono uppercase tracking-wider sticky top-0 z-10">
                <tr>
                  <th className="px-6 py-3 whitespace-nowrap">{t("admin.dashboard.col_provider")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_model")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_protocol")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_endpoint")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_weight")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_rpm")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_breaker")}</th>
                  <th className="px-6 py-3 whitespace-nowrap">{t("admin.dashboard.col_status")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-decorative)] text-[var(--text-secondary)]">
                {channels.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="p-16 text-center text-[var(--text-muted)] font-sans">
                      {loading ? t("common.loading") : t("admin.dashboard.empty_channels")}
                    </td>
                  </tr>
                ) : (
                  channels.map((ch) => {
                    const prov = providerMap.get(ch.provider_id);
                    const mdl = ch.model_id ? modelMap.get(ch.model_id) : undefined;
                    const isCooling = ch.breaker_state === "open" || ch.breaker_state === "cooling";
                    const statusVariant = !ch.enabled ? "secondary" : isCooling ? "warning" : "success";
                    const statusLabel = !ch.enabled
                      ? t("admin.dashboard.status_disabled")
                      : isCooling
                      ? t("admin.dashboard.status_cooling")
                      : t("admin.dashboard.status_healthy");

                    const breakerKey = `admin.dashboard.breaker_${ch.breaker_state || "closed"}`;
                    const breakerVariant = ch.breaker_state === "open" ? "danger" : ch.breaker_state === "cooling" ? "warning" : "success";

                    const modelDisplay =
                      ch.models && ch.models.length > 0
                        ? ch.models.map((bm) => modelMap.get(bm.model_id)?.title || bm.upstream_model).filter(Boolean).join(", ")
                        : mdl?.title || ch.upstream_model || mdl?.upstream_name || ch.model_id || "-";

                    return (
                      <tr key={ch.id} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
                        <td className="px-6 py-3 font-semibold text-[var(--text-primary)] whitespace-nowrap">
                          {prov?.title || prov?.code || ch.provider_id}
                        </td>
                        <td className="px-3 py-3 font-mono text-[var(--text-primary)] font-medium whitespace-nowrap" title={modelDisplay}>
                          <span className="max-w-[200px] truncate block">{modelDisplay}</span>
                        </td>
                        <td className="px-3 py-3 whitespace-nowrap">
                          <Badge variant="secondary" size="sm" className="font-mono">
                            {prov?.protocol || t("admin.dashboard.protocol_unknown")}
                          </Badge>
                        </td>
                        <td className="px-3 py-3 font-mono text-[var(--text-muted)] max-w-xs truncate" title={ch.base_url}>
                          {ch.base_url}
                        </td>
                        <td className="px-3 py-3 font-mono font-semibold whitespace-nowrap">{ch.weight}</td>
                        <td className="px-3 py-3 font-mono whitespace-nowrap">{ch.rpm} req/m</td>
                        <td className="px-3 py-3 whitespace-nowrap">
                          <Badge variant={breakerVariant} size="sm">
                            {t(breakerKey)}
                          </Badge>
                        </td>
                        <td className="px-6 py-3 whitespace-nowrap">
                          <Badge variant={statusVariant} size="sm">
                            {statusLabel}
                          </Badge>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Recent Dispatches Table */}
        <div className="flex-1 min-h-0 flex flex-col bg-[var(--bg-surface)] overflow-hidden">
          <div className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/30 flex items-center justify-between">
            <div className="font-semibold text-xs text-[var(--text-primary)] flex items-center gap-2">
              <Clock className="w-3.5 h-3.5 text-[var(--status-success)]" />
              <span>{t("admin.dashboard.recent_usage_title")}</span>
            </div>
            <Badge variant="secondary" size="sm">
              {usages.length}
            </Badge>
          </div>

          <div className="flex-1 min-h-0 overflow-auto">
            <table className="w-full text-xs text-left">
              <thead className="bg-[var(--bg-subtle)]/90 text-[var(--text-secondary)] border-b border-[var(--border-decorative)] font-mono uppercase tracking-wider sticky top-0 z-10">
                <tr>
                  <th className="px-6 py-3 whitespace-nowrap">{t("admin.dashboard.col_invocation_id")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_model")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_input_tokens")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_output_tokens")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_cost")}</th>
                  <th className="px-3 py-3 whitespace-nowrap">{t("admin.dashboard.col_duration")}</th>
                  <th className="px-6 py-3 whitespace-nowrap">{t("admin.dashboard.col_status")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-decorative)] text-[var(--text-secondary)] font-mono">
                {usages.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="p-16 text-center text-[var(--text-muted)] font-sans">
                      {loading ? t("common.loading") : t("admin.dashboard.empty_usage")}
                    </td>
                  </tr>
                ) : (
                  usages.map((u) => {
                    const mdl = modelMap.get(u.model_id);
                    const isSuccess = u.status === "succeeded" || u.status === "success";
                    const isDenied = u.status === "denied";
                    const statusVariant = isSuccess ? "success" : isDenied ? "warning" : "danger";
                    const outcomeKey = `admin.audit.outcome_${u.status.toLowerCase()}`;

                    return (
                      <tr key={u.id} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
                        <td className="px-6 py-3 text-[var(--action-primary)] font-medium whitespace-nowrap">
                          {(u.invocation_id || u.id).slice(0, 8)}...
                        </td>
                        <td className="px-3 py-3 text-[var(--text-primary)] whitespace-nowrap">{mdl?.title || u.model_id}</td>
                        <td className="px-3 py-3 text-[var(--text-muted)] whitespace-nowrap">{u.input_tokens.toLocaleString()}</td>
                        <td className="px-3 py-3 text-[var(--text-muted)] whitespace-nowrap">{u.output_tokens.toLocaleString()}</td>
                        <td className="px-3 py-3 text-[var(--text-primary)] font-semibold whitespace-nowrap">
                          {u.currency} {(u.cost_micros / 1000000).toFixed(6)}
                        </td>
                        <td className="px-3 py-3 text-[var(--text-muted)] whitespace-nowrap">{u.duration_ms}ms</td>
                        <td className="px-6 py-3 whitespace-nowrap">
                          <Badge variant={statusVariant} size="sm">
                            {t(outcomeKey, u.status)}
                          </Badge>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      </> : null}
    </div>
  );
}
