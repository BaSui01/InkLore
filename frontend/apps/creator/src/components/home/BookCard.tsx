import { ArrowUpRight, BookOpen, Clock3 } from "lucide-react";
import { type Novel } from "@inklore/contracts";
import { useI18n } from "@inklore/i18n";

export function bookDate(value: string, locale: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : new Intl.DateTimeFormat(locale, {
    timeZone: "Asia/Shanghai", month: "short", day: "numeric", year: "numeric",
  }).format(date);
}

export function BookCard({ novel, onOpen }: { novel: Novel; onOpen: (id: string) => void }) {
  const { t, locale } = useI18n();
  return <article className="home-book-card">
    <div className="home-book-cover" aria-hidden="true"><span className="home-book-spine" /><BookOpen size={24} strokeWidth={1.1} /><strong>{novel.title}</strong><span>{novel.category || t("home.uncategorized")}</span><i /></div>
    <div className="home-book-detail"><div className="home-book-meta"><span>{novel.category || t("home.uncategorized")}</span><span className="home-status">{t(`home.status.${novel.status}`)}</span></div>
      <h3>{novel.title}</h3><p>{novel.summary || t("home.noSummary")}</p>
      <div className="home-book-bottom"><span>{t("home.words", new Intl.NumberFormat(locale).format(novel.word_count))}</span>
        <button onClick={() => onOpen(novel.id)} aria-label={t("home.openBook", novel.title)}>{t("home.continue")}<ArrowUpRight size={16} /></button></div>
      <small className="home-book-date"><Clock3 size={12} aria-hidden="true" /><time dateTime={novel.updated_at}>{bookDate(novel.updated_at, locale)}</time>{t("home.beijingTime")}</small>
    </div>
  </article>;
}
