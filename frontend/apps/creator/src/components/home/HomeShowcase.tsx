import React from "react";
import {
  ArrowRight,
  BookMarked,
  Feather,
  Lightbulb,
  Orbit,
  PenLine,
  Route,
  ScrollText,
  Sparkles,
  Waypoints,
} from "lucide-react";
import { useI18n } from "@inklore/i18n";
import { Button } from "@inklore/ui";
import "./home-showcase.css";

export function HomeIllustration() {
  const { t } = useI18n();

  return (
    <aside className="ih-art" aria-label={t("home.illustration.label")}>
      <div className="ih-art__scene" aria-hidden="true">
        <div className="ih-art__orbit ih-art__orbit--outer" />
        <div className="ih-art__orbit ih-art__orbit--inner" />
        <span className="ih-art__star ih-art__star--one" />
        <span className="ih-art__star ih-art__star--two" />
        <span className="ih-art__star ih-art__star--three" />

        <div className="ih-art__paper ih-art__paper--back">
          <span className="ih-art__paper-line" />
          <span className="ih-art__paper-line" />
          <span className="ih-art__paper-line" />
          <span className="ih-art__paper-line ih-art__paper-line--short" />
        </div>
        <div className="ih-art__paper ih-art__paper--front">
          <span className="ih-art__paper-rule" />
          <span className="ih-art__paper-line" />
          <span className="ih-art__paper-line" />
          <span className="ih-art__paper-line ih-art__paper-line--wide" />
          <span className="ih-art__paper-line ih-art__paper-line--short" />
          <span className="ih-art__paper-seal" />
        </div>

        <svg className="ih-art__pen" viewBox="0 0 176 92" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M8 78L28 58L138 11C146 7 155 10 160 18L166 27C170 34 167 42 160 47L108 83L8 78Z" fill="currentColor" />
          <path d="M8 78L31 57L45 73L8 78Z" fill="var(--bg-paper)" />
          <path d="M31 57L38 50L53 67L45 73L31 57Z" fill="var(--home-accent)" />
          <path d="M112 25L147 57" stroke="var(--bg-paper)" strokeWidth="3" strokeLinecap="round" opacity=".62" />
          <path d="M121 19L155 51" stroke="var(--bg-paper)" strokeWidth="2" strokeLinecap="round" opacity=".38" />
        </svg>
        <span className="ih-art__ink-dot" />
      </div>

      <div className="ih-art__caption">
        <span className="ih-art__caption-mark" aria-hidden="true">
          <PenLine size={15} strokeWidth={1.7} />
        </span>
        <div>
          <p>{t("home.illustration.tag1")}</p>
          <p>{t("home.illustration.tag2")}</p>
        </div>
      </div>

      <div className="ih-art__copy">
        <span className="ih-art__copy-kicker">{t("home.illustration.label")}</span>
        <h2>{t("home.illustration.title")}</h2>
        <p>{t("home.illustration.detail")}</p>
      </div>
    </aside>
  );
}

export const HeroArtwork = HomeIllustration;

export interface HomeFeaturesProps {
  onCreate: () => void;
}

export function HomeFeatures({ onCreate }: HomeFeaturesProps) {
  const { t } = useI18n();

  return (
    <section className="ih-showcase" id="home-features" aria-labelledby="ih-features-title">
      <div className="ih-showcase__intro">
        <span className="ih-showcase__eyebrow">{t("home.features.eyebrow")}</span>
        <h2 id="ih-features-title">{t("home.features.title")}</h2>
        <p>{t("home.features.description")}</p>
      </div>

      <div className="ih-showcase__feature-grid">
        <article className="ih-feature-card">
          <span className="ih-feature-card__icon ih-feature-card__icon--gold" aria-hidden="true">
            <Route size={25} strokeWidth={1.45} />
          </span>
          <span className="ih-feature-card__line" aria-hidden="true" />
          <h3>{t("home.features.outline.title")}</h3>
          <p>{t("home.features.outline.description")}</p>
        </article>

        <article className="ih-feature-card">
          <span className="ih-feature-card__icon ih-feature-card__icon--sage" aria-hidden="true">
            <Orbit size={25} strokeWidth={1.45} />
          </span>
          <span className="ih-feature-card__line" aria-hidden="true" />
          <h3>{t("home.features.world.title")}</h3>
          <p>{t("home.features.world.description")}</p>
        </article>

        <article className="ih-feature-card">
          <span className="ih-feature-card__icon ih-feature-card__icon--ink" aria-hidden="true">
            <Lightbulb size={24} strokeWidth={1.45} />
          </span>
          <span className="ih-feature-card__line" aria-hidden="true" />
          <h3>{t("home.features.inspiration.title")}</h3>
          <p>{t("home.features.inspiration.description")}</p>
        </article>

        <article className="ih-feature-card">
          <span className="ih-feature-card__icon ih-feature-card__icon--clay" aria-hidden="true">
            <Feather size={25} strokeWidth={1.45} />
          </span>
          <span className="ih-feature-card__line" aria-hidden="true" />
          <h3>{t("home.features.writing.title")}</h3>
          <p>{t("home.features.writing.description")}</p>
        </article>
      </div>

      <section className="ih-journey" id="home-guide" tabIndex={-1} aria-labelledby="ih-journey-title">
        <div className="ih-journey__heading">
          <span className="ih-showcase__eyebrow">{t("home.journey.eyebrow")}</span>
          <h2 id="ih-journey-title">{t("home.journey.title")}</h2>
        </div>
        <div className="ih-journey__steps">
          <article className="ih-journey__step">
            <span className="ih-journey__number" aria-hidden="true">01</span>
            <Lightbulb className="ih-journey__icon" size={22} strokeWidth={1.4} aria-hidden="true" />
            <div>
              <h3>{t("home.journey.idea.title")}</h3>
              <p>{t("home.journey.idea.description")}</p>
            </div>
          </article>
          <span className="ih-journey__connector" aria-hidden="true"><Waypoints size={21} strokeWidth={1.2} /></span>
          <article className="ih-journey__step">
            <span className="ih-journey__number" aria-hidden="true">02</span>
            <BookMarked className="ih-journey__icon" size={22} strokeWidth={1.4} aria-hidden="true" />
            <div>
              <h3>{t("home.journey.plan.title")}</h3>
              <p>{t("home.journey.plan.description")}</p>
            </div>
          </article>
          <span className="ih-journey__connector" aria-hidden="true"><Waypoints size={21} strokeWidth={1.2} /></span>
          <article className="ih-journey__step">
            <span className="ih-journey__number" aria-hidden="true">03</span>
            <ScrollText className="ih-journey__icon" size={22} strokeWidth={1.4} aria-hidden="true" />
            <div>
              <h3>{t("home.journey.write.title")}</h3>
              <p>{t("home.journey.write.description")}</p>
            </div>
          </article>
        </div>
      </section>

      <section className="ih-closing" aria-labelledby="ih-closing-title">
        <div className="ih-closing__symbol" aria-hidden="true">
          <Sparkles size={25} strokeWidth={1.35} />
        </div>
        <div className="ih-closing__copy">
          <h2 id="ih-closing-title">{t("home.closing.title")}</h2>
          <p>{t("home.closing.description")}</p>
        </div>
        <Button className="ih-closing__action" variant="primary" size="lg" icon={<Feather size={17} strokeWidth={1.8} />} onClick={onCreate}>
          {t("home.closing.action")}
          <ArrowRight size={16} strokeWidth={1.8} aria-hidden="true" />
        </Button>
      </section>
    </section>
  );
}
