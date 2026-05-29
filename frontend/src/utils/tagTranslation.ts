import i18n from "../i18n";
import vndbTagTranslationsJaJPRaw from "./vndbTagTranslations.ja-JP.json";
import vndbTagTranslationsZhCNRaw from "./vndbTagTranslations.zh-CN.json";
import vndbTagTranslationsZhTWRaw from "./vndbTagTranslations.zh-TW.json";

type TranslationMap = Record<string, string>;

type SupportedTagTranslationLanguage = "ja-JP" | "zh-CN" | "zh-TW";

function createTranslationMap(rawTranslations: Record<string, unknown>) {
  return Object.fromEntries(
    Object.entries(rawTranslations).filter(
      ([key, value]) => !key.startsWith("_") && typeof value === "string",
    ),
  ) as TranslationMap;
}

const vndbTagTranslationsByLanguage: Record<
  SupportedTagTranslationLanguage,
  TranslationMap
> = {
  "ja-JP": createTranslationMap(
    vndbTagTranslationsJaJPRaw as Record<string, unknown>,
  ),
  "zh-CN": createTranslationMap(
    vndbTagTranslationsZhCNRaw as Record<string, unknown>,
  ),
  "zh-TW": createTranslationMap(
    vndbTagTranslationsZhTWRaw as Record<string, unknown>,
  ),
};

const normalizedTranslatedIndexesByLanguage = Object.fromEntries(
  Object.entries(vndbTagTranslationsByLanguage).map(
    ([language, translations]) => {
      const index = new Map<string, string[]>();

      for (const [rawName, translatedName] of Object.entries(translations)) {
        const normalized = normalizeTagSearchText(translatedName);
        if (!normalized || normalized === normalizeTagSearchText(rawName)) {
          continue;
        }
        const existing = index.get(normalized) ?? [];
        existing.push(rawName);
        index.set(normalized, existing);
      }

      return [language, index];
    },
  ),
) as Record<SupportedTagTranslationLanguage, Map<string, string[]>>;

function getCurrentTagTranslationLanguage() {
  const language = i18n.resolvedLanguage || i18n.language;

  if (language.startsWith("en")) {
    return undefined;
  }

  if (language.startsWith("ja")) {
    return "ja-JP";
  }

  if (
    language === "zh-TW"
    || language === "zh-HK"
    || language === "zh-MO"
    || language.toLowerCase().startsWith("zh-hant")
  ) {
    return "zh-TW";
  }

  if (language.startsWith("zh")) {
    return "zh-CN";
  }

  return undefined;
}

function getCurrentTagTranslationMap() {
  const language = getCurrentTagTranslationLanguage();
  return language ? vndbTagTranslationsByLanguage[language] : undefined;
}

function getCurrentTranslatedIndex() {
  const language = getCurrentTagTranslationLanguage();
  return language ? normalizedTranslatedIndexesByLanguage[language] : undefined;
}

export function getTagDisplayName(
  tagName: string,
  enableTranslation = true,
): string {
  if (!enableTranslation) {
    return tagName;
  }

  return getCurrentTagTranslationMap()?.[tagName] ?? tagName;
}

export function getTagTitle(
  tagName: string,
  enableTranslation = true,
): string | undefined {
  const displayName = getTagDisplayName(tagName, enableTranslation);
  return displayName === tagName ? undefined : tagName;
}

export function findRawTagNamesByTranslatedQuery(query: string): string[] {
  const normalizedQuery = normalizeTagSearchText(query);
  if (!normalizedQuery) {
    return [];
  }

  const translatedIndex = getCurrentTranslatedIndex();
  if (!translatedIndex) {
    return [];
  }

  const matches: string[] = [];
  for (const [translatedName, rawNames] of translatedIndex) {
    if (translatedName.includes(normalizedQuery)) {
      matches.push(...rawNames);
    }
  }

  return [...new Set(matches)];
}

export function filterTagNamesByDisplayQuery(
  tagNames: string[],
  query: string,
  enableTranslation = true,
): string[] {
  const normalizedQuery = normalizeTagSearchText(query);
  if (!normalizedQuery) {
    return tagNames;
  }

  return tagNames.filter((tagName) => {
    const rawName = normalizeTagSearchText(tagName);
    const displayName = normalizeTagSearchText(
      getTagDisplayName(tagName, enableTranslation),
    );
    return (
      rawName.includes(normalizedQuery) || displayName.includes(normalizedQuery)
    );
  });
}

export function normalizeTagSearchText(value: string): string {
  return value.trim().toLocaleLowerCase();
}
