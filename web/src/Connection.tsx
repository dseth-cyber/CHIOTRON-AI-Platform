import { useState } from 'react';
import { ApiError } from './api';
import { Modal } from './Modal';
import { useCredential, useIdentity } from './hooks';
import { useTranslation } from './LanguageContext';
import type { TranslationKey } from './i18n';

export const SCOPE_MODELS_READ = 'models:read';
export const SCOPE_ASSISTANTS_READ = 'assistants:read';
export const SCOPE_CHAT = 'chat:completions';
export const SCOPE_KNOWLEDGE_READ = 'knowledge:read';
export const SCOPE_KNOWLEDGE_WRITE = 'knowledge:write';
export const SCOPE_TOOLS_READ = 'tools:read';
export const SCOPE_AGENT_RUN = 'agent:run';
export const SCOPE_ADMIN_KEYS = 'admin:keys';
export const SCOPE_ADMIN_ASSISTANTS = 'admin:assistants';

const KNOWN_SCOPES = [
  SCOPE_MODELS_READ,
  SCOPE_ASSISTANTS_READ,
  SCOPE_CHAT,
  SCOPE_KNOWLEDGE_READ,
  SCOPE_KNOWLEDGE_WRITE,
  SCOPE_TOOLS_READ,
  SCOPE_AGENT_RUN,
  SCOPE_ADMIN_KEYS,
  SCOPE_ADMIN_ASSISTANTS,
] as const;

/** A scope the gateway added but this build does not know is shown verbatim. */
function isKnownScope(scope: string): scope is (typeof KNOWN_SCOPES)[number] {
  return (KNOWN_SCOPES as readonly string[]).includes(scope);
}

/** Sidebar status line: is this browser talking to the Control Plane, and as whom. */
export function ConnectionBadge({ onConnect }: { onConnect: () => void }) {
  const { t } = useTranslation();
  const [credential] = useCredential();
  const identity = useIdentity();

  if (credential === '') {
    return (
      <div className="connection offline">
        <span className="live-dot offline" /> {t('conn.notConnected')}
        <button className="text-button" onClick={onConnect}>
          {t('conn.addKey')} <span>→</span>
        </button>
      </div>
    );
  }
  if (identity.isPending) {
    return (
      <div className="connection">
        <span className="live-dot pending" /> {t('conn.checking')}
      </div>
    );
  }
  if (identity.isError) {
    const rejected = identity.error instanceof ApiError && identity.error.unauthenticated;
    return (
      <div className="connection offline">
        <span className="live-dot offline" /> {rejected ? t('conn.rejected') : t('conn.unreachable')}
        <button className="text-button" onClick={onConnect}>
          {t('action.changeKey')} <span>→</span>
        </button>
      </div>
    );
  }

  return (
    <div className="connection">
      <span className="live-dot" /> {t('conn.connectedAs')} <b>{identity.data.name}</b>
      <small>
        {t('conn.summary', {
          count: identity.data.scopes.length,
          rate: identity.data.rateLimitPerMinute,
        })}
      </small>
      <button className="text-button" onClick={onConnect}>
        {t('action.changeKey')} <span>→</span>
      </button>
    </div>
  );
}

export function ConnectDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const [credential, setCredential] = useCredential();
  const [draft, setDraft] = useState(credential);
  const identity = useIdentity();

  const save = () => {
    setCredential(draft);
    onClose();
  };
  const disconnect = () => {
    setCredential('');
    onClose();
  };

  return (
    <Modal title={t('dialog.connect.title')} onClose={onClose}>
      <p>{t('dialog.connect.intro')}</p>
      <p className="caution">{t('dialog.connect.caution')}</p>

      <label className="field">
        <span>{t('dialog.connect.keyLabel')}</span>
        <input
          type="password"
          className="credential-input"
          value={draft}
          spellCheck={false}
          autoComplete="off"
          placeholder="ceap_…"
          onChange={(event) => setDraft(event.target.value)}
        />
      </label>

      {credential !== '' && identity.isSuccess && (
        <div className="scope-list">
          {identity.data.scopes.map((scope) => (
            <span className="scope-chip" key={scope}>
              {isKnownScope(scope) ? t(`scope.${scope}` as TranslationKey) : scope}
            </span>
          ))}
        </div>
      )}
      {credential !== '' && identity.isError && <p className="error-note">{identity.error.message}</p>}

      <div className="modal-actions">
        {credential !== '' && (
          <button className="secondary" onClick={disconnect}>
            {t('action.disconnect')}
          </button>
        )}
        <button className="secondary" onClick={onClose}>
          {t('action.cancel')}
        </button>
        <button className="primary" onClick={save} disabled={draft.trim() === ''}>
          {t('action.save')}
        </button>
      </div>
    </Modal>
  );
}
