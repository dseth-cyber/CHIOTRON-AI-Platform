import { Tag } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import { useComputeHealth, useCredential, useIdentity, useModels, usePlatform, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { LANGUAGES, LANGUAGE_NAMES, type Language, type TranslationKey } from '../i18n';
import { classificationTone, statusTone, toneFor } from '../theme';
import { SCOPE_MODELS_READ } from '../Connection';

/** Every scope the platform defines, so a key can be read against the whole list. */
const ALL_SCOPES = [
  'models:read',
  'assistants:read',
  'chat:completions',
  'knowledge:read',
  'knowledge:write',
  'tools:read',
  'agent:run',
  'admin:keys',
  'admin:assistants',
] as const;

/**
 * Language, credential and what this browser is actually talking to.
 *
 * The scope table lists every scope the platform defines rather than only the
 * ones this key holds: "you do not have this" is the answer somebody comes to
 * this page for, and a list of what you do have cannot give it.
 */
export function Settings({ onConnect }: { onConnect: () => void }) {
  const { t, language, setLanguage, formatNumber } = useTranslation();
  const [credential] = useCredential();
  const identity = useIdentity();
  const platform = usePlatform();
  const { has } = useScopes();
  const models = useModels(has(SCOPE_MODELS_READ));
  const compute = useComputeHealth(credential !== '');

  return (
    <>
      <section className="page-intro">
        <p>{t('settings.intro')}</p>
        <span>{t('settings.note')}</span>
      </section>

      <section className="settings-grid">
        <section className="panel">
          <span className="panel-label">{t('settings.language')}</span>
          <p className="history-hint">{t('settings.language.body')}</p>
          <SearchableSelect
            label={t('lang.label')}
            value={language}
            options={LANGUAGES.map((option) => ({
              value: option,
              label: LANGUAGE_NAMES[option],
              detail: option,
            }))}
            onChange={(next) => setLanguage(next as Language)}
          />
        </section>

        <section className="panel">
          <span className="panel-label">{t('settings.credential')}</span>
          {credential === '' ? (
            <p className="history-hint">{t('conn.notConnected')}</p>
          ) : (
            <dl className="detail-pairs">
              <dt>{t('settings.keyName')}</dt>
              <dd>{identity.data?.name ?? '—'}</dd>
              <dt>{t('settings.keyId')}</dt>
              <dd>
                <code>{identity.data?.keyId ?? '—'}</code>
              </dd>
              <dt>{t('settings.company')}</dt>
              <dd>{identity.data?.companyId || t('settings.allCompanies')}</dd>
              <dt>{t('settings.department')}</dt>
              <dd>{identity.data?.department || t('settings.allDepartments')}</dd>
              <dt>{t('home.stat.clearance')}</dt>
              <dd>
                <Tag tone={toneFor(classificationTone, identity.data?.maxClassification)}>
                  {identity.data?.maxClassification ?? '—'}
                </Tag>
              </dd>
              <dt>{t('settings.rateLimit')}</dt>
              <dd>
                {identity.data
                  ? t('settings.perMinute', { count: formatNumber(identity.data.rateLimitPerMinute) })
                  : '—'}
              </dd>
            </dl>
          )}
          <button className="secondary" onClick={onConnect}>
            {credential === '' ? t('conn.addKey') : t('action.changeKey')}
          </button>
        </section>

        <section className="panel">
          <span className="panel-label">{t('settings.gateway')}</span>
          <dl className="detail-pairs">
            <dt>{t('settings.endpoint')}</dt>
            <dd>
              <code>{import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'}</code>
            </dd>
            <dt>{t('stat.environment')}</dt>
            <dd>{platform.data?.environment ?? '—'}</dd>
            <dt>{t('settings.version')}</dt>
            <dd>{platform.data?.version ?? '—'}</dd>
            <dt>{t('stat.compute')}</dt>
            <dd>
              <Tag tone={toneFor(statusTone, compute.data?.status)}>{compute.data?.status ?? '—'}</Tag>
            </dd>
            <dt>{t('stat.models')}</dt>
            <dd>
              {models.isSuccess
                ? `${formatNumber(models.data.models.filter((entry) => entry.available).length)}/${formatNumber(models.data.models.length)}`
                : '—'}
            </dd>
          </dl>
        </section>
      </section>

      <section className="panel">
        <span className="panel-label">{t('settings.scopes')}</span>
        <p className="history-hint">{t('settings.scopes.body')}</p>
        <ul className="scope-table">
          {ALL_SCOPES.map((scope) => (
            <li key={scope} className={has(scope) ? 'granted' : 'withheld'}>
              <span>{has(scope) ? '✓' : '·'}</span>
              <code>{scope}</code>
              <small>{t(`scope.${scope}` as TranslationKey)}</small>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}
