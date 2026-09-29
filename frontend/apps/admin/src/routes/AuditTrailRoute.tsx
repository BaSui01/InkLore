import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Copy, Clock3, Download, FileText, Filter, RefreshCw, Search, ShieldCheck } from "lucide-react";
import { Badge, Button, Input, Modal } from "@inklore/ui";
import { useTranslation } from "@inklore/i18n";
import { useAdminSession } from "../context/AdminSessionContext";
import { AdminHeader } from "../components/AdminHeader";
import { AdminPagination } from "../components/AdminPagination";
import { AdminNotice } from "../components/AdminNotice";
import { AuditEvent, AuditQueryOptions, exportAuditEventsNDJSON, fetchAuditEvents, fetchPlatformMembers } from "../api/adminApi";

// datetime-local is interpreted as a Beijing wall clock, never as the browser's local timezone.
function beijingInputToUTC(value: string): string | null {
  const parts = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
  if (!parts) return null;
  const [, y, m, d, h, minute] = parts.map(Number);
  if (m < 1 || m > 12 || d < 1 || d > 31 || h > 23 || minute > 59) return null;
  const beijingClock = new Date(Date.UTC(y, m - 1, d, h, minute));
  if (beijingClock.getUTCFullYear() !== y || beijingClock.getUTCMonth() !== m - 1 || beijingClock.getUTCDate() !== d) return null;
  return new Date(beijingClock.getTime() - 8 * 60 * 60 * 1000).toISOString().replace(".000Z", "Z");
}

// Only the backend-provided safe_changes are searchable. UUIDs are diagnostics, not human-facing descriptions.
const uuidPattern = /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/gi;

