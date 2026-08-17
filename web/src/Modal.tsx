import type { ReactNode } from 'react';
import { useTranslation } from './LanguageContext';

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

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal" role="dialog" aria-modal="true" aria-label={title}>
        <button className="close" aria-label={t('action.close')} onClick={onClose}>
          ×
        </button>
        <h2>{title}</h2>
        {children}
      </section>
    </div>
  );
}
