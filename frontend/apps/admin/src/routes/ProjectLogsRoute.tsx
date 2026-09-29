import { useEffect, useRef, useState, type FormEvent } from "react";
import { ClipboardCopy, FileText, RefreshCw } from "lucide-react";
import { Badge, Button, ErrorPresenter, Input } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { fetchProjectLogs, type ProjectLog } from "../api/adminApi";
import { AdminHeader } from "../components/AdminHeader";
import { AdminPagination } from "../components/AdminPagination";
import { useAdminSession } from "../context/AdminSessionContext";

type LogFilters = { level?: string; search?: string };

// Only codes and bounded diagnostic identifiers are shown; never render raw log messages or URLs.
const uuidPattern = /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/gi;
function safeCode(value: string | null | undefined, fallback: string): string {
  const code = typeof value === "string" ? value.trim() : "";
  return code.length <= 96 && /^[a-z][a-z0-9_.:-]*$/i.test(code) ? code.replace(uuidPattern, fallback) : fallback;
}

function safeOperation(value: string | null | undefined, fallback: string): string {
  if (typeof value !== "string" || value.length > 180) return fallback;
  return /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS) \/v1\/[a-z0-9-]+(?:\/(?:[a-z0-9-]+|:[a-z_]+))*$/.test(value) ? value : fallback;
}

