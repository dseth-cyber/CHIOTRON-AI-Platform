import { useTranslation } from '../LanguageContext';
import { EmptyState, Tag } from '../components/EmptyState';
import {
  useAssistants,
  useComputeHealth,
  useConversations,
  useCredential,
  useDocuments,
  useFavorites,
  useIdentity,
  usePlatform,
  useScopes,
} from '../hooks';
import { statusTone, toneFor } from '../theme';
import { SCOPE_ASSISTANTS_READ, SCOPE_CHAT, SCOPE_KNOWLEDGE_READ } from '../Connection';
import type { Navigate } from '../navigation';

/**
 * The workspace landing page.
 *
 * It answers three questions in order: who am I connected as, what can I do
 * right now, and what was I last doing. Everything it shows is filtered by the
 * connected key's scopes, so a page never advertises a door that is locked.
 */
export function Home({ onNavigate, onConnect }: { onNavigate: Navigate; onConnect: () => void }) {
  const { t, formatNumber, formatDate } = useTranslation();
  const [credential] = useCredential();
  const connected = credential !== '';
  const { has } = useScopes();

  const identity = useIdentity();
  const platform = usePlatform();
  const compute = useComputeHealth(connected);

  const canChat = has(SCOPE_CHAT) && has(SCOPE_ASSISTANTS_READ);
  const canRead = has(SCOPE_KNOWLEDGE_READ);

  const assistants = useAssistants(has(SCOPE_ASSISTANTS_READ));
  const history = useConversations(has(SCOPE_CHAT));
  const documents = useDocuments(canRead);
  const favorites = useFavorites(has(SCOPE_CHAT));

  if (!connected) {
    return (
      <EmptyState
        title={t('home.disconnected.title')}
        body={t('home.disconnected.body')}
        action={
          <button className="primary" onClick={onConnect}>
            {t('conn.addKey')}
          </button>
        }
      />
    );
  }

  const recent = (history.data?.conversations ?? []).slice(0, 5);
  const ready = documents.data?.status?.ready ?? 0;
  const corpus = documents.data?.documents.length ?? 0;

  return (
    <>
      <section className="page-intro">
        <p>{t('home.intro', { name: identity.data?.name ?? t('home.unknownCaller') })}</p>
        <span>
          {platform.isSuccess
            ? `${platform.data.name} · ${platform.data.environment} · ${platform.data.version}`
            : t('portal.discovery.loading')}
        </span>
      </section>

      <section className="quick-grid">
        <button className="quick-card" disabled={!canChat} onClick={() => onNavigate('chat')}>
          <span className="module-tag">NEW</span>
          <h2>{t('nav.newChat')}</h2>
          <p>{canChat ? t('home.quick.chat') : t('nav.chatDisabled')}</p>
        </button>
        <button className="quick-card" disabled={!canRead} onClick={() => onNavigate('search')}>
          <span className="module-tag">SRCH</span>
          <h2>{t('nav.search')}</h2>
          <p>{canRead ? t('home.quick.search') : t('nav.knowledgeDisabled')}</p>
        </button>
        <button className="quick-card" disabled={!canRead} onClick={() => onNavigate('documents')}>
          <span className="module-tag">DOC</span>
          <h2>{t('nav.documents')}</h2>
          <p>{canRead ? t('home.quick.documents') : t('nav.knowledgeDisabled')}</p>
        </button>
        <button
          className="quick-card"
          disabled={!has(SCOPE_ASSISTANTS_READ)}
          onClick={() => onNavigate('assistants')}
        >
          <span className="module-tag">ASST</span>
          <h2>{t('nav.assistants')}</h2>
          <p>{t('home.quick.assistants')}</p>
        </button>
      </section>

      <section className="stat-grid">
        <article className="stat-card">
          <span>{t('home.stat.clearance')}</span>
          <strong>{identity.data?.maxClassification ?? '—'}</strong>
          <small>
            {identity.data?.department
              ? t('home.stat.clearance.hint', { department: identity.data.department })
              : t('home.stat.clearance.hintEmpty')}
          </small>
        </article>
        <article className="stat-card">
          <span>{t('home.stat.assistants')}</span>
          <strong>{assistants.isSuccess ? formatNumber(assistants.data.length) : '—'}</strong>
          <small>{t('home.stat.assistants.hint')}</small>
        </article>
        <article className="stat-card">
          <span>{t('home.stat.corpus')}</span>
          <strong>{documents.isSuccess ? `${formatNumber(ready)}/${formatNumber(corpus)}` : '—'}</strong>
          <small>{canRead ? t('home.stat.corpus.hint') : t('home.stat.needsScope')}</small>
        </article>
        <article className={`stat-card tone-${toneFor(statusTone, compute.data?.status)}`}>
          <span>{t('stat.compute')}</span>
          <strong>{compute.data?.status ?? '—'}</strong>
          <small>
            {platform.data
              ? t('stat.compute.hint', { provider: platform.data.computeProvider })
              : t('stat.needsKey')}
          </small>
        </article>
      </section>

      <section className="home-columns">
        <section className="panel">
          <header className="panel-head">
            <span className="panel-label">{t('home.recent')}</span>
            <button className="text-button" onClick={() => onNavigate('history')}>
              {t('action.viewAll')} <span>→</span>
            </button>
          </header>
          {recent.length === 0 && <p className="history-hint">{t('chat.historyEmpty')}</p>}
          <ul className="plain-list">
            {recent.map((summary) => (
              <li key={summary.id}>
                <button
                  className="list-row"
                  onClick={() => onNavigate('chat', { conversationId: summary.id })}
                >
                  <b>{summary.title || t('chat.untitled')}</b>
                  <small>
                    {summary.assistantName ?? t('chat.unknownAssistant')} ·{' '}
                    {formatDate(summary.updatedAt)}
                  </small>
                </button>
              </li>
            ))}
          </ul>
        </section>

        <section className="panel">
          <header className="panel-head">
            <span className="panel-label">{t('nav.favorites')}</span>
            <button className="text-button" onClick={() => onNavigate('favorites')}>
              {t('action.viewAll')} <span>→</span>
            </button>
          </header>
          {(favorites.data ?? []).length === 0 && (
            <p className="history-hint">{t('favorites.empty.body')}</p>
          )}
          <ul className="plain-list">
            {(favorites.data ?? []).slice(0, 5).map((mark) => (
              <li key={`${mark.kind}:${mark.targetId}`}>
                <span className="list-row static">
                  <b>{mark.label}</b>
                  <small>
                    <Tag>{t(`favorite.kind.${mark.kind}`)}</Tag> {mark.detail ?? ''}
                  </small>
                </span>
              </li>
            ))}
          </ul>
        </section>
      </section>
    </>
  );
}
