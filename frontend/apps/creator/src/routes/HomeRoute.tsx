import { useEffect, useMemo, useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ArrowDown, ArrowRight, ArrowUpRight, BookOpen, Feather, Library, Plus, RefreshCw, Search, Sparkles, WifiOff } from "lucide-react";
import { Button, isImeComposing } from "@inklore/ui";
import { useI18n } from "@inklore/i18n";
import { useHomeBookshelf } from "../domains/content/useHomeBookshelf";
import { HomeHeader, scrollHomeSection } from "../components/home/HomeHeader";
import { HomeNotice } from "../components/home/HomeNotice";
import { BookCard, bookDate } from "../components/home/BookCard";
import { HomeIllustration, HomeFeatures } from "../components/home/HomeShowcase";
import { useCreatorSession } from "../context/CreatorSessionContext";
import "../components/home/home.css";

export interface HomeRouteProps {
  idea: string;
  onIdeaChange: (idea: string) => void;
  onOpenNovel: (novelId: string) => void;
  onNavigate: (view: "outline" | "worldview" | "analysis" | "agent-guide") => void;
}

function HomePage({ onOpenNovel, onNavigate, idea, onIdeaChange }: HomeRouteProps) {
  const { t, locale } = useI18n();
  const { isAuthenticated, openAuthPage } = useCreatorSession();
  const shelf = useHomeBookshelf(isAuthenticated);
  const { query, novels, filtered } = shelf;
  const [offline, setOffline] = useState(!navigator.onLine);
  useEffect(() => {
    const update = () => setOffline(!navigator.onLine);
    window.addEventListener("online", update); window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);
  const number = new Intl.NumberFormat(locale);
  const latest = isAuthenticated ? novels[0] : undefined;
  const unavailable = shelf.forbidden || (!query.data && query.isError);
  const hasSnapshot = !!query.data && !shelf.forbidden;
  const create = () => { if (isAuthenticated) onNavigate("agent-guide"); else openAuthPage("login"); };

  return <div className="creator-home" lang={locale}>
    <a className="home-skip" href="#home-main" onClick={event => { event.preventDefault(); scrollHomeSection("home-main"); }}>{t("home.skip")}</a>
    <HomeHeader />
    <main id="home-main" tabIndex={-1}>
      {offline && <div className="home-offline" role="status"><WifiOff size={18} />{t("home.offline")}</div>}
      <section className="home-hero home-container" aria-labelledby="home-title">
        <div className="home-hero-copy">
          <div className="home-eyebrow"><span /><span>{t("home.eyebrow")}</span><span className="home-eyebrow-divider" /><span>{t("home.eyebrowTail")}</span></div>
          <h1 id="home-title">{t("home.titleFirst")}<br /><em>{t("home.titleSecond")}</em><span className="home-title-period">。</span></h1>
          <p className="home-hero-description">{t("home.description")}</p>
          <div className="home-composer">
            <div className="home-composer-heading"><Feather size={17} /><label htmlFor="home-idea">{t("home.ideaLabel")}</label><span>{t("home.ideaBadge")}</span></div>
            <textarea id="home-idea" value={idea} onChange={event => onIdeaChange(event.target.value)} onKeyDown={event => {
              if (isImeComposing(event)) return;
              if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
                event.preventDefault();
                if (!offline) create();
              }
            }} rows={3} placeholder={t("home.ideaPlaceholder")} aria-describedby="home-idea-help" />
            <div className="home-composer-footer"><small id="home-idea-help"><Sparkles size={14} />{t("home.ideaHelp")}</small>
              <Button className="home-composer-submit" variant="ghost" onClick={create} disabled={offline} aria-label={t("home.startWithIdea")} title={t("home.startWithIdea")}><ArrowRight size={20} /></Button></div>
          </div>
          <div className="home-hero-actions"><Button className="home-button home-button-primary" onClick={create} disabled={offline} icon={<Plus size={18} />}>{t("home.newBook")}</Button>
            <Button className="home-button home-button-secondary" onClick={() => scrollHomeSection("home-bookshelf")} icon={<Library size={18} />}>{t("home.myBooks")}</Button>
            <button className="home-text-button" onClick={() => scrollHomeSection("home-guide")}>{t("home.startGuide")}<ArrowUpRight size={16} /></button></div>
          <p className="home-hero-footnote"><span />{t("home.authorControl")}</p>
        </div>
        <div className="home-hero-visual">
          <HomeIllustration />
          <div className="home-latest-card" aria-busy={isAuthenticated && query.isPending}>
            <div className="home-latest-top"><span className="home-latest-icon"><BookOpen size={18} /></span><span>{t("home.latest")}</span><span className="home-latest-mark" /></div>
            {!isAuthenticated ? <><h2>{t("home.guestTitle")}</h2><p>{t("home.guestHint")}</p><button className="home-text-button" onClick={() => openAuthPage("login")}>{t("home.login")}<ArrowRight size={15} /></button></> : query.isPending ? <p className="home-loading-text" role="status">{t("home.loading")}</p> : unavailable ? <p>{t("home.previewError")}</p> : latest ? <>
              <h2>{latest.title}</h2><p>{latest.summary || t("home.noSummary")}</p>
              <div className="home-latest-bottom"><span>{t("home.words", number.format(latest.word_count))}</span><button onClick={() => onOpenNovel(latest.id)} aria-label={t("home.openBook", latest.title)}>{t("home.continue")}<ArrowRight size={15} /></button></div>
            </> : <><h2>{t("home.firstBook")}</h2><p>{t("home.firstBookHint")}</p><button className="home-text-button" onClick={create} disabled={offline}>{t("home.writeFirst")}<ArrowRight size={15} /></button></>}
          </div>
        </div>
      </section>
      <div className="home-section-divider home-container"><span>{t("home.divider")}</span><ArrowDown size={16} /></div>
      <section className="home-bookshelf home-container" aria-labelledby="home-bookshelf" aria-busy={query.isFetching}>
        <div className="home-section-heading"><div><span className="home-kicker">{t("home.booksKicker")}</span><h2 id="home-bookshelf" tabIndex={-1}>{t("home.myBooks")}</h2><p>{t("home.booksDescription")}</p></div>
          <Button className="home-button home-button-secondary" onClick={create} disabled={offline} icon={<Plus size={17} />}>{t("home.newBook")}</Button></div>
        {!isAuthenticated ? <div className="home-empty"><span className="home-empty-icon"><Library size={29} /></span><h3>{t("home.guestTitle")}</h3><p>{t("home.guestHint")}</p><Button className="home-button home-button-primary" onClick={() => openAuthPage("login")}>{t("home.login")}</Button></div> : <>
        <div className="home-shelf-toolbar"><div className="home-filter-group" role="group" aria-label={t("home.filter")}>
          <button className={shelf.status === "active" ? "is-active" : ""} aria-pressed={shelf.status === "active"} onClick={() => shelf.setStatus("active")}>{t("home.status.active")}</button>
          <button className={shelf.status === "archived" ? "is-active" : ""} aria-pressed={shelf.status === "archived"} onClick={() => shelf.setStatus("archived")}>{t("home.status.archived")}</button>
        </div><div className="home-search-actions"><label className="home-search"><Search size={17} aria-hidden="true" /><input type="search" value={shelf.search} onChange={event => shelf.setSearch(event.target.value)} placeholder={t("home.search")} aria-label={t("home.search")} /></label>
          <Button className="home-icon-button" variant="ghost" onClick={() => void query.refetch()} disabled={query.isFetching || offline} title={t("home.refresh")} aria-label={t("home.refresh")}><RefreshCw size={17} className={query.isFetching ? "home-spinning" : ""} /></Button></div></div>
        {query.isError && <HomeNotice error={query.error} stale={hasSnapshot} busy={query.isFetching} retry={() => void query.refetch()} />}
        {hasSnapshot && <div className="home-shelf-summary"><span>{t("home.loaded", number.format(novels.length))}{query.hasNextPage ? t("home.moreAvailable") : ""}</span><span>{t("home.loadedWords", number.format(novels.reduce((count, novel) => count + novel.word_count, 0)))}</span>{latest && <span>{t("home.lastUpdated", bookDate(latest.updated_at, locale))}</span>}</div>}
        {query.isPending && <div className="home-shelf-loading" role="status"><span className="home-loading-orbit" /><span>{t("home.loading")}</span></div>}
        {hasSnapshot && filtered.length > 0 && <div className="home-book-grid">{filtered.map(novel => <BookCard key={novel.id} novel={novel} onOpen={onOpenNovel} />)}</div>}
        {hasSnapshot && filtered.length === 0 && <div className="home-empty"><span className="home-empty-icon">{shelf.search ? <Search size={29} /> : <BookOpen size={29} />}</span>
          <h3>{t(shelf.search ? "home.searchEmpty" : shelf.status === "archived" ? "home.archiveEmpty" : "home.empty")}</h3><p>{t(shelf.search ? "home.searchEmptyHint" : "home.emptyHint")}</p>
          {shelf.search ? <Button className="home-button home-button-secondary" onClick={() => shelf.setSearch("")}>{t("home.clearSearch")}</Button> : shelf.status === "active" && <Button className="home-button home-button-primary" onClick={create} disabled={offline} icon={<Plus size={17} />}>{t("home.writeFirst")}</Button>}</div>}
        {query.hasNextPage && !shelf.forbidden && <div className="home-load-more"><Button className="home-button home-button-secondary" onClick={() => void query.fetchNextPage()} loading={query.isFetchingNextPage} disabled={query.isFetching || offline}>{t("home.loadMore")}</Button><small>{t("home.searchScope")}</small></div>}
        </>}
      </section>
      <HomeFeatures onCreate={create} />
      <nav className="home-tool-links home-container" aria-label={t("home.toolsNavigation")}><button onClick={() => onNavigate("outline")}>{t("home.tool.outline")}<ArrowUpRight size={16} /></button><button onClick={() => onNavigate("worldview")}>{t("home.tool.worldview")}<ArrowUpRight size={16} /></button><button onClick={() => onNavigate("analysis")}>{t("home.tool.analysis")}<ArrowUpRight size={16} /></button></nav>
    </main>
    <footer className="home-footer home-container"><span className="home-footer-brand"><Feather size={17} />{t("home.brand")}</span><p>{t("home.footer")}</p><button className="home-text-button" onClick={() => scrollHomeSection("home-main")}>{t("home.backTop")}<ArrowUpRight size={15} /></button></footer>
  </div>;
}

export function HomeRoute(props: HomeRouteProps) {
  const { snapshot } = useCreatorSession();
  const client = useMemo(() => new QueryClient({ defaultOptions: { queries: { retry: false } } }), [snapshot.sessionEpoch]);
  useEffect(() => () => client.clear(), [client]);
  return <QueryClientProvider client={client}><HomePage {...props} /></QueryClientProvider>;
}
