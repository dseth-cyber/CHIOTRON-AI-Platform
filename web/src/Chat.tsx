import { useEffect, useRef, useState } from 'react';
import { ApiError, streamChat, type ChatMessage, type TokenUsage } from './api';
import { useModels } from './hooks';

type Turn = ChatMessage & { usage?: TokenUsage; finishReason?: string; latencyMs?: number };

export function ChatWorkspace({ onConnect }: { onConnect: () => void }) {
  const models = useModels(true);
  const [model, setModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [turns, setTurns] = useState<Turn[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState<string>('');
  const abort = useRef<AbortController | null>(null);
  const transcript = useRef<HTMLDivElement>(null);

  // Default to whatever the gateway says is the default route, so the portal
  // never hard-codes a model name.
  useEffect(() => {
    if (model === '' && models.data) setModel(models.data.default);
  }, [model, models.data]);

  useEffect(() => {
    transcript.current?.scrollTo({ top: transcript.current.scrollHeight });
  }, [turns]);

  // Abandon an in-flight stream if the workspace goes away.
  useEffect(() => () => abort.current?.abort(), []);

  const send = async () => {
    const question = prompt.trim();
    if (question === '' || streaming) return;

    const history: Turn[] = [...turns, { role: 'user', content: question }];
    setTurns([...history, { role: 'assistant', content: '' }]);
    setPrompt('');
    setError('');
    setStreaming(true);

    const controller = new AbortController();
    abort.current = controller;
    const started = performance.now();

    // The reply is built up locally and written into the last turn on each
    // frame, so React re-renders with the text as it arrives.
    let reply = '';
    try {
      await streamChat(
        {
          model,
          messages: history.map(({ role, content }) => ({ role, content })),
          temperature: 0.2,
        },
        {
          onContent: (delta) => {
            reply += delta;
            setTurns((current) =>
              current.map((turn, index) =>
                index === current.length - 1 ? { ...turn, content: reply } : turn,
              ),
            );
          },
          onDone: (finishReason, usage) => {
            const latencyMs = Math.round(performance.now() - started);
            setTurns((current) =>
              current.map((turn, index) =>
                index === current.length - 1
                  ? { ...turn, content: reply, usage, finishReason, latencyMs }
                  : turn,
              ),
            );
          },
        },
        controller.signal,
      );
    } catch (failed) {
      if (controller.signal.aborted) {
        setTurns((current) =>
          current.map((turn, index) =>
            index === current.length - 1 ? { ...turn, finishReason: 'cancelled' } : turn,
          ),
        );
      } else {
        setError(failed instanceof Error ? failed.message : 'the request failed');
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

  if (models.isError) {
    const rejected = models.error instanceof ApiError && models.error.status === 403;
    return (
      <section className="chat-empty">
        <h2>{rejected ? 'This key cannot run completions' : 'Cannot reach the model catalogue'}</h2>
        <p>
          {rejected
            ? 'The connected key is missing the chat:completions scope. Connect a key that has it.'
            : models.error.message}
        </p>
        <button className="primary" onClick={onConnect}>
          Change key
        </button>
      </section>
    );
  }

  return (
    <section className="chat">
      <header className="chat-bar">
        <label className="field inline">
          <span>Model</span>
          <select value={model} onChange={(event) => setModel(event.target.value)}>
            {(models.data?.models ?? []).map((entry) => (
              <option key={entry.logical} value={entry.logical} disabled={!entry.available}>
                {entry.logical} · {entry.provider}/{entry.model}
                {entry.available ? '' : ' (not loaded)'}
              </option>
            ))}
          </select>
        </label>
        <button className="secondary" onClick={() => setTurns([])} disabled={turns.length === 0}>
          Clear
        </button>
      </header>

      <div className="transcript" ref={transcript}>
        {turns.length === 0 && (
          <p className="transcript-hint">
            Messages are sent through the AI Gateway. Nothing here reaches a model provider
            directly.
          </p>
        )}
        {turns.map((turn, index) => (
          <article className={`turn ${turn.role}`} key={index}>
            <span className="turn-role">{turn.role === 'user' ? 'You' : 'Assistant'}</span>
            <p>
              {turn.content}
              {streaming && index === turns.length - 1 && <i className="caret" />}
            </p>
            {turn.usage && (
              <small>
                {turn.usage.totalTokens} tokens ({turn.usage.promptTokens} in ·{' '}
                {turn.usage.completionTokens} out) · {turn.latencyMs} ms · {turn.finishReason}
              </small>
            )}
            {!turn.usage && turn.finishReason === 'cancelled' && <small>cancelled</small>}
          </article>
        ))}
      </div>

      {error !== '' && <p className="error-note">{error}</p>}

      <div className="composer">
        <textarea
          value={prompt}
          rows={3}
          placeholder="Ask something…"
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
            Stop
          </button>
        ) : (
          <button className="primary" onClick={() => void send()} disabled={prompt.trim() === ''}>
            Send
          </button>
        )}
      </div>
    </section>
  );
}
