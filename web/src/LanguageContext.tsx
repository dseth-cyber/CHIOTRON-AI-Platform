import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  detectLanguage,
  formatDate as formatDateIn,
  formatNumber as formatNumberIn,
  LANGUAGES,
  LANGUAGE_NAMES,
  storeLanguage,
  translate,
  type Language,
  type TranslationKey,
} from './i18n';

type Translator = (key: TranslationKey, params?: Record<string, string | number>) => string;

type LanguageValue = {
  language: Language;
  setLanguage: (language: Language) => void;
  t: Translator;
  formatDate: (date: Date | string) => string;
  formatNumber: (value: number) => string;
};

const LanguageContext = createContext<LanguageValue | null>(null);

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>(detectLanguage);

  // Screen readers and the browser's own hyphenation depend on this being right.
  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);

  const setLanguage = useCallback((next: Language) => {
    storeLanguage(next);
    setLanguageState(next);
  }, []);

  const value = useMemo<LanguageValue>(
    () => ({
      language,
      setLanguage,
      t: (key, params) => translate(language, key, params),
      formatDate: (date) => formatDateIn(date, language),
      formatNumber: (amount) => formatNumberIn(amount, language),
    }),
    [language, setLanguage],
  );

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

export function useTranslation(): LanguageValue {
  const value = useContext(LanguageContext);
  if (value === null) {
    throw new Error('useTranslation must be used inside a LanguageProvider');
  }
  return value;
}

export function LanguageSwitcher() {
  const { language, setLanguage, t } = useTranslation();

  return (
    <label className="field inline language">
      <span className="visually-hidden">{t('lang.label')}</span>
      <select
        value={language}
        aria-label={t('lang.label')}
        onChange={(event) => setLanguage(event.target.value as Language)}
      >
        {LANGUAGES.map((option) => (
          <option key={option} value={option}>
            {LANGUAGE_NAMES[option]}
          </option>
        ))}
      </select>
    </label>
  );
}
