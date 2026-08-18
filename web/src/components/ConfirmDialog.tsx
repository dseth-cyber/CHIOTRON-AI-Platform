import { useState } from 'react';
import { Modal } from '../Modal';
import { useTranslation } from '../LanguageContext';

/**
 * The one way to confirm a destructive action.
 *
 * ARCHITECTURE-v1 section 36 forbids `confirm()`, and section 43 forbids
 * destroying anything from a single click. Two levels are offered:
 *
 *   - a plain confirm, for a reversible action such as moving to the trash
 *   - `confirmationCode`, for permanent destruction: the exact text has to be
 *     typed back, so a reflex click on the wrong row cannot destroy anything
 *
 * The busy state lives here rather than at the call site because a dialog that
 * stays clickable while its action runs is how something gets deleted twice.
 */
export function ConfirmDialog({
  title,
  body,
  caution,
  confirmLabel,
  confirmationCode,
  destructive = false,
  onCancel,
  onConfirm,
}: {
  title: string;
  body: string;
  caution?: string;
  confirmLabel: string;
  confirmationCode?: string;
  destructive?: boolean;
  onCancel: () => void;
  onConfirm: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [typed, setTyped] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const unlocked = confirmationCode === undefined || typed.trim() === confirmationCode;

  const run = async () => {
    setBusy(true);
    setError('');
    try {
      await onConfirm();
    } catch (failed) {
      // The dialog stays open on failure: closing it would leave the user
      // believing something happened that did not.
      setError(failed instanceof Error ? failed.message : t('confirm.failed'));
      setBusy(false);
    }
  };

  return (
    <Modal title={title} onClose={busy ? () => undefined : onCancel}>
      <p>{body}</p>
      {caution && <p className="caution">{caution}</p>}

      {confirmationCode !== undefined && (
        <label className="field">
          <span>{t('confirm.typeToConfirm')}</span>
          <code className="confirm-code">{confirmationCode}</code>
          <input
            value={typed}
            spellCheck={false}
            autoComplete="off"
            autoFocus
            onChange={(event) => setTyped(event.target.value)}
          />
        </label>
      )}

      {error !== '' && <p className="error-note">{error}</p>}

      <div className="modal-actions">
        <button className="secondary" disabled={busy} onClick={onCancel}>
          {t('action.cancel')}
        </button>
        <button
          className={destructive ? 'danger-primary' : 'primary'}
          disabled={busy || !unlocked}
          onClick={() => void run()}
        >
          {busy ? t('confirm.working') : confirmLabel}
        </button>
      </div>
    </Modal>
  );
}
