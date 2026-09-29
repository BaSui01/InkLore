/**
 * @inklore/admin-console - 计费规则与每日算力配额面板 (BillingRulesPanel)
 * 严格遵循 Zero Hardcoding Contract、No Emoji Policy 与零硬编码色彩主题
 */

import React, { useState } from "react";
import { Database, Coins, Layers, Save } from "lucide-react";
import { Badge, Button, Input } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { LlmBudget, LlmPricing, LlmModel, updateLlmBudget } from "../api/adminApi";

interface BillingRulesPanelProps {
  budget: LlmBudget | null;
  pricings: LlmPricing[];
  models: LlmModel[];
  onBudgetUpdated: (updated: LlmBudget) => void;
  onSuccess: (msg: string) => void;
}

export function BillingRulesPanel({
  budget,
  pricings,
  models,
  onBudgetUpdated,
  onSuccess,
}: BillingRulesPanelProps) {
  const { t } = useTranslation();

  const [budgetLimitInput, setBudgetLimitInput] = useState(() =>
    budget ? (budget.daily_limit_micros / 1000000).toFixed(2) : ""
  );
  const [budgetSaving, setBudgetSaving] = useState(false);
  const [budgetMessage, setBudgetMessage] = useState<string | null>(null);

  React.useEffect(() => {
    if (budget) {
      setBudgetLimitInput((budget.daily_limit_micros / 1000000).toFixed(2));
    }
  }, [budget]);

  const modelMap = React.useMemo(() => {
    const map = new Map<string, LlmModel>();
    models.forEach((m) => map.set(m.id, m));
    return map;
  }, [models]);

  const handleSaveBudget = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!budget) return;
    const limitNum = parseFloat(budgetLimitInput);
    if (isNaN(limitNum) || limitNum < 0) {
      setBudgetMessage("请输入合法的正数日限额金额。");
      return;
    }
    const micros = Math.round(limitNum * 1000000);
    setBudgetSaving(true);
    setBudgetMessage(null);
    try {
      const updated = await updateLlmBudget({
        daily_limit_micros: micros,
        expected_revision: budget.revision,
      });
      onBudgetUpdated(updated);
      onSuccess(t("admin.users.success_budget_saved"));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setBudgetMessage(msg);
    } finally {
      setBudgetSaving(false);
    }
  };

  const formatDate = (isoStr: string) => {
    if (!isoStr) return "-";
    try {
      const d = new Date(isoStr);
      return d.toLocaleDateString() + " " + d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    } catch {
      return isoStr;
    }
  };

  return (
    <div className="space-y-6">
      {/* 日算力预算卡片 */}
      <div className="p-5 bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-xl shadow-sm space-y-5">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-decorative)] pb-3">
          <div className="space-y-0.5">
            <div className="font-semibold text-sm text-[var(--text-primary)] flex items-center gap-2">
              <Database className="w-4 h-4 text-[var(--action-primary)]" />
              <span>{t("admin.users.budget_card_title")}</span>
            </div>
            <p className="text-xs text-[var(--text-secondary)]">
              {t("admin.users.billing_desc")}
            </p>
          </div>

          <Badge variant={budget?.over_budget ? "danger" : "success"} size="sm" className="font-medium">
            {budget?.over_budget ? t("admin.users.budget_status_over") : t("admin.users.budget_status_normal")}
          </Badge>
        </div>

        {/* 预算关键指标网格 */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="p-3.5 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] space-y-1">
            <div className="text-xs text-[var(--text-secondary)]">{t("admin.users.budget_limit_label")}</div>
            <div className="text-xl font-bold font-mono text-[var(--text-primary)]">
              ￥{budget ? (budget.daily_limit_micros / 1000000).toFixed(2) : "0.00"}
            </div>
          </div>

          <div className="p-3.5 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] space-y-1">
            <div className="text-xs text-[var(--text-secondary)]">{t("admin.users.budget_spent_label")}</div>
            <div className="text-xl font-bold font-mono text-amber-600 dark:text-amber-400">
              ￥{budget ? (budget.spent_micros / 1000000).toFixed(4) : "0.0000"}
            </div>
          </div>

          <div className="p-3.5 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] space-y-1">
            <div className="text-xs text-[var(--text-secondary)]">{t("admin.users.budget_remaining_label")}</div>
            <div className="text-xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
              ￥{budget ? (Math.max(0, budget.daily_limit_micros - budget.spent_micros) / 1000000).toFixed(2) : "0.00"}
            </div>
          </div>

          <div className="p-3.5 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border-decorative)] space-y-1">
            <div className="text-xs text-[var(--text-secondary)]">{t("admin.users.budget_reserved_label")}</div>
            <div className="text-xl font-bold font-mono text-indigo-500">
              ￥{budget ? (budget.reserved_micros / 1000000).toFixed(4) : "0.0000"}
            </div>
          </div>
        </div>

        {/* 预算调整表单 */}
        <form onSubmit={handleSaveBudget} className="pt-2 border-t border-[var(--border-decorative)] flex flex-wrap items-end gap-3">
          <div className="space-y-1.5 flex-1 min-w-[240px] max-w-xs">
            <label htmlFor="billing-budget-limit" className="block text-xs font-semibold text-[var(--text-primary)]">
              {t("admin.users.budget_limit_label")}
            </label>
            <Input
              id="billing-budget-limit"
              type="number"
              step="0.01"
              min="0"
              value={budgetLimitInput}
              onChange={(e) => setBudgetLimitInput(e.target.value)}
              placeholder="如: 100.00"
              className="w-full text-xs font-mono"
              disabled={budgetSaving}
            />
          </div>

          <Button
            type="submit"
            variant="primary"
            size="sm"
            icon={<Save className="w-3.5 h-3.5" />}
            loading={budgetSaving}
          >
            {t("admin.users.btn_save_budget")}
          </Button>

          {budgetMessage && (
            <span className="text-xs text-red-500 font-medium py-1.5">{budgetMessage}</span>
          )}
        </form>
      </div>

      {/* 双层货币解耦机制说明卡片 */}
      <div className="p-5 bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-xl shadow-sm space-y-3">
        <div className="font-semibold text-sm text-[var(--text-primary)] flex items-center gap-2">
          <Coins className="w-4 h-4 text-amber-500" />
          <span>{t("admin.users.dual_currency_title")}</span>
        </div>
        <p className="text-xs leading-relaxed text-[var(--text-secondary)]">
          {t("admin.users.dual_currency_desc")}
        </p>
        <div className="flex flex-wrap items-center gap-4 pt-1 font-mono text-xs">
          <Badge variant="secondary" size="sm" className="gap-1 px-2.5 py-1">
            <span>{t("admin.users.currency_anchor")}</span>
          </Badge>
          <Badge variant="secondary" size="sm" className="gap-1 px-2.5 py-1">
            <span>{t("admin.users.micros_anchor")}</span>
          </Badge>
        </div>
      </div>

      {/* 在册模型生效计费目录 */}
      <div className="bg-[var(--bg-surface)] border border-[var(--border-decorative)] rounded-xl overflow-hidden shadow-sm">
        <div className="p-4 border-b border-[var(--border-decorative)] flex items-center justify-between">
          <div className="font-semibold text-sm text-[var(--text-primary)] flex items-center gap-2">
            <Layers className="w-4 h-4 text-[var(--action-primary)]" />
            <span>{t("admin.users.pricing_catalog_title")}</span>
          </div>
          <Badge variant="secondary" size="sm">
            {pricings.length}
          </Badge>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-xs text-left">
            <thead className="bg-[var(--bg-subtle)] text-[var(--text-secondary)] border-b border-[var(--border-decorative)] font-mono uppercase tracking-wider">
              <tr>
                <th className="p-3.5">{t("admin.users.pricing_col_model")}</th>
                <th className="p-3.5">{t("admin.users.pricing_col_input")}</th>
                <th className="p-3.5">{t("admin.users.pricing_col_output")}</th>
                <th className="p-3.5">{t("admin.users.pricing_col_cache")}</th>
                <th className="p-3.5">{t("admin.users.pricing_currency")}</th>
                <th className="p-3.5">{t("admin.users.pricing_col_effective")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--border-decorative)] text-[var(--text-secondary)]">
              {pricings.length === 0 ? (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-[var(--text-muted)] font-sans">
                    暂无定价规则记录，请在模型配置中发布定价版本
                  </td>
                </tr>
              ) : (
                pricings.map((pr) => {
                  const m = modelMap.get(pr.model_id);
                  return (
                    <tr key={pr.id} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
                      <td className="p-3.5 font-semibold text-[var(--text-primary)]">
                        <div>{m?.title || pr.model_id}</div>
                        {m && <div className="text-[10px] text-[var(--text-muted)] font-mono">{pr.model_id}</div>}
                      </td>
                      <td className="p-3.5 font-mono text-[var(--text-primary)]">
                        ￥{(pr.input_micros_per_million / 1000000).toFixed(2)}
                      </td>
                      <td className="p-3.5 font-mono text-[var(--text-primary)]">
                        ￥{(pr.output_micros_per_million / 1000000).toFixed(2)}
                      </td>
                      <td className="p-3.5 font-mono text-emerald-600 dark:text-emerald-400">
                        ￥{(pr.cache_micros_per_million / 1000000).toFixed(2)}
                      </td>
                      <td className="p-3.5 font-mono">
                        <Badge variant="secondary" size="sm">
                          {pr.currency || "CNY"}
                        </Badge>
                      </td>
                      <td className="p-3.5 font-mono text-[var(--text-muted)]">
                        {formatDate(pr.effective_at || pr.created_at)}
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
  );
}
