import { useState } from 'react';
import { ApiError } from './api';
import { Modal } from './Modal';
import { useCredential, useIdentity } from './hooks';

export const SCOPE_MODELS_READ = 'models:read';
export const SCOPE_ASSISTANTS_READ = 'assistants:read';
export const SCOPE_CHAT = 'chat:completions';
export const SCOPE_ADMIN_KEYS = 'admin:keys';
export const SCOPE_ADMIN_ASSISTANTS = 'admin:assistants';

const SCOPE_LABELS: Record<string, string> = {
  [SCOPE_MODELS_READ]: 'Read the model catalogue',
  [SCOPE_ASSISTANTS_READ]: 'Read the assistant catalogue',
  [SCOPE_CHAT]: 'Run completions and read own history',
  [SCOPE_ADMIN_KEYS]: 'Manage API keys',
  [SCOPE_ADMIN_ASSISTANTS]: 'Manage assistants',
};

/** Sidebar status line: is this browser talking to the Control Plane, and as whom. */
export function ConnectionBadge({ onConnect }: { onConnect: () => void }) {
  const [credential] = useCredential();
  const identity = useIdentity();

  if (credential === '') {
    return (
      <div className="connection offline">
        <span className="live-dot offline" /> Not connected
        <button className="text-button" onClick={onConnect}>
          Add API key <span>→</span>
        </button>
      </div>
    );
  }
  if (identity.isPending) {
    return (
      <div className="connection">
        <span className="live-dot pending" /> Checking credential…
      </div>
    );
  }
  if (identity.isError) {
    const rejected = identity.error instanceof ApiError && identity.error.unauthenticated;
    return (
      <div className="connection offline">
        <span className="live-dot offline" /> {rejected ? 'Key rejected' : 'Gateway unreachable'}
        <button className="text-button" onClick={onConnect}>
          Change key <span>→</span>
        </button>
      </div>
    );
  }

  return (
    <div className="connection">
      <span className="live-dot" /> Connected as <b>{identity.data.name}</b>
      <small>
        {identity.data.scopes.length} scope{identity.data.scopes.length === 1 ? '' : 's'} ·{' '}
        {identity.data.rateLimitPerMinute}/min
      </small>
      <button className="text-button" onClick={onConnect}>
        Change key <span>→</span>
      </button>
    </div>
  );
}

export function ConnectDialog({ onClose }: { onClose: () => void }) {
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
    <Modal title="Connect to the AI Gateway" onClose={onClose}>
      <p>
        The portal calls the Control Plane with an API key. Paste one minted with{' '}
        <code>apikey create</code> or from the admin endpoint. It is kept in this tab only and is
        cleared when the tab closes.
      </p>
      <p className="caution">
        Give the portal the narrowest key that does the job. A browser is not a safe place for{' '}
        <code>admin:keys</code>. This is a development bridge until the Identity Service issues JWTs
        to the portal.
      </p>

      <label className="field">
        <span>API key</span>
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
              {SCOPE_LABELS[scope] ?? scope}
            </span>
          ))}
        </div>
      )}
      {credential !== '' && identity.isError && (
        <p className="error-note">{identity.error.message}</p>
      )}

      <div className="modal-actions">
        {credential !== '' && (
          <button className="secondary" onClick={disconnect}>
            Disconnect
          </button>
        )}
        <button className="secondary" onClick={onClose}>
          Cancel
        </button>
        <button className="primary" onClick={save} disabled={draft.trim() === ''}>
          Save key
        </button>
      </div>
    </Modal>
  );
}
