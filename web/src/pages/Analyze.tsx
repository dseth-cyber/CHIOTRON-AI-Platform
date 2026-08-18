import { useRef, useState } from 'react';
import { streamChat, type TokenUsage } from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import { useAssistants, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import type { TranslationKey } from '../i18n';
import { SCOPE_ASSISTANTS_READ, SCOPE_CHAT } from '../Connection';

/**
 * How much extracted text is sent to the model.
 *
 * The gateway caps a chat body at 1 MiB, and the development model is a 0.5B
 * with a small context anyway, so a long file is truncated and the page says so.
 * Silently sending a prefix and presenting the answer as covering the whole file
 * would be a lie about what was read.
 */
const MAX_CHARACTERS = 40_000;

/** What the browser can turn into text without a parsing dependency. */
const READABLE = ['.txt', '.md', '.csv', '.tsv', '.json', '.log'];

const TASKS = ['summarize', 'keyPoints', 'translate', 'rewrite', 'table', 'ask'] as const;
type Task = (typeof TASKS)[number];

type Loaded = {
  name: string;
  bytes: number;
  text: string;
  truncated: boolean;
  /** Present only for delimited files, so the page can show the data as a table. */
  table?: { headers: string[]; rows: string[][] };
};

/**
 * The Analyze workspace (ARCHITECTURE-v1 section 11).
 *
 * A file is read in the browser and never uploaded: analysing something is not
 * the same as filing it in the corpus, and a document somebody only wanted a
 * summary of should not acquire a classification, an owner and a permanent row.
 */
export function Analyze() {
  const { t, formatNumber } = useTranslation();
  const { has } = useScopes();
  const assistants = useAssistants(has(SCOPE_ASSISTANTS_READ));

  const [file, setFile] = useState<Loaded | null>(null);
  const [model, setModel] = useState('');
  const [task, setTask] = useState<Task>('summarize');
  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState('');
  const [usage, setUsage] = useState<TokenUsage | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const abort = useRef<AbortController | null>(null);

  if (!has(SCOPE_CHAT)) {
    return <EmptyState title={t('analyze.scope.title')} body={t('analyze.scope.body')} />;
  }

  const load = async (picked: File | undefined) => {
    if (!picked) return;
    setError('');
    setAnswer('');
    setUsage(null);

    const extension = picked.name.slice(picked.name.lastIndexOf('.')).toLowerCase();
    if (!READABLE.includes(extension)) {
      setFile(null);
      setError(t('analyze.unsupported', { extension, list: READABLE.join(' ') }));
      return;
    }

    const raw = await picked.text();
    const truncated = raw.length > MAX_CHARACTERS;
    setFile({
      name: picked.name,
      bytes: picked.size,
      text: truncated ? raw.slice(0, MAX_CHARACTERS) : raw,
      truncated,
      table:
        extension === '.csv' || extension === '.tsv' ? parseDelimited(raw, extension) : undefined,
    });
  };

  const run = async () => {
    if (!file || busy) return;
    setBusy(true);
    setError('');
    setAnswer('');
    setUsage(null);

    const controller = new AbortController();
    abort.current = controller;
    let reply = '';

    try {
      await streamChat(
        {
          // Stateless: the transcript stays in this tab. Analysing a file is not
          // a conversation somebody should later find in their history.
          messages: [
            { role: 'system', content: t('analyze.system') },
            {
              role: 'user',
              content: [
                instructionFor(task, question, t),
                '',
                '--- ' + file.name + ' ---',
                file.text,
              ].join('\n'),
            },
          ],
          ...(model === '' ? {} : { model }),
        },
        {
          onContent: (delta) => {
            reply += delta;
            setAnswer(reply);
          },
          onDone: (_finishReason, tokens) => setUsage(tokens ?? null),
        },
        controller.signal,
      );
    } catch (failed) {
      if (!controller.signal.aborted) {
        setError(failed instanceof Error ? failed.message : t('analyze.error'));
      }
    } finally {
      setBusy(false);
      abort.current = null;
    }
  };

  const columns: Column<string[]>[] =
    file?.table?.headers.map((header, index) => ({
      key: String(index),
      header: header || t('analyze.column', { index: index + 1 }),
      sortValue: (row) => row[index] ?? '',
      cell: (row) => row[index] ?? '',
    })) ?? [];

  return (
    <>
      <section className="page-intro">
        <p>{t('analyze.intro')}</p>
        <span>{t('analyze.note')}</span>
      </section>

      <section className="panel">
        <span className="panel-label">{t('analyze.source')}</span>
        <label className="field">
          <span>{t('analyze.pick', { list: READABLE.join(' ') })}</span>
          <input
            type="file"
            accept={READABLE.join(',')}
            onChange={(event) => void load(event.target.files?.[0])}
          />
        </label>
        {file && (
          <p className="history-hint">
            {t('analyze.loaded', {
              name: file.name,
              kb: formatNumber(Math.max(1, Math.round(file.bytes / 1024))),
              characters: formatNumber(file.text.length),
            })}
            {file.truncated && (
              <b className="warn">
                {' '}
                {t('analyze.truncated', { limit: formatNumber(MAX_CHARACTERS) })}
              </b>
            )}
          </p>
        )}
        <p className="source-note">{t('analyze.unavailable')}</p>
      </section>

      {file?.table && (
        <>
          <p className="source-note">
            {t('analyze.tablePreview', {
              rows: formatNumber(file.table.rows.length),
              columns: formatNumber(file.table.headers.length),
            })}
          </p>
          <DataTable
            columns={columns}
            rows={file.table.rows}
            rowKey={(row) => row.join('')}
            empty={t('analyze.tableEmpty')}
          />
        </>
      )}

      <section className="panel analyze-controls">
        <span className="panel-label">{t('analyze.task')}</span>
        <div className="mode-switch wrap" role="tablist" aria-label={t('analyze.task')}>
          {TASKS.map((option) => (
            <button
              key={option}
              role="tab"
              aria-selected={task === option}
              className={task === option ? 'active' : ''}
              onClick={() => setTask(option)}
            >
              {t(`analyze.task.${option}`)}
            </button>
          ))}
        </div>

        {task === 'ask' && (
          <label className="field">
            <span>{t('analyze.question')}</span>
            <input
              value={question}
              placeholder={t('analyze.questionHint')}
              onChange={(event) => setQuestion(event.target.value)}
            />
          </label>
        )}

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

        <div className="modal-actions">
          {busy ? (
            <button className="secondary" onClick={() => abort.current?.abort()}>
              {t('action.stop')}
            </button>
          ) : (
            <button
              className="primary"
              disabled={!file || (task === 'ask' && question.trim() === '')}
              onClick={() => void run()}
            >
              {t('analyze.run')}
            </button>
          )}
        </div>
      </section>

      {error !== '' && <p className="error-note">{error}</p>}

      {(answer !== '' || busy) && (
        <section className="panel result-panel">
          <header className="panel-head">
            <span className="panel-label">{t('analyze.result')}</span>
            <button className="text-button" onClick={() => void navigator.clipboard.writeText(answer)}>
              {t('action.copy')}
            </button>
          </header>
          <p className="result-body">
            {answer}
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

/**
 * The instruction is translated, so the model is asked in the reader's own
 * language and answers in it without a separate "reply in X" instruction.
 */
function instructionFor(
  task: Task,
  question: string,
  t: (key: TranslationKey, params?: Record<string, string | number>) => string,
): string {
  if (task === 'ask') return t('analyze.prompt.ask') + ' ' + question;
  return t(`analyze.prompt.${task}`);
}

/**
 * Splits a delimited file for preview.
 *
 * Quoted fields are handled because any CSV exported from a spreadsheet will
 * contain them, and a preview that splits on a quoted comma shows the user a
 * table that does not match their file.
 */
function parseDelimited(raw: string, extension: string): Loaded['table'] {
  const delimiter = extension === '.tsv' ? '\t' : ',';
  const lines = raw.split(/\r?\n/).filter((line) => line.trim() !== '');
  if (lines.length === 0) return undefined;

  const split = (line: string): string[] => {
    const fields: string[] = [];
    let field = '';
    let quoted = false;
    for (let index = 0; index < line.length; index += 1) {
      const character = line[index]!;
      if (quoted) {
        if (character === '"' && line[index + 1] === '"') {
          field += '"';
          index += 1;
        } else if (character === '"') {
          quoted = false;
        } else {
          field += character;
        }
      } else if (character === '"') {
        quoted = true;
      } else if (character === delimiter) {
        fields.push(field);
        field = '';
      } else {
        field += character;
      }
    }
    fields.push(field);
    return fields;
  };

  return {
    headers: split(lines[0]!),
    // The preview is bounded: a 200k-row export must not try to render as DOM.
    rows: lines.slice(1, 501).map(split),
  };
}
