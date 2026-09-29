import React from "react";
import { useTranslation } from "@inklore/i18n";
import { useTheme } from "../theme/ThemeProvider";
import {
  Sun,
  Moon,
  Globe,
  Feather,
  Terminal,
} from "lucide-react";
import "./AuthPageLayout.css";

export interface AuthPageLayoutProps {
  audience?: "creator" | "admin";
  className?: string;
  children: React.ReactNode;
}

export function AuthPageLayout({
  audience = "creator",
  className,
  children,
}: AuthPageLayoutProps) {
  const { t, locale, setLocale } = useTranslation();
  const { mode: themeMode, setMode: setThemeMode } = useTheme();

  const mark = audience === "admin" ? <Terminal /> : <Feather />;
  const productArea = audience === "admin"
    ? t("auth.admin_brand_label")
    : t("auth.creator_brand_label");

  return (
    <div className={`auth-shell min-h-screen w-screen flex flex-col lg:flex-row text-[var(--text-primary)] overflow-x-hidden ${className || ""}`}>
      <aside className="auth-brand" aria-label={productArea}>
        <div className="auth-brand-top">
          <div className="auth-brand-mark" aria-hidden="true">{mark}</div>
          <span className="auth-brand-name">InkLore</span>
        </div>
        <div className="auth-brand-bottom">
          <span className="auth-brand-rule" aria-hidden="true" />
          <span>{productArea}</span>
        </div>
      </aside>

      <section className="auth-main">
        <main className="auth-main-content">
          <div className="auth-form-column">{children}</div>
        </main>
        <footer className="auth-footer">
          <div className="auth-copyright">{t("auth.footer_copyright")}</div>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setThemeMode(themeMode === "dark" ? "light" : "dark")}
              className="auth-utility-button"
              title={themeMode === "dark" ? t("auth.theme_light") : t("auth.theme_dark")}
            >
              {themeMode === "dark" ? <Moon /> : <Sun />}
            </button>
            <button
              type="button"
              onClick={() => setLocale(locale.startsWith("zh") ? "en-US" : "zh-CN")}
              className="auth-utility-button auth-language-button"
              title={t("admin.sidebar.lang_switch")}
            >
              <Globe />
              <span>{locale.startsWith("zh") ? "EN" : "中"}</span>
            </button>
          </div>
        </footer>
      </section>
    </div>
  );
}

