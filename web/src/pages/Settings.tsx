import { useState } from 'react';
import { Tag } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import {
  useComputeHealth,
  useCredential,
  useIdentity,
  useModels,
  usePlatform,
  usePlatformSettings,
  usePromptTemplates,
  useRefreshSettings,
  useScopes,
} from '../hooks';
import { useTheme } from '@/contexts/ThemeContext';
import { useTranslation } from '../LanguageContext';
import { LANGUAGES, LANGUAGE_NAMES, type Language, type TranslationKey } from '../i18n';
import { classificationTone, statusTone, toneFor } from '../theme';
import { SCOPE_ADMIN_KEYS, SCOPE_MODELS_READ } from '../Connection';
import { updatePlatformSetting } from '../api';

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

export function Settings({ onConnect }: { onConnect: () => void }) {
  const { t, language, setLanguage, formatNumber } = useTranslation();
  const { theme, setTheme } = useTheme();
  const [credential] = useCredential();
  const identity = useIdentity();
  const platform = usePlatform();
  const { has } = useScopes();
  const canAdmin = has(SCOPE_ADMIN_KEYS);
  const models = useModels(has(SCOPE_MODELS_READ));
  const compute = useComputeHealth(credential !== '');
  const settingsQuery = usePlatformSettings(canAdmin);
  const promptsQuery = usePromptTemplates(credential !== '');
  const refreshSettings = useRefreshSettings();

  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState<string>('');
  const [saveBusy, setSaveBusy] = useState<boolean>(false);

  const handleStartEdit = (key: string, currentValue: string) => {
    setEditingKey(key);
    setEditValue(currentValue);
  };

  const handleSaveSetting = async (key: string, description: string) => {
    setSaveBusy(true);
    try {
      let parsedValue: any = editValue;
      try {
        parsedValue = JSON.parse(editValue);
      } catch {
        // If not valid JSON, pass as string
      }
      await updatePlatformSetting(key, parsedValue, description);
      refreshSettings();
      setEditingKey(null);
    } catch (err) {
      console.error('Failed to update setting', err);
    } finally {
      setSaveBusy(false);
    }
  };

  return (
    <>
      <section className="page-intro">
        <p>{t('settings.intro')}</p>
        <span>{t('settings.note')}</span>
      </section>

      <section className="settings-grid">
        <section className="panel" style={{ gridColumn: '1 / -1' }}>
          <span className="panel-label">Theme / ธีมระบบ</span>
          <p className="history-hint">เลือกรูปแบบการแสดงผลของหน้าจอ (Modern Glassmorphism, Dark, Light) เพื่อประสบการณ์การใช้งานที่ดีที่สุด</p>
          <div className="theme-card-grid">
            <button
              type="button"
              className={`theme-select-card ${theme === 'glassmorphism' ? 'active' : ''}`}
              onClick={() => setTheme('glassmorphism')}
            >
              <span className="theme-select-icon">🔮</span>
              <div>
                <strong>Modern Glassmorphism</strong>
                <small>กระจกโปร่งแสง ไล่เฉดสีม่วงนีออน (แนะนำ)</small>
              </div>
            </button>
            <button
              type="button"
              className={`theme-select-card ${theme === 'dark' ? 'active' : ''}`}
              onClick={() => setTheme('dark')}
            >
              <span className="theme-select-icon">🌙</span>
              <div>
                <strong>Dark (CHIOTRON)</strong>
                <small>ธีมมืดดั้งเดิม น้ำเงินกรมท่า คลาสสิก</small>
              </div>
            </button>
            <button
              type="button"
              className={`theme-select-card ${theme === 'light' ? 'active' : ''}`}
              onClick={() => setTheme('light')}
            >
              <span className="theme-select-icon">☀️</span>
              <div>
                <strong>Light</strong>
                <small>ธีมสว่าง สะอาด เรียบง่าย</small>
              </div>
            </button>
          </div>
        </section>

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

      {canAdmin && (
        <section className="panel">
          <span className="panel-label">{t('settings.platformSettings')}</span>
          <p className="history-hint">{t('settings.platformSettings.body')}</p>
          {settingsQuery.isLoading ? (
            <p className="history-hint">{t('table.loading')}</p>
          ) : (settingsQuery.data ?? []).length === 0 ? (
            <p className="history-hint">{t('settings.empty')}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t('settings.column.key')}</th>
                    <th>{t('settings.column.value')}</th>
                    <th>{t('settings.column.description')}</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {(settingsQuery.data ?? []).map((setting) => (
                    <tr key={setting.key}>
                      <td><code>{setting.key}</code></td>
                      <td>
                        {editingKey === setting.key ? (
                          <input
                            type="text"
                            className="input-text"
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            disabled={saveBusy}
                          />
                        ) : (
                          <code>{setting.value}</code>
                        )}
                      </td>
                      <td><small>{setting.description}</small></td>
                      <td style={{ textAlign: 'right' }}>
                        {editingKey === setting.key ? (
                          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                            <button
                              className="primary"
                              onClick={() => handleSaveSetting(setting.key, setting.description)}
                              disabled={saveBusy}
                            >
                              {t('settings.save')}
                            </button>
                            <button
                              className="secondary"
                              onClick={() => setEditingKey(null)}
                              disabled={saveBusy}
                            >
                              {t('action.cancel')}
                            </button>
                          </div>
                        ) : (
                          <button
                            className="secondary"
                            onClick={() => handleStartEdit(setting.key, setting.value)}
                          >
                            {t('settings.edit')}
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      {credential !== '' && (
        <section className="panel">
          <span className="panel-label">{t('settings.promptTemplates')}</span>
          <p className="history-hint">{t('settings.promptTemplates.body')}</p>
          {promptsQuery.isLoading ? (
            <p className="history-hint">{t('table.loading')}</p>
          ) : (promptsQuery.data ?? []).length === 0 ? (
            <p className="history-hint">{t('settings.emptyPrompts')}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t('settings.column.name')}</th>
                    <th>{t('settings.column.slug')}</th>
                    <th>{t('settings.column.template')}</th>
                  </tr>
                </thead>
                <tbody>
                  {(promptsQuery.data ?? []).map((tpl) => (
                    <tr key={tpl.id}>
                      <td>
                        <span className="cell-stack">
                          <b>{tpl.name}</b>
                          <small>{tpl.description}</small>
                        </span>
                      </td>
                      <td><code>{tpl.slug}</code></td>
                      <td><small style={{ whiteSpace: 'pre-wrap' }}>{tpl.template}</small></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

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
