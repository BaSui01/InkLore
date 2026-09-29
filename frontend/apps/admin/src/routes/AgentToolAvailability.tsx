import { Wrench } from "lucide-react";
import { useTranslation } from "@inklore/i18n";
import type { AgentBuiltinToolView } from "../api/adminApi";

interface AgentToolAvailabilityProps {
  tools: AgentBuiltinToolView[];
  loadedOnce: boolean;
  loading: boolean;
  searchQuery: string;
}

export function AgentToolAvailability({ tools, loadedOnce, loading, searchQuery }: AgentToolAvailabilityProps) {
  const { t } = useTranslation();
  const query = searchQuery.trim().toLowerCase();
  const visibleTools = query
    ? tools.filter((tool) => [tool.title, tool.tool_key, tool.description].some((value) => value.toLowerCase().includes(query)))
    : tools;

  return (
    <div className="flex-1 min-h-0 overflow-auto p-6" role="tabpanel">
      {!loadedOnce || (loading && !tools.length) ? (
        <p role="status" className="p-8 text-center text-sm text-[var(--text-secondary)]">
          {loading ? t("common.loading") : t("admin.agents.load_unavailable")}
        </p>
      ) : visibleTools.length === 0 ? (
        <div role="status" className="rounded-2xl border border-[var(--border-decorative)] bg-[var(--bg-surface)] p-10 text-center space-y-3">
          <Wrench className="mx-auto w-7 h-7 text-[var(--text-muted)]" aria-hidden="true" />
          <h3 className="text-base font-bold text-[var(--text-primary)]">
            {query ? t("admin.agents.search_empty") : t("admin.agents.empty_builtin_tools")}
          </h3>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {visibleTools.map((tool) => (
            <article key={tool.tool_key} className="rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-surface)] p-5 space-y-2">
              <div className="flex items-start gap-2 justify-between">
                <h3 className="font-bold text-[var(--text-primary)]">{tool.title}</h3>
                <span className={`text-xs font-medium ${tool.execution_status === "available" ? "text-[var(--status-success)]" : "text-[var(--text-muted)]"}`}>
                  {t(tool.execution_status === "available"
                    ? "admin.agents.executable_status"
                    : tool.execution_status === "not_bound"
                      ? "admin.agents.tool_not_bound"
                      : "admin.agents.tool_runtime_status_unknown")}
                </span>
              </div>
              {tool.description && <p className="text-sm text-[var(--text-secondary)] whitespace-pre-wrap">{tool.description}</p>}
            </article>
          ))}
        </div>
      )}
    </div>
  );
}

