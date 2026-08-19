import type { ReactNode } from 'react';
import { useTranslation } from './LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';

export function Modal({
  title,
  children,
  onClose,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { themeConfig } = useTheme();

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm" role="presentation">
      <section
        className={`relative w-full max-w-lg rounded-2xl border p-6 shadow-2xl transition-all ${themeConfig.cardBorder} ${themeConfig.card}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <button
          className={`absolute top-4 right-4 text-xl font-bold w-8 h-8 rounded-lg flex items-center justify-center transition-colors ${themeConfig.text.secondary} hover:${themeConfig.text.primary} hover:bg-white/10`}
          aria-label={t('action.close')}
          onClick={onClose}
        >
          ×
        </button>
        <h2 className={`text-xl font-bold mb-4 ${themeConfig.text.primary}`}>{title}</h2>
        <div className={`space-y-4 text-sm ${themeConfig.text.secondary}`}>{children}</div>
      </section>
    </div>
  );
}
