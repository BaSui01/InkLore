/**
 * @inklore/i18n - 国际化上下文与 Hook
 * 支持 React19，具备参数插值与安全降级能力
 */

import React, { createContext, useContext, useState, useCallback, useMemo } from "react";
import { Locale, TranslationFunction, TranslationParamValue, TranslationParamMap, I18nContextValue } from "./types";
import { zhCN } from "./locales/zh-CN";
import { enUS } from "./locales/en-US";

const dictionaries: Record<Locale, Record<string, string>> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

const I18nContext = createContext<I18nContextValue | null>(null);

export interface I18nProviderProps {
  children: React.ReactNode;
  defaultLocale?: Locale;
}

export function I18nProvider({ children, defaultLocale }: I18nProviderProps) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    if (defaultLocale) return defaultLocale;
    if (typeof localStorage !== "undefined") {
      const saved = localStorage.getItem("inklore_locale");
      if (saved === "zh-CN" || saved === "en-US") {
        return saved;
      }
    }
    if (typeof navigator !== "undefined" && navigator.language) {
      return navigator.language.startsWith("zh") ? "zh-CN" : "en-US";
    }
    return "zh-CN";
  });

  const setLocale = useCallback((newLocale: Locale) => {
    setLocaleState(newLocale);
    if (typeof localStorage !== "undefined") {
      try {
        localStorage.setItem("inklore_locale", newLocale);
      } catch {}
    }
  }, []);

  const t: TranslationFunction = useCallback(
    (key: string, ...args: (TranslationParamValue | TranslationParamMap)[]): string => {
      const dict = dictionaries[locale] || dictionaries["zh-CN"];
      let text = dict[key] || dictionaries["zh-CN"][key] || key;

      if (args.length === 0) return text;

      // 检查第一个参数是否为对象 map
      if (args.length === 1 && typeof args[0] === "object" && args[0] !== null && !Array.isArray(args[0])) {
        const params = args[0] as TranslationParamMap;
        Object.entries(params).forEach(([k, v]) => {
          text = text.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
        });
        return text;
      }

      // 如果第一个参数是数组
      if (args.length === 1 && Array.isArray(args[0])) {
        const arr = args[0] as TranslationParamValue[];
        arr.forEach((val, idx) => {
          text = text.replace(new RegExp(`\\{${idx}\\}`, "g"), String(val));
        });
        return text;
      }

      // 展开参数列表 t("key", arg0, arg1, ...)
      args.forEach((val, idx) => {
        text = text.replace(new RegExp(`\\{${idx}\\}`, "g"), String(val));
      });

      return text;
    },
    [locale]
  );

  const value = useMemo<I18nContextValue>(
    () => ({ locale, setLocale, t }),
    [locale, setLocale, t]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    const tFallback: TranslationFunction = (k, ...args) => {
      let text = zhCN[k] || k;
      args.forEach((val, idx) => {
        text = text.replace(new RegExp(`\\{${idx}\\}`, "g"), String(val));
      });
      return text;
    };
    return {
      locale: "zh-CN",
      setLocale: () => {},
      t: tFallback,
    };
  }
  return ctx;
}

export function useTranslation() {
  const { t, locale, setLocale } = useI18n();
  return { t, locale, setLocale };
}
