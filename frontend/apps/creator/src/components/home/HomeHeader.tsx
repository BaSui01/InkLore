import { useState } from "react";
import { Feather, Languages, LogOut, Menu, Moon, Sun, X } from "lucide-react";
import { Button, useTheme } from "@inklore/ui";
import { useI18n } from "@inklore/i18n";
import { useCreatorSession } from "../../context/CreatorSessionContext";
import { HomeNotice } from "./HomeNotice";

export function scrollHomeSection(id: string) {
  const section = document.getElementById(id);
  section?.scrollIntoView({ behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" });
  section?.focus({ preventScroll: true });
}

export function HomeHeader() {
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode } = useTheme();
  const { principal, logout, isAuthenticated, openAuthPage } = useCreatorSession();
  const [menu, setMenu] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const signOut = async () => { setLeaving(true); try { await logout(); } catch (cause) { setError(cause); } finally { setLeaving(false); } };
  const go = (id: string) => { scrollHomeSection(id); setMenu(false); };
  return <>
    <header className="home-header">
      <div className="home-header-inner">
        <button className="home-brand" onClick={() => go("home-main")} aria-label={t("home.brandHome")}>
          <span className="home-brand-mark"><Feather size={23} strokeWidth={1.4} /></span>
          <span>{t("home.brand")}<small>{t("home.brandCaption")}</small></span>
        </button>
        <nav className={`home-navigation${menu ? " is-open" : ""}`} id="home-navigation" aria-label={t("home.navigation")}>
          <button className="is-current" onClick={() => go("home-main")} aria-current="page">{t("home.nav.home")}</button>
          <button onClick={() => go("home-bookshelf")}>{t("home.nav.books")}</button>
          <button onClick={() => go("home-features")}>{t("home.nav.features")}</button>
          <button onClick={() => go("home-guide")}>{t("home.nav.guide")}</button>
          {isAuthenticated && <button className="home-mobile-logout" onClick={signOut} disabled={leaving}>{t("home.logout")}</button>}
        </nav>
        <div className="home-header-actions">
          <Button variant="ghost" className="home-icon-button" title={t("home.language")} aria-label={t("home.language")}
            onClick={() => setLocale(locale === "zh-CN" ? "en-US" : "zh-CN")}><Languages size={19} /></Button>
          <Button variant="ghost" className="home-icon-button" title={t(mode === "light" ? "home.dark" : "home.light")} aria-label={t(mode === "light" ? "home.dark" : "home.light")}
            onClick={() => setMode(mode === "light" ? "dark" : "light")}>{mode === "light" ? <Moon size={18} /> : <Sun size={18} />}</Button>
          {isAuthenticated ? <><span className="home-account" title={principal?.display_name || t("home.author")}><span className="home-avatar">{Array.from(principal?.display_name || t("home.author"))[0]}</span><span>{principal?.display_name || t("home.author")}</span></span>
          <Button variant="ghost" className="home-icon-button home-logout" aria-label={t("home.logout")} title={t("home.logout")} disabled={leaving}
            onClick={signOut}><LogOut size={17} /></Button></> : <Button className="home-button home-button-secondary" onClick={() => openAuthPage("login")}>{t("home.login")}</Button>}
          <Button variant="ghost" className="home-icon-button home-menu-toggle" aria-controls="home-navigation" aria-expanded={menu} aria-label={t(menu ? "home.closeMenu" : "home.openMenu")}
            onClick={() => setMenu(!menu)}>{menu ? <X size={21} /> : <Menu size={21} />}</Button>
        </div>
      </div>
    </header>
    {error != null && <HomeNotice error={error} />}
  </>;
}
