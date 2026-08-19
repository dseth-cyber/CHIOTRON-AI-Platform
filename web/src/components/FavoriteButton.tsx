import { useEffect, useState } from 'react';
import type { FavoriteKind } from '../api';
import { useToggleFavorite } from '../hooks';
import { useTranslation } from '../LanguageContext';

/**
 * Marks one assistant, conversation or document.
 *
 * Provides immediate optimistic visual feedback and error recovery.
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
  const [optimisticMarked, setOptimisticMarked] = useState<boolean | null>(null);

  const isMarked = optimisticMarked !== null ? optimisticMarked : marked;

  useEffect(() => {
    setOptimisticMarked(null);
  }, [marked]);

  return (
    <button
      type="button"
      className={isMarked ? 'favorite marked' : 'favorite'}
      disabled={busy}
      aria-pressed={isMarked}
      title={isMarked ? t('favorite.remove', { label }) : t('favorite.add', { label })}
      aria-label={isMarked ? t('favorite.remove', { label }) : t('favorite.add', { label })}
      onClick={async (e) => {
        e.stopPropagation();
        const next = !isMarked;
        setOptimisticMarked(next);
        setBusy(true);
        try {
          await toggle(kind, targetId, next);
        } catch {
          setOptimisticMarked(!next); // revert on error
        } finally {
          setBusy(false);
        }
      }}
    >
      {isMarked ? '★' : '☆'}
    </button>
  );
}
