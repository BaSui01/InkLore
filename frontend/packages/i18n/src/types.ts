/**
 * @inklore/i18n - 国际化核心类型定义
 * 纯净排版与零 Emoji 铁律
 */

export type Locale = "zh-CN" | "en-US";

export type TranslationParamValue = string | number;

export type TranslationParamMap = Record<string, TranslationParamValue>;

export type TranslationParams = TranslationParamMap | TranslationParamValue[];

export interface TranslationFunction {
  (key: string, ...args: (TranslationParamValue | TranslationParamMap)[]): string;
}

export interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: TranslationFunction;
}