function ProjectLogsContent() {
  const { t, locale } = useTranslation();
  const requestSerial = useRef(0);
  const [entries, setEntries] = useState<ProjectLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [copyErrorId, setCopyErrorId] = useState<string | null>(null);
  const [level, setLevel] = useState("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<LogFilters>({});
  // Each element is the server-issued start cursor of one page; no synthetic offset/total.
  const [pageStarts, setPageStarts] = useState<(string | undefined)[]>([undefined]);
  const [pageSize, setPageSize] = useState(20);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [refreshToken, setRefreshToken] = useState(0);

  useEffect(() => {
    const serial = ++requestSerial.current;
    setLoading(true);
    setLoadError(null);
    setEntries([]);
    setNextCursor(null);
    setHasMore(false);
    setCopyErrorId(null);
    void (async () => {
      try {
        const page = await fetchProjectLogs({ ...filters, cursor: pageStarts[pageStarts.length - 1], limit: pageSize });
        if (serial !== requestSerial.current) return;
        setEntries(page.items);
        setNextCursor(page.next_cursor || null);
        setHasMore(page.has_more && !!page.next_cursor);
      } catch (error) {
        if (serial === requestSerial.current) setLoadError(error);
      } finally {
        if (serial === requestSerial.current) setLoading(false);
      }
    })();
    return () => { requestSerial.current += 1; };
  }, [filters, pageStarts, pageSize, refreshToken]);

  const resetPage = () => {
    setPageStarts([undefined]);
    setRefreshToken((previous) => previous + 1);
  };

  const applyFilters = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFilters({ level: level || undefined, search: Array.from(search.trim()).slice(0, 200).join("") || undefined });
    resetPage();
  };

  const formatTime = (value: string) => {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return t("admin.project_logs.unknown_time");
    return `${new Intl.DateTimeFormat(locale, {
      timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false,
    }).format(date)} ${t("admin.project_logs.timezone")}`;
  };

  const levelVariant = (value: string): "danger" | "warning" | "secondary" => {
    if (value.toUpperCase() === "ERROR") return "danger";
    if (value.toUpperCase() === "WARN") return "warning";
    return "secondary";
  };

  const levelLabel = (value: string) => {
    const code = safeCode(value, t("admin.project_logs.not_available"));
    const key = `admin.project_logs.level_${code.toLowerCase()}`;
    const translated = t(key);
    return translated === key ? code : translated;
  };

  const eventLabel = (value: string) => {
    const code = safeCode(value, t("admin.project_logs.not_available"));
    const key = `admin.project_logs.event_${code.replace(/[.:-]/g, "_").toLowerCase()}`;
    const translated = t(key);
    // An unknown event is a machine code, not a guessed human description.
    return translated === key ? code : translated;
  };

  const copyDiagnostic = (rowId: string, value: string) => {
    if (!navigator.clipboard) { setCopyErrorId(rowId); return; }
    void navigator.clipboard.writeText(value).then(() => setCopyErrorId(null)).catch(() => setCopyErrorId(rowId));
  };

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminHeader
        title={t("admin.project_logs.title")}
        badge={<Badge size="sm" variant="secondary"><FileText className="w-3.5 h-3.5 mr-1" />{t("admin.project_logs.read_only")}</Badge>}
        actions={<Button size="sm" variant="outline" icon={<RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />} onClick={resetPage}>{t("admin.project_logs.refresh")}</Button>}
        className="px-6 py-3.5 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)]"
      />
      <form onSubmit={applyFilters} className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 flex flex-wrap items-center justify-between gap-3" aria-label={t("admin.project_logs.filters_title")}>
        <div className="flex flex-wrap items-center gap-3 flex-1 min-w-[280px]">
          <div className="w-72 max-w-sm">
            <Input value={search} maxLength={200} onChange={(event) => setSearch(event.target.value)} placeholder={t("admin.project_logs.filter_search_placeholder")} />
          </div>
          <div className="w-36">
            <select value={level} onChange={(event) => setLevel(event.target.value)} className="w-full h-8 px-2.5 text-xs rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] cursor-pointer">
              <option value="">{t("admin.project_logs.all_levels")}</option>
              {["DEBUG", "INFO", "WARN", "ERROR"].map((option) => <option key={option} value={option}>{levelLabel(option)}</option>)}
            </select>
          </div>
          <Button type="submit" size="sm" variant="primary">{t("admin.project_logs.apply_filters")}</Button>
        </div>
      </form>

      {loadError != null && (
        <div className="shrink-0 px-6 py-3 bg-red-500/5 border-b border-[var(--border-decorative)]">
          <ErrorPresenter error={loadError} onRetry={() => setRefreshToken((previous) => previous + 1)} />
        </div>
      )}

      <div className="flex-1 min-h-0 overflow-auto bg-[var(--bg-surface)]" aria-busy={loading}>
        <table className="w-full min-w-[850px] text-left text-xs">
          <thead className="bg-[var(--bg-subtle)]/90 text-[var(--text-secondary)] text-xs border-b border-[var(--border-decorative)] sticky top-0 z-10 font-mono">
            <tr>
              <th scope="col" className="px-6 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_time")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_level")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_event")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_operation")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_status")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_duration")}</th>
              <th scope="col" className="px-6 py-3 font-medium whitespace-nowrap">{t("admin.project_logs.col_diagnostics")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border-decorative)]">
            {entries.map((entry) => {
              const diagnostics = [
                ["admin.project_logs.detail_id", entry.id],
                ["admin.project_logs.detail_request", entry.request_id],
                ["admin.project_logs.detail_trace", entry.trace_id],
                ["admin.project_logs.detail_time_utc", entry.created_at],
              ] as const;
              return <tr key={entry.id} className="hover:bg-[var(--bg-subtle)]/50 align-top transition-colors">
                <td className="px-6 py-3.5 whitespace-nowrap text-[var(--text-secondary)] tabular-nums">{formatTime(entry.created_at)}</td>
                <td className="px-4 py-3.5"><Badge size="sm" variant={levelVariant(entry.level)}>{levelLabel(entry.level)}</Badge></td>
                <td className="px-4 py-3.5 font-medium break-words max-w-[200px]">{eventLabel(entry.event)}</td>
                <td className="px-4 py-3.5 text-[var(--text-secondary)] break-words max-w-[240px]">
                  <span>{safeOperation(entry.operation, t("admin.project_logs.not_available"))}</span>
                </td>
                <td className="px-4 py-3.5 text-xs text-[var(--text-secondary)] tabular-nums whitespace-nowrap">
                  {entry.status != null && Number.isInteger(entry.status) && entry.status >= 100 && entry.status <= 599 ? entry.status : t("admin.project_logs.not_available")}
                  {entry.error_code && entry.error_code !== "none" && <span className="block break-all text-[var(--status-danger)] font-mono">{safeCode(entry.error_code, t("admin.project_logs.not_available"))}</span>}
                </td>
                <td className="px-4 py-3.5 text-xs tabular-nums whitespace-nowrap text-[var(--text-secondary)]">
                  {entry.duration_ms != null && Number.isFinite(entry.duration_ms) && entry.duration_ms >= 0 ? t("admin.project_logs.duration_value", entry.duration_ms) : t("admin.project_logs.not_available")}
                </td>
                <td className="px-6 py-3.5 text-xs">
                  <details className="max-w-[230px]">
                    <summary className="cursor-pointer text-[var(--action-primary)] font-medium">{t("admin.project_logs.diagnostics_title")}</summary>
                    <div className="space-y-2 mt-2.5">
                      {diagnostics.filter(([, value]) => typeof value === "string" && value.length > 0).map(([label, value]) => (
                        <div key={label} className="flex gap-2 items-start text-xs">
                          <div className="min-w-0">
                            <span className="block font-medium text-[var(--text-secondary)]">{t(label)}</span>
                            <code className="block break-all select-all text-[var(--text-primary)] font-mono">{value}</code>
                          </div>
<button type="button" className="p-1 shrink-0 text-[var(--action-primary)] hover:opacity-80 transition-opacity cursor-pointer" aria-label={`${t("admin.project_logs.copy")} ${t(label)}`} onClick={() => copyDiagnostic(entry.id, value!)}><ClipboardCopy className="w-3.5 h-3.5" /></button>
                        </div>
                      ))}
                      {copyErrorId === entry.id && <p role="alert" className="text-[var(--status-danger)]">{t("admin.project_logs.copy_failed")}</p>}
                    </div>
                  </details>
                </td>
              </tr>;
            })}
            {entries.length === 0 && <tr><td colSpan={7} className="px-6 py-28 text-center text-[var(--text-muted)]">
              {loading ? t("admin.project_logs.loading") : loadError ? t("admin.project_logs.load_failed") : t("admin.project_logs.empty")}
            </td></tr>}
          </tbody>
        </table>
      </div>

      <AdminPagination
        page={pageStarts.length}
        pageSize={pageSize}
        onPageSizeChange={(newSize) => {
          setPageSize(newSize);
          resetPage();
        }}
        pageSizeOptions={[10, 20, 50, 100]}
        hasMore={hasMore && !!nextCursor}
        hasPrev={pageStarts.length > 1}
        loading={loading || !!loadError}
        onPrevPage={() => setPageStarts((starts) => starts.slice(0, -1))}
        onNextPage={() => nextCursor && setPageStarts((starts) => [...starts, nextCursor])}
        className="px-6 py-2 bg-[var(--bg-surface)] border-t border-[var(--border-decorative)] shrink-0"
      />
    </div>
  );
}

export function ProjectLogsRoute() {
  const { isAuthenticated, snapshot } = useAdminSession();
  const tenantId = snapshot.principal?.tenant_id;
  if (!isAuthenticated || !tenantId) return null;
  // Remount on tenant switch: opaque cursors and cached results never cross tenant boundaries.
  return <ProjectLogsContent key={tenantId} />;
}
