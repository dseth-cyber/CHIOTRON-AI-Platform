import { useEffect, useRef, useState } from 'react';
import {
  ApiError,
  deleteConversation,
  streamChat,
  type ConversationSummary,
  type TokenUsage,
} from './api';
import { useAssistants, useConversation, useConversations, useRefreshHistory } from './hooks';
import { useTranslation } from './LanguageContext';

type Turn = {
  role: 'user' | 'assistant';
  content: string;
  redacted?: boolean;
  usage?: TokenUsage;
  finishReason?: string;
  latencyMs?: number;
};

export function ChatWorkspace({ onConnect }: { onConnect: () => void }) {
  const { t, formatNumber } = useTranslation();
  const assistants = useAssistants(true);
  const history = useConversations(true);
  const refreshHistory = useRefreshHistory();

  const [assistant, setAssistant] = useState('');
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [prompt, setPrompt] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState('');

  const detail = useConversation(conversationId);
  const abort = useRef<AbortController | null>(null);
  const transcript = useRef<HTMLDivElement>(null);
  // Which conversation the visible turns were loaded from, so a background
  // refetch cannot overwrite a reply that is still streaming.
  const hydratedFor = useRef<string | null>(null);

  useEffect(() => {
    if (assistant === '' && assistants.data && assistants.data.length > 0) {
      setAssistant(assistants.data[0]!.slug);
    }
  }, [assistant, assistants.data]);

  useEffect(() => {
    if (conversationId === null || detail.data === undefined) return;
    if (hydratedFor.current === conversationId) return;
    hydratedFor.current = conversationId;
    setTurns(
      detail.data.messages.map((message) => ({
        role: message.role,
        content: message.content,
        redacted: message.redacted,
        usage: message.completionTokens
          ? {
              promptTokens: message.promptTokens ?? 0,
              completionTokens: message.completionTokens,
              totalTokens: (message.promptTokens ?? 0) + message.completionTokens,
            }
          : undefined,
      })),
    );
  }, [conversationId, detail.data]);

  useEffect(() => {
    transcript.current?.scrollTo({ top: transcript.current.scrollHeight });
  }, [turns]);

  useEffect(() => () => abort.current?.abort(), []);

  const startNew = () => {
    abort.current?.abort();
    hydratedFor.current = null;
    setConversationId(null);
    setTurns([]);
    setError('');
  };

  const open = (summary: ConversationSummary) => {
    abort.current?.abort();
    setError('');
    if (summary.assistantSlug) setAssistant(summary.assistantSlug);
    setConversationId(summary.id);
  };

  const remove = async (summary: ConversationSummary) => {
    try {
      await deleteConversation(summary.id);
      if (conversationId === summary.id) startNew();
      refreshHistory();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('chat.error.deleteFailed'));
    }
  };

  const send = async () => {
    const question = prompt.trim();
    if (question === '' || streaming || assistant === '') return;

    setTurns((current) => [
      ...current,
      { role: 'user', content: question },
      { role: 'assistant', content: '' },
    ]);
    setPrompt('');
    setError('');
    setStreaming(true);

    const controller = new AbortController();
    abort.current = controller;
    const started = performance.now();

    // The reply is accumulated locally and written into the last turn on each
    // frame, so React re-renders with the text as it arrives.
    let reply = '';
    const patchLast = (patch: Partial<Turn>) =>
      setTurns((current) =>
        current.map((turn, index) => (index === current.length - 1 ? { ...turn, ...patch } : turn)),
      );

    try {
      await streamChat(
        // conversationId is omitted for a new conversation; the assistant then
        // decides the model and the instructions.
        conversationId === null
          ? { assistant, message: question }
          : { conversationId, message: question },
        {
          onConversation: (id) => {
            // Claim the id before any content so a refetch does not re-hydrate
            // over the reply currently streaming in.
            hydratedFor.current = id;
            setConversationId(id);
          },
          onContent: (delta) => {
            reply += delta;
            patchLast({ content: reply });
          },
          onDone: (finishReason, usage) => {
            patchLast({
              content: reply,
              usage,
              finishReason,
              latencyMs: Math.round(performance.now() - started),
            });
          },
        },
        controller.signal,
      );
      refreshHistory();
    } catch (failed) {
      if (controller.signal.aborted) {
        patchLast({ finishReason: 'cancelled' });
      } else {
        setError(failed instanceof Error ? failed.message : t('chat.error.failed'));
        // Drop the empty assistant turn rather than leaving a blank bubble.
        setTurns((current) =>
          current.filter((turn, index) => index !== current.length - 1 || turn.content !== ''),
        );
      }
    } finally {
      setStreaming(false);
      abort.current = null;
    }
  };

  if (assistants.isError) {
    const rejected = assistants.error instanceof ApiError && assistants.error.status === 403;
    return (
      <section className="chat-empty">
        <h2>{rejected ? t('chat.error.scope.title') : t('chat.error.catalogue.title')}</h2>
        <p>{rejected ? t('chat.error.scope.body') : assistants.error.message}</p>
        <button className="primary" onClick={onConnect}>
          {t('action.changeKey')}
        </button>
      </section>
    );
  }

  const selected = assistants.data?.find((entry) => entry.slug === assistant);
  const conversations = history.data?.conversations ?? [];

  return (
    <section className="workspace">
      <aside className="history">
        <div className="history-head">
          <span className="panel-label">{t('chat.history')}</span>
          <button className="text-button" onClick={startNew}>
            {t('action.newChat')} <span>+</span>
          </button>
        </div>
        {history.isError && <p className="error-note">{history.error.message}</p>}
        {conversations.length === 0 && !history.isPending && (
          <p className="history-hint">{t('chat.historyEmpty')}</p>
        )}
        <ul className="history-list">
          {conversations.map((summary) => {
            const title = summary.title || t('chat.untitled');
            return (
              <li key={summary.id} className={summary.id === conversationId ? 'current' : ''}>
                <button className="history-item" onClick={() => open(summary)}>
                  <b>{title}</b>
                  <small>
                    {t('chat.summary', {
                      assistant: summary.assistantName ?? t('chat.unknownAssistant'),
                      messages: formatNumber(summary.messageCount),
                      tokens: formatNumber(summary.totalTokens),
                    })}
                  </small>
                </button>
                <button
                  className="history-delete"
                  aria-label={t('chat.delete', { title })}
                  onClick={() => void remove(summary)}
                >
                  ×
                </button>
              </li>
            );
          })}
        </ul>
        {history.data?.promptsPersisted === false && (
          <p className="history-hint">{t('chat.promptsOff')}</p>
        )}
      </aside>

      <div className="chat">
        <header className="chat-bar">
          <label className="field inline">
            <span>{t('chat.assistant')}</span>
            <select
              value={assistant}
              // An existing conversation is bound to the assistant it started
              // with, so switching is only possible on a new one.
              disabled={conversationId !== null || streaming}
              onChange={(event) => setAssistant(event.target.value)}
            >
              {(assistants.data ?? []).map((entry) => (
                <option key={entry.slug} value={entry.slug}>
                  {entry.name}
                </option>
              ))}
            </select>
          </label>
          <span className="chat-meta">
            {selected
              ? t('chat.assistantMeta', {
                  description: selected.description,
                  model: selected.logicalModel,
                })
              : ''}
          </span>
        </header>

        <div className="transcript" ref={transcript}>
          {turns.length === 0 && <p className="transcript-hint">{t('chat.transcriptHint')}</p>}
          {turns.map((turn, index) => (
            <article className={`turn ${turn.role}`} key={index}>
              <span className="turn-role">
                {turn.role === 'user' ? t('chat.you') : t('chat.assistantRole')}
              </span>
              <p>
                {turn.redacted ? <i className="redacted">{t('chat.notStored')}</i> : turn.content}
                {streaming && index === turns.length - 1 && <i className="caret" />}
              </p>
              {turn.usage && (
                <small>
                  {t('chat.usage', {
                    total: formatNumber(turn.usage.totalTokens),
                    input: formatNumber(turn.usage.promptTokens),
                    output: formatNumber(turn.usage.completionTokens),
                  })}
                  {turn.latencyMs ? ` · ${formatNumber(turn.latencyMs)} ms` : ''}
                  {turn.finishReason ? ` · ${turn.finishReason}` : ''}
                </small>
              )}
              {!turn.usage && turn.finishReason === 'cancelled' && (
                <small>{t('chat.cancelled')}</small>
              )}
            </article>
          ))}
        </div>

        {error !== '' && <p className="error-note">{error}</p>}

        <div className="composer">
          <textarea
            value={prompt}
            rows={3}
            placeholder={
              selected ? t('chat.placeholderNamed', { name: selected.name }) : t('chat.placeholder')
            }
            onChange={(event) => setPrompt(event.target.value)}
            onKeyDown={(event) => {
              // Enter sends, Shift+Enter starts a new line.
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault();
                void send();
              }
            }}
          />
          {streaming ? (
            <button className="secondary" onClick={() => abort.current?.abort()}>
              {t('action.stop')}
            </button>
          ) : (
            <button
              className="primary"
              onClick={() => void send()}
              disabled={prompt.trim() === '' || assistant === ''}
            >
              {t('action.send')}
            </button>
          )}
        </div>
      </div>
    </section>
  );
}
