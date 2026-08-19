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
  const [isMarked, setIsMarked] = useState(marked);

  useEffect(() => {
    setIsMarked(marked);
  }, [marked]);

  const handleToggle = async (e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    if (busy) return;

    const nextState = !isMarked;
    setIsMarked(nextState);
    setBusy(true);

    try {
      await toggle(kind, targetId, nextState);
    } catch (err) {
      console.error('Failed to toggle favorite:', err);
      setIsMarked(!nextState); // Revert on failure
    } finally {
      setBusy(false);
    }
  };

  return (
    <button
      type="button"
      className={`favorite ${isMarked ? 'marked' : ''}`}
      disabled={busy}
      aria-pressed={isMarked}
      title={isMarked ? t('favorite.remove', { label }) : t('favorite.add', { label })}
      aria-label={isMarked ? t('favorite.remove', { label }) : t('favorite.add', { label })}
      onClick={handleToggle}
    >
      <span className="star-glyph">{isMarked ? '★' : '☆'}</span>
    </button>
  );
}
