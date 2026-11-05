import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { isApiError, type Novel, type UpdateNovelInput } from "@inklore/contracts";
import { useI18n } from "@inklore/i18n";
import { getApiSDK } from "@inklore/transport";
import { Button, ErrorPresenter, Input, Modal } from "@inklore/ui";
import { NovelProgress } from "./NovelProgress";

function beijingDay() {
  const parts = new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date());
  const part = (type: string) => parts.find(value => value.type === type)!.value;
  return `${part("year")}-${part("month")}-${part("day")}`;
}

export function WritingProgress({ novel, onChanged }: { novel: Novel; onChanged: (novel: Novel) => void }) {
  const { t, locale } = useI18n();
  const api = getApiSDK();
  const [day, setDay] = useState(beijingDay);
  const [editing, setEditing] = useState(false);
  useEffect(() => {
    const interval = setInterval(() => setDay(beijingDay()), 30_000);
    return () => clearInterval(interval);
  }, []);
  const statistics = useQuery({
    queryKey: ["content-writing-statistics", api.getSession()?.getScope(), novel.id, novel.revision, day],
    queryFn: ({ signal }) => api.content.writingStatistics(novel.id, day, day, signal),
    retry: false, staleTime: 30_000, refetchInterval: 60_000,
  });
  const denied = isApiError(statistics.error) && [403, 404].includes(statistics.error.httpStatus);
  const today = statistics.data?.days.find(item => item.date === day);
  const number = new Intl.NumberFormat(locale);
  return <section className="rounded-xl border border-[var(--border-decorative)] bg-[var(--bg-surface)] p-4 space-y-3" aria-labelledby="writing-progress-title">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <h2 id="writing-progress-title" className="font-semibold text-sm">{t("writing.title")}</h2>
      {novel.status === "active" && !denied && <Button size="sm" variant="ghost" onClick={() => setEditing(true)}>{t("writing.settings")}</Button>}
    </div>
    {statistics.isPending ? <p role="status">{t("writing.loading")}</p> : statistics.isError ? <div role="alert">
      <p>{t(denied ? "writing.denied" : "writing.failed")}</p>
      <ErrorPresenter error={statistics.error} />
      {!denied && <Button size="sm" onClick={() => void statistics.refetch()}>{t("common.refresh")}</Button>}
    </div> : statistics.data && today ? <>
      <div className="grid gap-3 sm:grid-cols-2">
        <NovelProgress novel={{ ...novel, word_count: statistics.data.current_word_count }} />
        <div className="space-y-1 text-xs" data-testid="today-writing-progress">
          <p>{t("writing.todayAdded", number.format(today.added_words))}</p>
          <p>{t("writing.todayRemoved", number.format(today.removed_words))}</p>
          <p>{t("writing.todayNet", number.format(today.net_words))}</p>
        </div>
      </div>
      {statistics.data.current_word_count === 0 && today.save_count === 0 && <p className="text-xs text-[var(--text-muted)]">{t("writing.empty")}</p>}
    </> : <p role="alert">{t("writing.failed")}</p>}
    <p className="text-xs text-[var(--text-muted)]">{t("writing.basis")}</p>
    {editing && !denied && <WritingGoals novel={novel} onClose={() => setEditing(false)} onSaved={value => { onChanged(value); setEditing(false); }} />}
  </section>;
}

function WritingGoals({ novel, onClose, onSaved }: { novel: Novel; onClose: () => void; onSaved: (novel: Novel) => void }) {
  const { t } = useI18n();
  const api = getApiSDK();
  const [baseline, setBaseline] = useState(novel);
  const [target, setTarget] = useState(novel.target_words === 0 ? "" : String(novel.target_words));
  const [date, setDate] = useState(novel.target_completion_date ?? "");
  const [error, setError] = useState<unknown>(null);
  const [pending, setPending] = useState(false);
  const [invalid, setInvalid] = useState(false);
  const locked = useRef(false);
  const intent = useRef<{ serialized: string; key: string } | null>(null);
  const conflict = isApiError(error) && error.code === "content.version_conflict";
  const denied = isApiError(error) && [403, 404].includes(error.httpStatus);
  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (locked.current || denied || conflict) return;
    const words = target === "" ? 0 : Number(target);
    if (!Number.isSafeInteger(words) || words < 0 || words > 100000000) { setInvalid(true); return; }
    const input: UpdateNovelInput = { expected_revision: baseline.revision, title: baseline.title, summary: baseline.summary,
      category: baseline.category, target_words: words, target_completion_date: date || null };
    const serialized = JSON.stringify(input);
    if (intent.current?.serialized !== serialized) intent.current = { serialized, key: crypto.randomUUID() };
    locked.current = true; setPending(true); setInvalid(false); setError(null);
    try { onSaved(await api.content.updateNovel(novel.id, input, intent.current!.key)); }
    catch (cause) { setError(cause); }
    finally { locked.current = false; setPending(false); }
  };
  const refresh = async () => {
    if (locked.current) return;
    locked.current = true; setPending(true);
    try { setBaseline(await api.content.getNovel(novel.id)); intent.current = null; setError(null); }
    catch (cause) { setError(cause); }
    finally { locked.current = false; setPending(false); }
  };
  return <Modal open title={t("writing.settings")} onClose={() => { if (!locked.current) onClose(); }}>
    <form onSubmit={save} className="space-y-4" aria-busy={pending} data-testid="writing-goals-form">
      <p className="text-sm text-[var(--text-muted)]">{t("writing.optional")}</p>
      <label className="block space-y-1" htmlFor="writing-target-words"><span>{t("writing.targetWords")}</span>
        <Input id="writing-target-words" type="number" min={0} max={100000000} step={1} value={target} disabled={pending || denied} onChange={e => setTarget(e.target.value)} error={invalid ? t("writing.invalid") : undefined} />
      </label>
      <label className="block space-y-1" htmlFor="writing-target-date"><span>{t("writing.targetDate")}</span>
        <Input id="writing-target-date" type="date" min="0001-01-01" max="9999-12-31" value={date} disabled={pending || denied} onChange={e => setDate(e.target.value)} />
      </label>
      {error != null && <ErrorPresenter error={error} />}
      {conflict && <div role="alert"><p>{t("writing.conflict")}</p><Button type="button" onClick={() => void refresh()} disabled={pending}>{t("common.refresh")}</Button></div>}
      {denied && <p role="alert">{t("writing.denied")}</p>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose} disabled={pending}>{t("common.cancel")}</Button>
        <Button type="submit" loading={pending} disabled={denied || conflict}>{t("common.save")}</Button>
      </div>
    </form>
  </Modal>;
}
