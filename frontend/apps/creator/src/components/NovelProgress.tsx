import type { Novel } from "@inklore/contracts";
import { useI18n } from "@inklore/i18n";

/** A capped visual track with an uncapped, accessible completion percentage. */
export function NovelProgress({ novel }: { novel: Novel }) {
  const { t, locale } = useI18n();
  const number = new Intl.NumberFormat(locale);
  const percent = novel.target_words > 0 ? novel.word_count / novel.target_words * 100 : null;
  return <div className="space-y-1 text-xs" data-testid="novel-writing-progress">
    <p>{t("writing.total", number.format(novel.word_count))}</p>
    {percent !== null && <>
      <p>{t("writing.completion", new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(percent), number.format(novel.target_words))}</p>
      <progress className="w-full h-2 accent-[var(--action-primary)]" value={Math.min(novel.word_count, novel.target_words)} max={novel.target_words}
        aria-label={t("writing.goalProgress")} aria-valuetext={t("writing.percent", String(percent))} />
      {percent > 100 && <p className="font-semibold">{t("writing.exceeded", number.format(novel.word_count - novel.target_words))}</p>}
    </>}
    {novel.target_completion_date && <p>{t("writing.deadline", novel.target_completion_date)}</p>}
  </div>;
}
