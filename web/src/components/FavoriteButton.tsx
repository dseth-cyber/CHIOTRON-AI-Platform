import { useState } from 'react';
import type { FavoriteKind } from '../api';
import { useToggleFavorite } from '../hooks';
import { useTranslation } from '../LanguageContext';

/**
 * Marks one assistant, conversation or document.
 *
 * Failures are swallowed into a disabled state rather than an error banner: a
 * favourite that did not save is worth a quiet retry, not an interruption to
 * whatever the page was actually for.
 */
export function FavoriteButton({
  kind,
  targetId,
  marked,
  label,
}: {
  kind: FavoriteKind;
  targetId: string;
  marked: boolean;
  label: string;
}) {
  const { t } = useTranslation();
  const toggle = useToggleFavorite();
  const [busy, setBusy] = useState(false);

  return (
    <button
      className={marked ? 'favorite marked' : 'favorite'}
      disabled={busy}
      aria-pressed={marked}
      title={marked ? t('favorite.remove', { label }) : t('favorite.add', { label })}
      aria-label={marked ? t('favorite.remove', { label }) : t('favorite.add', { label })}
      onClick={async () => {
        setBusy(true);
        try {
          await toggle(kind, targetId, !marked);
        } finally {
          setBusy(false);
        }
      }}
    >
      {marked ? '★' : '☆'}
    </button>
  );
}
