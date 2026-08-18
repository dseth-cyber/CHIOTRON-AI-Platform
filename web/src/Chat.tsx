import { useEffect, useRef, useState } from 'react';
import { ApiError, streamChat, type TokenUsage } from './api';
import { SearchableSelect } from './components/SearchableSelect';
import { FavoriteButton } from './components/FavoriteButton';
import { EmptyState } from './components/EmptyState';
import { useAssistants, useConversation, useFavorites, useRefreshHistory, useScopes } from './hooks';
import { useTranslation } from './LanguageContext';
import { SCOPE_CHAT } from './Connection';
import type { ChatTarget, Navigate } from './navigation';

type Turn = {
  role: 'user' | 'assistant';
  content: string;
  redacted?: boolean;
  usage?: TokenUsage;
  finishReason?: string;
  latencyMs?: number;
};

/**
 * The chat workspace.
 *
 * History lives on its own page (ARCHITECTURE-v1 section 10), so this is only
 * the transcript and the composer. It opens either on a stored conversation or
 * on an assistant, whichever the caller navigated with.
 */
export function ChatWorkspace({
  target,
  onConnect,
  onNavigate,
}: {
  target: ChatTarget | null;
  onConnect: () => void;
  onNavigate: Navigate;
}) {
  const { t, formatNumber } = useTranslation();
  const { has } = useScopes();
  const assistants = useAssistants(true);
  const favorites = useFavorites(has(SCOPE_CHAT));
  const refreshHistory = useRefreshHistory();

  const [assistant, setAssistant] = useState(target?.assistant ?? '');
  const [conversationId, setConversationId] = useState<string | null>(target?.conversationId ?? null);
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
    if (detail.data.conversation.assistantSlug) setAssistant(detail.data.conversation.assistantSlug);
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
      <EmptyState
        title={rejected ? t('chat.error.scope.title') : t('chat.error.catalogue.title')}
        body={rejected ? t('chat.error.scope.body') : assistants.error.message}
        action={
          <button className="primary" onClick={onConnect}>
            {t('action.changeKey')}
          </button>
        }
      />
    );
  }

  const selected = assistants.data?.find((entry) => entry.slug === assistant);
  const marked = new Set(
    (favorites.data ?? []).filter((mark) => mark.kind === 'conversation').map((mark) => mark.targetId),
  );
  const title = detail.data?.conversation.title ?? '';

  return (
    <section className="chat-page">
      <header className="chat-bar">
        <SearchableSelect
          label={t('chat.assistant')}
          value={assistant}
          // An existing conversation is bound to the assistant it started with,
          // so switching is only possible on a new one.
          disabled={conversationId !== null || streaming}
          options={(assistants.data ?? []).map((entry) => ({
            value: entry.slug,
            label: entry.name,
            detail: entry.logicalModel,
            disabled: !entry.enabled,
          }))}
          onChange={setAssistant}
        />
        <span className="chat-meta">
          {selected
            ? t('chat.assistantMeta', {
                description: selected.description,
                model: selected.logicalModel,
              })
            : ''}
        </span>
        <div className="chat-tools">
          {conversationId !== null && (
            <FavoriteButton
              kind="conversation"
              targetId={conversationId}
              marked={marked.has(conversationId)}
              label={title || t('chat.untitled')}
            />
          )}
          <button className="text-button" onClick={() => onNavigate('history')}>
            {t('chat.history')} <span>→</span>
          </button>
          <button className="secondary" onClick={startNew} disabled={conversationId === null && turns.length === 0}>
            {t('action.newChat')}
          </button>
        </div>
      </header>

      {conversationId !== null && title !== '' && <p className="chat-title">{title}</p>}

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
            {!turn.usage && turn.finishReason === 'cancelled' && <small>{t('chat.cancelled')}</small>}
          </article>
        ))}
      </div>

      {error !== '' && <p className="error-note">{error}</p>}
      {detail.data?.promptsPersisted === false && <p className="source-note">{t('chat.promptsOff')}</p>}

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
    </section>
  );
}
