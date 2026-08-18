import { useRef, useState } from 'react';
import { streamChat, type TokenUsage } from '../api';
import { EmptyState } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import { useAssistants, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import type { TranslationKey } from '../i18n';
import { LANGUAGES, LANGUAGE_NAMES } from '../i18n';
import { SCOPE_ASSISTANTS_READ, SCOPE_CHAT } from '../Connection';

const KINDS = ['report', 'document', 'email', 'presentation', 'code'] as const;
type Kind = (typeof KINDS)[number];

const LENGTHS = ['short', 'medium', 'long'] as const;
type Length = (typeof LENGTHS)[number];

const TONES = ['neutral', 'formal', 'friendly', 'technical'] as const;
type Tone = (typeof TONES)[number];

/**
 * The Create workspace (ARCHITECTURE-v1 section 11).
 *
 * Everything here is text the platform can actually produce today. Section 12
 * also lists image and chart generation; neither is offered, because the compute
 * plane runs one 0.5B text model on a 2 GB card and there is no image model to
 * route to. The page says so rather than showing controls that would fail.
 */
export function Create() {
  const { t, formatNumber } = useTranslation();
  const { has } = useScopes();
  const assistants = useAssistants(has(SCOPE_ASSISTANTS_READ));

  const [kind, setKind] = useState<Kind>('report');
  const [brief, setBrief] = useState('');
  const [audience, setAudience] = useState('');
  const [tone, setTone] = useState<Tone>('neutral');
  const [length, setLength] = useState<Length>('medium');
  const [outputLanguage, setOutputLanguage] = useState<string>('');
  const [model, setModel] = useState('');

  const [draft, setDraft] = useState('');
  const [usage, setUsage] = useState<TokenUsage | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const abort = useRef<AbortController | null>(null);

  if (!has(SCOPE_CHAT)) {
    return <EmptyState title={t('create.scope.title')} body={t('create.scope.body')} />;
  }

  const generate = async () => {
    if (brief.trim() === '' || busy) return;
    setBusy(true);
    setError('');
    setDraft('');
    setUsage(null);

    const controller = new AbortController();
    abort.current = controller;
    let reply = '';

    const request = [
      t(`create.prompt.${kind}` as TranslationKey),
      t('create.prompt.brief', { brief: brief.trim() }),
      audience.trim() === '' ? '' : t('create.prompt.audience', { audience: audience.trim() }),
      t('create.prompt.tone', { tone: t(`create.tone.${tone}` as TranslationKey) }),
      t('create.prompt.length', { length: t(`create.length.${length}` as TranslationKey) }),
      outputLanguage === ''
        ? ''
        : t('create.prompt.language', { language: LANGUAGE_NAMES[outputLanguage as never] }),
    ]
      .filter((line) => line !== '')
      .join('\n');

    try {
      await streamChat(
        {
          // Stateless, like Analyze: a draft is a work product, not a
          // conversation, and it should not appear in somebody's history.
          messages: [
            { role: 'system', content: t('create.system') },
            { role: 'user', content: request },
          ],
          ...(model === '' ? {} : { model }),
        },
        {
          onContent: (delta) => {
            reply += delta;
            setDraft(reply);
          },
          onDone: (_finishReason, tokens) => setUsage(tokens ?? null),
        },
        controller.signal,
      );
    } catch (failed) {
      if (!controller.signal.aborted) {
        setError(failed instanceof Error ? failed.message : t('create.error'));
      }
    } finally {
      setBusy(false);
      abort.current = null;
    }
  };

  const download = () => {
    // A draft that only exists in a browser tab is not a deliverable. The
    // extension follows the kind so a code draft does not arrive as .md.
    const extension = kind === 'code' ? 'txt' : 'md';
    const blob = new Blob([draft], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${kind}-${new Date().toISOString().slice(0, 10)}.${extension}`;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      <section className="page-intro">
        <p>{t('create.intro')}</p>
        <span>{t('create.note')}</span>
      </section>

      <section className="panel">
        <span className="panel-label">{t('create.kind')}</span>
        <div className="mode-switch wrap" role="tablist" aria-label={t('create.kind')}>
          {KINDS.map((option) => (
            <button
              key={option}
              role="tab"
              aria-selected={kind === option}
              className={kind === option ? 'active' : ''}
              onClick={() => setKind(option)}
            >
              {t(`create.kind.${option}`)}
            </button>
          ))}
        </div>

        <label className="field">
          <span>{t('create.brief')}</span>
          <textarea
            rows={4}
            value={brief}
            placeholder={t(`create.brief.hint.${kind}`)}
            onChange={(event) => setBrief(event.target.value)}
          />
        </label>

        <label className="field">
          <span>{t('create.audience')}</span>
          <input
            value={audience}
            placeholder={t('create.audience.hint')}
            onChange={(event) => setAudience(event.target.value)}
          />
        </label>

        <div className="field-row">
          <SearchableSelect
            label={t('create.tone')}
            value={tone}
            options={TONES.map((option) => ({
              value: option,
              label: t(`create.tone.${option}`),
            }))}
            onChange={(next) => setTone(next as Tone)}
          />
          <SearchableSelect
            label={t('create.length')}
            value={length}
            options={LENGTHS.map((option) => ({
              value: option,
              label: t(`create.length.${option}`),
            }))}
            onChange={(next) => setLength(next as Length)}
          />
        </div>

        <div className="field-row">
          <SearchableSelect
            label={t('create.language')}
            value={outputLanguage}
            placeholder={t('create.language.default')}
            options={LANGUAGES.map((option) => ({
              value: option,
              label: LANGUAGE_NAMES[option],
              detail: option,
            }))}
            onChange={setOutputLanguage}
          />
          <SearchableSelect
            label={t('analyze.model')}
            value={model}
            placeholder={t('analyze.defaultModel')}
            options={(assistants.data ?? []).map((entry) => ({
              value: entry.logicalModel,
              label: entry.name,
              detail: entry.logicalModel,
            }))}
            onChange={setModel}
          />
        </div>

        <div className="modal-actions">
          {busy ? (
            <button className="secondary" onClick={() => abort.current?.abort()}>
              {t('action.stop')}
            </button>
          ) : (
            <button className="primary" disabled={brief.trim() === ''} onClick={() => void generate()}>
              {t('create.generate')}
            </button>
          )}
        </div>

        <p className="source-note">{t('create.unavailable')}</p>
      </section>

      {error !== '' && <p className="error-note">{error}</p>}

      {(draft !== '' || busy) && (
        <section className="panel result-panel">
          <header className="panel-head">
            <span className="panel-label">{t('create.draft')}</span>
            <span className="result-actions">
              <button className="text-button" onClick={() => void navigator.clipboard.writeText(draft)}>
                {t('action.copy')}
              </button>
              <button className="text-button" disabled={busy || draft === ''} onClick={download}>
                {t('action.download')}
              </button>
            </span>
          </header>
          <p className="result-body">
            {draft}
            {busy && <i className="caret" />}
          </p>
          {usage && (
            <small>
              {t('chat.usage', {
                total: formatNumber(usage.totalTokens),
                input: formatNumber(usage.promptTokens),
                output: formatNumber(usage.completionTokens),
              })}
            </small>
          )}
        </section>
      )}
    </>
  );
}
