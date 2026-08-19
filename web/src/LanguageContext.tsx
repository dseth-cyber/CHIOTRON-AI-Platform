import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
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

const COUNTRY_CODES: Record<Language, string> = {
  th: 'TH',
  en: 'GB',
  zh: 'CN',
  my: 'MM',
  ja: 'JP',
};

export function LanguageSwitcher() {
  const { language, setLanguage, t } = useTranslation();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [open]);

  const currentCode = COUNTRY_CODES[language] ?? 'TH';

  return (
    <div className="lang-switcher-wrap" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-label={t('lang.label')}
        aria-expanded={open}
        className="lang-trigger-btn"
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="m5 8 6 6" />
          <path d="m4 14 6-6 2-3" />
          <path d="M2 5h12" />
          <path d="M7 2h1" />
          <path d="m22 22-5-10-5 10" />
          <path d="M14 18h6" />
        </svg>
        <span>{currentCode}</span>
      </button>

      {open && (
        <div className="lang-dropdown-menu">
          <div className="lang-menu-title">เลือกภาษา</div>
          {LANGUAGES.map((lang) => {
            const isSelected = language === lang;
            const code = COUNTRY_CODES[lang] ?? lang.toUpperCase();
            const name = LANGUAGE_NAMES[lang];
            return (
              <button
                key={lang}
                type="button"
                onClick={() => {
                  setLanguage(lang);
                  setOpen(false);
                }}
                className={isSelected ? 'lang-menu-item active' : 'lang-menu-item'}
              >
                <span className="lang-menu-code">{code}</span>
                <span>{name}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