export function AuditTrailRoute() {
  const { t, locale } = useTranslation();
  const { isAuthenticated, snapshot } = useAdminSession();
  const tenantId = snapshot.principal?.tenant_id;
  const requestSerial = useRef(0);
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [hasLoaded, setHasLoaded] = useState(false);
  const [exportError, setExportError] = useState<unknown>(null);
  const [exporting, setExporting] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);
  const [exportFrom, setExportFrom] = useState("");
  const [exportTo, setExportTo] = useState("");
  const [exportRangeError, setExportRangeError] = useState("");
  const [exportComplete, setExportComplete] = useState(false);
  const [memberLookupFailed, setMemberLookupFailed] = useState(false);
  const [memberNames, setMemberNames] = useState<Record<string, string>>({});
  const [pageStarts, setPageStarts] = useState<{ scope: string; cursors: (string | undefined)[] }>({ scope: "", cursors: [undefined] });
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [pageSize, setPageSize] = useState(20);
  const [refreshToken, setRefreshToken] = useState(0);
  const [filterAction, setFilterAction] = useState("");
  const [filterFrom, setFilterFrom] = useState("");
  const [filterTo, setFilterTo] = useState("");
  const [filterError, setFilterError] = useState("");
  const [appliedFilters, setAppliedFilters] = useState<AuditQueryOptions>({});
  const [pageSearch, setPageSearch] = useState("");
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);
  const [copyError, setCopyError] = useState(false);
  const dismissNotice = useCallback(() => {
    setExportError(null);
    setLoadError(null);
    setFilterError("");
    setExportComplete(false);
  }, []);
  const scopeKey = JSON.stringify([tenantId, appliedFilters.action, appliedFilters.from, appliedFilters.to, pageSize]);
  const scopeRef = useRef(scopeKey);
  scopeRef.current = scopeKey;
  const visiblePages = pageStarts.scope === scopeKey ? pageStarts.cursors : [undefined];
  const visibleEvents = pageStarts.scope === scopeKey ? events : [];
  const canContinue = pageStarts.scope === scopeKey && hasMore && !!nextCursor;

  const resolveActor = (event: AuditEvent) => {
    if (!event.actor_id) return t("admin.audit.actor_unidentified");
    return memberNames[event.actor_id] || t("admin.audit.actor_unidentified");
  };

  useEffect(() => {
    if (!isAuthenticated || !tenantId) return;
    let active = true;
    setMemberNames({});
    setMemberLookupFailed(false);
    void (async () => {
      const names: Record<string, string> = {};
      let cursor: string | undefined;
      const seen = new Set<string>();
      try {
        for (;;) {
          const page = await fetchPlatformMembers(100, cursor);
          if (!active) return;
          for (const member of page.items) {
            names[member.user_id] = member.display_name?.trim() || member.email?.trim() || t("admin.audit.actor_unidentified");
          }
          if (!page.has_more || !page.next_cursor) break;
          if (seen.has(page.next_cursor)) throw new Error("Repeated member cursor");
          seen.add(page.next_cursor);
          cursor = page.next_cursor;
        }
        setMemberNames(names);
      } catch {
        // A partial directory must not be mistaken for a complete identity lookup.
        if (active) setMemberLookupFailed(true);
      }
    })();
    return () => { active = false; };
  }, [isAuthenticated, tenantId, refreshToken, t]);

  const loadPage = useCallback(async (cursor: string | undefined, starts: (string | undefined)[], filters: AuditQueryOptions, limit: number, scope: string) => {
    if (scope !== scopeRef.current) return;
    const serial = ++requestSerial.current;
    setLoading(true);
    try {
      const response = await fetchAuditEvents({ ...filters, limit, cursor });
      if (serial !== requestSerial.current || scope !== scopeRef.current) return;
      setEvents(response.items);
      setPageStarts({ scope, cursors: starts });
      setNextCursor(response.next_cursor || null);
      setHasMore(response.has_more && !!response.next_cursor);
      setSelectedEvent(null);
      setHasLoaded(true);
      setLoadError(null);
    } catch (error) {
      if (serial === requestSerial.current && scope === scopeRef.current) setLoadError(error);
    } finally {
      if (serial === requestSerial.current && scope === scopeRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isAuthenticated && tenantId) void loadPage(undefined, [undefined], appliedFilters, pageSize, scopeKey);
    else { requestSerial.current += 1; setEvents([]); setLoading(false); }
    return () => { requestSerial.current += 1; };
  }, [isAuthenticated, tenantId, appliedFilters, pageSize, refreshToken, loadPage, scopeKey]);

  const resetToFirstPage = () => {
    requestSerial.current += 1;
    setPageStarts({ scope: "", cursors: [undefined] });
    setNextCursor(null);
    setHasMore(false);
    setSelectedEvent(null);
    setRefreshToken((previous) => previous + 1);
  };

  const applyFilters = () => {
    const from = filterFrom ? beijingInputToUTC(filterFrom) : undefined;
    const to = filterTo ? beijingInputToUTC(filterTo) : undefined;
    if ((filterFrom && !from) || (filterTo && !to) || (from && to && from > to)) {
      setFilterError(t("admin.audit.invalid_time_range"));
      return;
    }
    setFilterError("");
    requestSerial.current += 1;
    setEvents([]);
    setPageSearch("");
    setPageStarts({ scope: "", cursors: [undefined] });
    setNextCursor(null);
    setHasMore(false);
    setSelectedEvent(null);
    setAppliedFilters({ action: filterAction.trim() || undefined, from: from || undefined, to: to || undefined });
    setRefreshToken((previous) => previous + 1);
  };

  const changePageSize = (value: number) => {
    requestSerial.current += 1;
    setEvents([]);
    setPageSearch("");
    setPageStarts({ scope: "", cursors: [undefined] });
    setNextCursor(null);
    setHasMore(false);
    setSelectedEvent(null);
    setPageSize(value);
  };

  const openExport = () => {
    const now = new Date();
    const asBeijingInput = (utc: string) => new Date(new Date(utc).getTime() + 8 * 60 * 60 * 1000).toISOString().slice(0, 16);
    setExportFrom(asBeijingInput(appliedFilters.from || new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString()));
    setExportTo(asBeijingInput(appliedFilters.to || now.toISOString()));
    setExportRangeError("");
    setExportError(null);
    setExportComplete(false);
    setExportOpen(true);
  };

  const exportEvents = async () => {
    const from = beijingInputToUTC(exportFrom);
    const to = beijingInputToUTC(exportTo);
    if (!from || !to) { setExportRangeError(t("admin.audit.export_range_required")); return; }
    if (from > to) { setExportRangeError(t("admin.audit.invalid_time_range")); return; }
    if (new Date(to).getTime() - new Date(from).getTime() > 31 * 24 * 60 * 60 * 1000) {
      setExportRangeError(t("admin.audit.export_range_too_large"));
      return;
    }
    setExportRangeError("");
    setExportError(null);
    setExporting(true);
    try {
      const ndjson = await exportAuditEventsNDJSON({ action: appliedFilters.action, from, to });
      const url = URL.createObjectURL(new Blob([ndjson], { type: "application/x-ndjson;charset=utf-8" }));
      const link = document.createElement("a");
      link.href = url;
      link.download = `audit_events_${Date.now()}.ndjson`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      setExportOpen(false);
      setExportComplete(true);
    } catch (error) {
      setExportError(error);
    } finally {
      setExporting(false);
    }
  };

  const formatTime = (value: string) => {
    const time = new Date(value);
    if (Number.isNaN(time.getTime())) return "-";
    return new Intl.DateTimeFormat(locale, {
      timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false,
    }).format(time) + " CST";
  };

  const actionLabel = (action: string) => {
    const aliases: Record<string, string> = {
      "auth.bootstrap": "auth_bootstrap",
      "llm.catalog.mutation": "llm_catalog_mutation",
      "budget.configure": "budget_configure",
      "llm.budget_configured": "llm_budget_configured",
      "auth.password_changed": "auth_password_change",
      "auth.password_change": "auth_password_change",
    };
    const name = aliases[action] || action.replace(/[.:-]/g, "_");
    const key = `admin.audit.action_${name}`;
    const translated = t(key);
    return translated !== key ? translated : t("admin.audit.action_other");
  };

  const outcomeLabel = (value: string) => {
    const normalized = value.toLowerCase() === "success" ? "succeeded" : value.toLowerCase();
    const key = `admin.audit.outcome_${normalized}`;
    const translated = t(key);
    return translated !== key ? translated : t("admin.audit.outcome_other");
  };

  const outcomeVariant = (value: string): "success" | "warning" | "danger" | "secondary" => {
    if (["succeeded", "success"].includes(value.toLowerCase())) return "success";
    if (value.toLowerCase() === "denied") return "warning";
    if (["failed", "error"].includes(value.toLowerCase())) return "danger";
    return "secondary";
  };

  const safeValue = (value?: string | null) => value?.replace(uuidPattern, t("admin.audit.redacted_identifier")) || "-";
  const resourceLabel = (event: AuditEvent) => {
    const action = event.action.toLowerCase();
    if (action.startsWith("auth.")) return t("admin.audit.resource_account");
    if (action.startsWith("content.novel.")) return t("admin.audit.resource_novel");
    if (action.includes("member") || action.includes("session")) return t("admin.audit.resource_member");
    if (action.startsWith("llm.") || action.startsWith("budget.") || action.includes("config")) return t("admin.audit.resource_configuration");
    if (action.startsWith("agent.") || action.includes("run")) return t("admin.audit.resource_runtime");
    return t("admin.audit.resource_other");
  };
  const filteredEvents = useMemo(() => {
    const needle = pageSearch.trim().toLocaleLowerCase(locale);
    if (!needle) return visibleEvents;
    return visibleEvents.filter((event) => [
      safeValue(resolveActor(event)), actionLabel(event.action), outcomeLabel(event.outcome), resourceLabel(event),
      ...(event.safe_changes || []).flatMap((change) => [safeValue(change.field), safeValue(change.before), safeValue(change.after)]),
    ].some((value) => value.toLocaleLowerCase(locale).includes(needle)));
  }, [visibleEvents, pageSearch, memberNames, locale, t]);

  const diagnosticFields = selectedEvent ? [
    ["admin.audit.detail_id", selectedEvent.id],
    ["admin.audit.detail_trace", selectedEvent.trace_id],
    ["admin.audit.detail_request", selectedEvent.request_id],
    ["admin.audit.detail_resource", selectedEvent.resource_id],
    ["admin.audit.detail_time_utc", selectedEvent.created_at],
    ["admin.audit.detail_action", selectedEvent.action],
  ] as const : [];

  return (
    <div className="flex-1 flex flex-col h-full min-h-0 w-full overflow-hidden bg-[var(--bg-surface)] text-[var(--text-primary)]">
      <AdminNotice
        error={exportError ?? loadError ?? filterError ?? null}
        success={exportComplete ? t("admin.audit.export_complete") : null}
        onRetry={exportError == null && loadError != null ? resetToFirstPage : undefined}
        onDismiss={dismissNotice}
      />
      <AdminHeader
        title={t("admin.audit.title")}
        badge={<Badge size="sm" variant="secondary"><ShieldCheck className="w-3.5 h-3.5 mr-1" />{t("admin.audit.read_only")}</Badge>}
        actions={<>
          <Button size="sm" variant="outline" icon={<Download className="w-4 h-4" />} onClick={openExport}>{t("admin.audit.btn_export")}</Button>
          <Button size="sm" variant="outline" icon={<RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />} onClick={resetToFirstPage}>{t("admin.audit.btn_refresh")}</Button>
        </>}
        className="px-6 py-3.5 bg-[var(--bg-surface)] border-b border-[var(--border-decorative)]"
      />

      <div className="shrink-0 px-6 py-2.5 border-b border-[var(--border-decorative)] bg-[var(--bg-subtle)]/40 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3 flex-1 min-w-[320px]">
          <div className="w-48 max-w-xs">
            <Input value={filterAction} onChange={(event) => setFilterAction(event.target.value)} placeholder={t("admin.audit.filter_action_all")} />
          </div>
          <div className="w-44">
            <input type="datetime-local" value={filterFrom} onChange={(event) => setFilterFrom(event.target.value)} className="w-full h-8 px-2 text-xs rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] cursor-pointer" />
          </div>
          <div className="w-44">
            <input type="datetime-local" value={filterTo} onChange={(event) => setFilterTo(event.target.value)} className="w-full h-8 px-2 text-xs rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--action-primary)] cursor-pointer" />
          </div>
          <Button type="button" size="sm" variant="primary" onClick={applyFilters}>{t("admin.audit.apply_filters")}</Button>
          {filterError && <span className="text-xs text-[var(--status-danger)]">{filterError}</span>}
        </div>
        <div className="w-64">
          <Input value={pageSearch} onChange={(event) => setPageSearch(event.target.value)} placeholder={t("admin.audit.search_current_page")} prefixIcon={<Search className="w-4 h-4" />} aria-label={t("admin.audit.search_current_page")} />
        </div>
      </div>

      <div className="flex-1 min-h-0 overflow-auto bg-[var(--bg-surface)]" aria-busy={loading}>
        <table className="w-full min-w-[760px] text-left text-xs">
          <thead className="bg-[var(--bg-subtle)]/90 text-[var(--text-secondary)] text-xs border-b border-[var(--border-decorative)] sticky top-0 z-10 font-mono">
            <tr>
              <th scope="col" className="px-6 py-3 font-medium whitespace-nowrap">{t("admin.audit.col_action")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.audit.col_resource")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.audit.col_actor")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.audit.col_time")}</th>
              <th scope="col" className="px-4 py-3 font-medium whitespace-nowrap">{t("admin.audit.col_status")}</th>
              <th scope="col" className="px-6 py-3 font-medium whitespace-nowrap">{t("admin.audit.detail_title")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border-decorative)]">
            {filteredEvents.map((event) => (
              <tr key={event.id} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
                <td className="px-6 py-3.5 font-medium text-[var(--text-primary)]">{actionLabel(event.action)}</td>
                <td className="px-4 py-3.5 text-[var(--text-secondary)]">{resourceLabel(event)}</td>
                <td className="px-4 py-3.5 text-[var(--text-secondary)] max-w-[190px] truncate" title={safeValue(resolveActor(event))}>{safeValue(resolveActor(event))}</td>
                <td className="px-4 py-3.5 text-[var(--text-secondary)] whitespace-nowrap tabular-nums">{formatTime(event.created_at)}</td>
                <td className="px-4 py-3.5"><Badge variant={outcomeVariant(event.outcome)} size="sm">{outcomeLabel(event.outcome)}</Badge></td>
                <td className="px-6 py-3.5"><Button size="sm" variant="outline" onClick={() => { setCopyError(false); setSelectedEvent(event); }}>{t("admin.audit.view_details")}</Button></td>
              </tr>
            ))}
            {filteredEvents.length === 0 && <tr><td colSpan={6} className="px-6 py-28 text-center text-[var(--text-muted)]">
              {loading ? t("common.loading") : loadError ? t("admin.audit.load_failed") : visibleEvents.length ? t("admin.audit.no_matches_on_page") : t("admin.audit.empty")}
            </td></tr>}
          </tbody>
        </table>
      </div>

      <AdminPagination
        page={visiblePages.length}
        pageSize={pageSize}
        onPageSizeChange={changePageSize}
        pageSizeOptions={[10, 20, 50]}
        hasMore={canContinue && !!nextCursor}
        hasPrev={visiblePages.length > 1}
        loading={loading}
        onPrevPage={() => void loadPage(visiblePages[visiblePages.length - 2], visiblePages.slice(0, -1), appliedFilters, pageSize, scopeKey)}
        onNextPage={() => nextCursor && void loadPage(nextCursor, [...visiblePages, nextCursor], appliedFilters, pageSize, scopeKey)}
        className="px-6 py-2 bg-[var(--bg-surface)] border-t border-[var(--border-decorative)] shrink-0"
      />
      <Modal
        open={exportOpen}
        onClose={() => { if (!exporting) setExportOpen(false); }}
        title={<span className="flex items-center gap-2"><Download className="w-5 h-5 text-[var(--action-primary)]" />{t("admin.audit.export_title")}</span>}
        maxWidth="lg"
        footer={<div className="flex justify-end gap-2">
          <Button size="sm" variant="outline" disabled={exporting} onClick={() => setExportOpen(false)}>{t("admin.audit.btn_close")}</Button>
          <Button size="sm" variant="primary" loading={exporting} onClick={() => void exportEvents()}>{t("admin.audit.btn_export")}</Button>
        </div>}
      >
        <div className="space-y-4 text-sm">
          <p className="text-[var(--text-secondary)]">{t("admin.audit.export_description")}</p>
          <p className="text-xs text-[var(--text-muted)]">{t("admin.audit.export_limit_hint")}</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="space-y-1 text-[var(--text-secondary)]"><span>{t("admin.audit.filter_from")} · CST</span>
              <input type="datetime-local" value={exportFrom} onChange={(event) => setExportFrom(event.target.value)} disabled={exporting} className="w-full h-9 px-3 rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)]" />
            </label>
            <label className="space-y-1 text-[var(--text-secondary)]"><span>{t("admin.audit.filter_to")} · CST</span>
              <input type="datetime-local" value={exportTo} onChange={(event) => setExportTo(event.target.value)} disabled={exporting} className="w-full h-9 px-3 rounded-lg bg-[var(--bg-canvas)] border border-[var(--border-decorative)] text-[var(--text-primary)]" />
            </label>
          </div>
          {exportRangeError && <p role="alert" className="text-[var(--status-danger)]">{exportRangeError}</p>}
        </div>
      </Modal>
      <Modal
        open={!!selectedEvent}
        onClose={() => setSelectedEvent(null)}
        title={<span className="flex items-center gap-2"><FileText className="w-5 h-5 text-[var(--action-primary)]" />{t("admin.audit.detail_title")}</span>}
        maxWidth="2xl"
        footer={<Button size="sm" variant="outline" onClick={() => setSelectedEvent(null)}>{t("admin.audit.btn_close")}</Button>}
      >
        {selectedEvent && <div className="space-y-5 text-sm max-h-[70vh] overflow-y-auto pr-1">
          <div className="rounded-xl bg-[var(--bg-subtle)] p-4 border border-[var(--border-decorative)] space-y-3">
            <div className="flex flex-wrap items-center gap-2"><span className="font-semibold text-[var(--text-primary)]">{actionLabel(selectedEvent.action)}</span><Badge size="sm" variant={outcomeVariant(selectedEvent.outcome)}>{outcomeLabel(selectedEvent.outcome)}</Badge></div>
            <p className="text-[var(--text-secondary)]">{t("admin.audit.detail_actor")}: <span className="text-[var(--text-primary)] font-medium">{resolveActor(selectedEvent)}</span></p>
            <p className="text-xs text-[var(--text-muted)]">{formatTime(selectedEvent.created_at)}</p>
          </div>
          <div className="space-y-2">
            <h4 className="font-semibold text-[var(--text-primary)]">{t("admin.audit.detail_safe_changes")} <span className="text-[var(--text-muted)] font-normal">({selectedEvent.safe_changes?.length || 0})</span></h4>
            {!selectedEvent.safe_changes?.length ? <div className="rounded-xl bg-[var(--bg-subtle)] p-4 text-[var(--text-muted)]">{t("admin.audit.detail_no_changes")}</div> :
              <div className="overflow-x-auto rounded-xl border border-[var(--border-decorative)]"><table className="w-full min-w-[430px] text-left text-xs">
                <thead className="bg-[var(--bg-subtle)] text-[var(--text-secondary)]"><tr><th className="p-3">{t("admin.audit.detail_change_field")}</th><th className="p-3">{t("admin.audit.detail_change_before")}</th><th className="p-3">{t("admin.audit.detail_change_after")}</th></tr></thead>
                <tbody className="divide-y divide-[var(--border-decorative)]">{selectedEvent.safe_changes.map((change, index) => <tr key={index}><td className="p-3 text-[var(--text-primary)] break-words">{safeValue(change.field)}</td><td className="p-3 text-[var(--text-secondary)] break-all">{safeValue(change.before)}</td><td className="p-3 text-[var(--text-secondary)] break-all">{safeValue(change.after)}</td></tr>)}</tbody>
              </table></div>}
          </div>
          <details className="rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-canvas)] p-4">
            <summary className="cursor-pointer font-medium text-[var(--text-secondary)]">{t("admin.audit.diagnostics_title")}</summary>
            <div className="mt-4 space-y-3">{diagnosticFields.filter(([, value]) => !!value).map(([label, value]) => <div key={label} className="flex items-start justify-between gap-3 border-b border-[var(--border-decorative)] pb-2">
              <div className="min-w-0"><span className="block text-xs text-[var(--text-muted)]">{t(label)}</span><code className="block break-all select-all text-xs text-[var(--text-secondary)]">{value}</code></div>
              <button type="button" aria-label={`${t("common.copy")} ${t(label)}`} className="shrink-0 text-[var(--action-primary)] p-1" onClick={() => {
                if (!navigator.clipboard) { setCopyError(true); return; }
                void navigator.clipboard.writeText(value || "").then(() => setCopyError(false)).catch(() => setCopyError(true));
              }}><Copy className="w-4 h-4" /></button>
            </div>)}
              {copyError && <p role="alert" className="text-xs text-[var(--status-danger)]">{t("admin.audit.copy_failed")}</p>}
            </div>
          </details>
        </div>}
      </Modal>
    </div>
  );
}
