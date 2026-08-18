import { useTranslation } from '../LanguageContext';
import type { Navigate } from '../navigation';

/**
 * Shared Chats is named in ARCHITECTURE-v1 section 10 and is not built.
 *
 * The page exists to say so. A conversation belongs to an API key, not to a
 * person: today two people using the same key already see the same history, and
 * two keys held by one person see none of each other's. Sharing needs an owner
 * that survives a key rotation, which is the Identity Service contract that
 * section 13 item 2 is still waiting on. Building a share button on top of key
 * ownership would create a feature that looks right and leaks across whoever
 * holds the credential.
 */
export function SharedChats({ onNavigate }: { onNavigate: Navigate }) {
  const { t } = useTranslation();

  return (
    <section className="blocked-page">
      <span className="module-tag">BLOCKED</span>
      <h2>{t('shared.title')}</h2>
      <p>{t('shared.body')}</p>
      <p className="caution">{t('blocker.identity')}</p>
      <div className="modal-actions">
        <button className="secondary" onClick={() => onNavigate('roadmap')}>
          {t('action.viewRoadmap')}
        </button>
        <button className="primary" onClick={() => onNavigate('history')}>
          {t('shared.useHistory')}
        </button>
      </div>
    </section>
  );
}
