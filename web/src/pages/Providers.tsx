import { useState } from 'react';
import {
  checkProvider,
  createProvider,
  deleteProvider,
  deleteRoute,
  saveRoute,
  updateProvider,
  type ComputeProvider,
  type ModelRoute,
  type ProviderCheck,
} from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState, Tag } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { Modal } from '../Modal';
import { useProviderRegistry, useRefreshProviders, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { classificationTone, statusTone, toneFor } from '../theme';
import { SCOPE_ADMIN_KEYS } from '../Connection';

/**
 * Model providers and the routing table.
 *
 * ARCHITECTURE-v1 sections 46 and 53: which backend answers a logical model is
 * configuration an operator changes here, not an environment variable that
 * needs a redeploy. Adding a cloud provider during development and switching
 * back to the local GPU afterwards is two edits on this page.
 */

const CLASSIFICATION_I18N: Record<string, Record<string, string>> = {
  th: {
    public: 'สาธารณะ (Public)',
    internal: 'ภายใน (Internal)',
    restricted: 'จำกัดสิทธิ์ (Restricted)',
    confidential: 'ลับเฉพาะ (Confidential)',
  },
  en: {
    public: 'Public',
    internal: 'Internal',
    restricted: 'Restricted',
    confidential: 'Confidential',
  },
  zh: {
    public: '公开 (Public)',
    internal: '内部 (Internal)',
    restricted: '受限 (Restricted)',
    confidential: '机密 (Confidential)',
  },
  ja: {
    public: '公開 (Public)',
    internal: '社内 (Internal)',
    restricted: '制限 (Restricted)',
    confidential: '機密 (Confidential)',
  },
  my: {
    public: 'အများသုံး (Public)',
    internal: 'အတွင်းပိုင်း (Internal)',
    restricted: 'ကန့်သတ် (Restricted)',
    confidential: 'လျှို့ဝှက် (Confidential)',
  },
};

const STATUS_I18N: Record<string, Record<string, string>> = {
  th: {
    available: 'พร้อมใช้งาน (Available)',
    unreachable: 'ไม่สามารถติดต่อได้',
    degraded: 'ประสิทธิภาพลดลง',
    disabled: 'ปิดใช้งาน',
    checking: 'กำลังตรวจสอบ...',
  },
  en: {
    available: 'Available',
    unreachable: 'Unreachable',
    degraded: 'Degraded',
    disabled: 'Disabled',
    checking: 'Checking...',
  },
  zh: {
    available: '可用 (Available)',
    unreachable: '不可达',
    degraded: '性能受损',
    disabled: '已禁用',
    checking: '正在检查...',
  },
  ja: {
    available: '利用可能 (Available)',
    unreachable: '接続不可',
    degraded: 'パフォーマンス低下',
    disabled: '無効',
    checking: '確認中...',
  },
  my: {
    available: 'အသုံးပြုနိုင်သည် (Available)',
    unreachable: 'မချိတ်ဆက်နိုင်ပါ',
    degraded: 'စွမ်းဆောင်ရည်ကျဆင်း',
    disabled: 'ပိတ်ထားသည်',
    checking: 'စစ်ဆေးနေသည်...',
  },
};

function getModelSize(modelName: string): string {
  const m = modelName.toLowerCase();
  if (m.includes('qwen3:8b') || m.includes('qwen3-8b')) return '5.2 GB';
  if (m.includes('qwen3:4b') || m.includes('qwen3-4b')) return '2.5 GB';
  if (m.includes('qwen3:0.6b') || m.includes('qwen3-0.6b') || m.includes('qwen3:0.5b')) return '522 MB';
  if (m.includes('qwen2.5:0.5b') || m.includes('qwen2.5-0.5b')) return '397 MB';
  if (m.includes('qwen2.5:7b') || m.includes('qwen2.5-7b')) return '4.7 GB';
  if (m.includes('qwen2.5:14b') || m.includes('qwen2.5-14b')) return '9.0 GB';
  if (m.includes('nomic-embed')) return '274 MB';
  if (m.includes('llama3:8b') || m.includes('llama3.1:8b')) return '4.7 GB';
  if (m.includes('mistral')) return '4.1 GB';
  if (m.includes('deepseek-r1:8b')) return '4.9 GB';
  if (m.includes('deepseek-r1:1.5b')) return '1.1 GB';
  return 'Local GPU';
}

/**
 * Model providers and the routing table.
 *
 * ARCHITECTURE-v1 sections 46 and 53: which backend answers a logical model is
 * configuration an operator changes here, not an environment variable that
 * needs a redeploy. Adding a cloud provider during development and switching
 * back to the local GPU afterwards is two edits on this page.
 */
export function Providers() {
  const { t, formatDate, language } = useTranslation();
  const { has } = useScopes();
  const canAdmin = has(SCOPE_ADMIN_KEYS);
  const registry = useProviderRegistry(canAdmin);
  const refresh = useRefreshProviders();

  const [editing, setEditing] = useState<ComputeProvider | null>(null);
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<ComputeProvider | null>(null);
  const [routing, setRouting] = useState<ModelRoute | null>(null);
  const [addingRoute, setAddingRoute] = useState(false);
  const [removingRoute, setRemovingRoute] = useState<ModelRoute | null>(null);
  const [checked, setChecked] = useState<Record<string, ProviderCheck>>({});
  const [error, setError] = useState('');

  if (!canAdmin) {
    return <EmptyState title={t('providers.scope.title')} body={t('providers.scope.body')} />;
  }

  const providers = registry.data?.providers ?? [];
  const routes = registry.data?.routes ?? [];
  const classifications = registry.data?.classifications ?? [];

  const runCheck = async (record: ComputeProvider) => {
    setError('');
    try {
      const result = await checkProvider(record.slug);
      if (result) setChecked((current) => ({ ...current, [record.slug]: result }));
      refresh();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('providers.error.check'));
    }
  };

  const toggle = async (record: ComputeProvider) => {
    setError('');
    try {
      await updateProvider(record.slug, { enabled: !record.enabled });
      refresh();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('providers.error.save'));
    }
  };

  const toggleRoute = async (record: ModelRoute) => {
    setError('');
    try {
      await saveRoute({
        logical: record.logical,
        provider: record.provider,
        model: record.model,
        default: record.default,
        enabled: !record.enabled,
      });
      refresh();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('providers.error.save'));
    }
  };

  const formatClassification = (val: string) => {
    const locMap = CLASSIFICATION_I18N[language] ?? CLASSIFICATION_I18N.th;
    return locMap[val.toLowerCase()] || val;
  };

  const formatStatus = (val: string) => {
    const locMap = STATUS_I18N[language] ?? STATUS_I18N.th;
    return locMap[val.toLowerCase()] || val;
  };

  const providerColumns: Column<ComputeProvider>[] = [
    {
      key: 'name',
      header: t('providers.column.provider'),
      sortValue: (row) => row.slug,
      cell: (row) => (
        <span className="cell-stack">
          <b>{row.name || row.slug}</b>
          <small>
            <code>{row.slug}</code> · {row.kind}
          </small>
        </span>
      ),
    },
    {
      key: 'baseUrl',
      header: t('providers.column.endpoint'),
      cell: (row) => <code>{row.baseUrl}</code>,
    },
    {
      key: 'credential',
      header: t('providers.column.credential'),
      cell: (row) =>
        row.hasCredential ? (
          <Tag tone="ok">···{row.credentialHint}</Tag>
        ) : (
          <Tag>{t('providers.noCredential')}</Tag>
        ),
    },
    {
      key: 'ceiling',
      header: t('providers.column.ceiling'),
      sortValue: (row) => row.maxClassification,
      cell: (row) => (
        <Tag tone={toneFor(classificationTone, row.maxClassification)}>
          {formatClassification(row.maxClassification)}
        </Tag>
      ),
    },
    {
      key: 'status',
      header: t('providers.column.status'),
      sortValue: (row) => row.lastStatus ?? '',
      cell: (row) => {
        const live = checked[row.slug];
        const rawStatus = live?.status ?? row.lastStatus;
        return (
          <span className="cell-stack">
            {!row.enabled && <Tag tone="warn">{formatStatus('disabled')}</Tag>}
            {rawStatus && (
              <Tag tone={toneFor(statusTone, rawStatus)}>
                {formatStatus(rawStatus)}
              </Tag>
            )}
            {(live?.error || row.lastError) && (
              <small className="warn">{live?.error || row.lastError}</small>
            )}
            {row.lastCheckedAt && !live && <small>{formatDate(row.lastCheckedAt)}</small>}
          </span>
        );
      },
    },
    {
      key: 'actions',
      header: t('history.column.actions'),
      align: 'end',
      cell: (row) => (
        <span className="row-actions">
          <button className="secondary" onClick={() => void runCheck(row)}>
            {t('providers.check')}
          </button>
          <button className="secondary" onClick={() => setEditing(row)}>
            {t('providers.edit')}
          </button>
          <button
            type="button"
            className={`ui-toggle-switch ${row.enabled ? 'active' : ''}`}
            onClick={() => void toggle(row)}
            title={row.enabled ? t('providers.disable') : t('providers.enable')}
          >
            <span className="ui-toggle-track">
              <span className="ui-toggle-thumb" />
            </span>
            <span className="ui-toggle-label">
              {row.enabled
                ? (language === 'th' ? 'เปิด (ON)' : language === 'zh' ? '开启 (ON)' : language === 'ja' ? '有効 (ON)' : language === 'my' ? 'ဖွင့် (ON)' : 'ON')
                : (language === 'th' ? 'ปิด (OFF)' : language === 'zh' ? '关闭 (OFF)' : language === 'ja' ? '無効 (OFF)' : language === 'my' ? 'ပိတ် (OFF)' : 'OFF')}
            </span>
          </button>
          <button className="danger-button" onClick={() => setRemoving(row)}>
            {t('action.delete')}
          </button>
        </span>
      ),
    },
  ];

  const routeColumns: Column<ModelRoute>[] = [
    {
      key: 'logical',
      header: t('providers.column.logical'),
      sortValue: (row) => row.logical,
      cell: (row) => (
        <span className="cell-stack">
          <b>
            <code>{row.logical}</code>
          </b>
          {row.default && <small>{t('providers.isDefault')}</small>}
        </span>
      ),
    },
    {
      key: 'target',
      header: t('providers.column.target'),
      sortValue: (row) => row.provider,
      cell: (row) => (
        <code>
          {row.provider}/{row.model}
        </code>
      ),
    },
    {
      key: 'size',
      header:
        language === 'th'
          ? 'ขนาดโมเดล'
          : language === 'zh'
          ? '模型大小'
          : language === 'ja'
          ? 'モデルサイズ'
          : language === 'my'
          ? 'Model အရွယ်အစား'
          : 'Model Size',
      sortValue: (row) => row.model,
      cell: (row) => (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
          <span style={{ fontSize: '0.85rem' }}>💾</span>
          <code style={{ color: '#00d2ff', fontWeight: 700, fontSize: '0.86rem' }}>
            {getModelSize(row.model)}
          </code>
        </span>
      ),
    },
    {
      key: 'ceiling',
      header: t('providers.column.ceiling'),
      cell: (row) => {
        const owner = providers.find((entry) => entry.slug === row.provider);
        return owner ? (
          <Tag tone={toneFor(classificationTone, owner.maxClassification)}>
            {formatClassification(owner.maxClassification)}
          </Tag>
        ) : (
          <Tag tone="danger">{t('providers.missingProvider')}</Tag>
        );
      },
    },
    {
      key: 'status',
      header: t('providers.column.status'),
      cell: (row) => (
        <button
          type="button"
          className={`ui-toggle-switch ${row.enabled ? 'active' : ''}`}
          onClick={() => void toggleRoute(row)}
          title={row.enabled ? t('providers.disable') : t('providers.enable')}
        >
          <span className="ui-toggle-track">
            <span className="ui-toggle-thumb" />
          </span>
          <span className="ui-toggle-label">
            {row.enabled
              ? (language === 'th' ? 'เปิด (ON)' : language === 'zh' ? '开启 (ON)' : language === 'ja' ? '有効 (ON)' : language === 'my' ? 'ဖွင့် (ON)' : 'ON')
              : (language === 'th' ? 'ปิด (OFF)' : language === 'zh' ? '关闭 (OFF)' : language === 'ja' ? '無効 (OFF)' : language === 'my' ? 'ပိတ် (OFF)' : 'OFF')}
          </span>
        </button>
      ),
    },
    {
      key: 'actions',
      header: t('history.column.actions'),
      align: 'end',
      cell: (row) => (
        <span className="row-actions">
          <button className="secondary" onClick={() => setRouting(row)}>
            {t('providers.edit')}
          </button>
          <button
            className="danger-button"
            disabled={row.default}
            title={row.default ? t('providers.cannotDeleteDefault') : undefined}
            onClick={() => setRemovingRoute(row)}
          >
            {t('action.delete')}
          </button>
        </span>
      ),
    },
  ];

  return (
    <>
      <section className="page-intro">
        <p>{t('providers.intro')}</p>
        <span>{t('providers.note')}</span>
      </section>

      {registry.data?.credentialStorage === false && (
        <section className="notice">
          <div>
            <b>{t('providers.noEncryption.title')}</b>
            <p>{t('providers.noEncryption.body')}</p>
          </div>
        </section>
      )}

      <p className="caution">{t('providers.egressWarning')}</p>

      {error !== '' && <p className="error-note">{error}</p>}

      <DataTable
        columns={providerColumns}
        rows={providers}
        rowKey={(row) => row.slug}
        loading={registry.isPending}
        error={registry.isError ? registry.error.message : undefined}
        empty={t('providers.empty')}
        actions={
          <button className="primary" onClick={() => setAdding(true)}>
            {t('providers.add')}
          </button>
        }
      />

      <h2 className="section-heading">{t('providers.routes')}</h2>
      <p className="source-note">{t('providers.routes.note')}</p>
      <DataTable
        columns={routeColumns}
        rows={routes}
        rowKey={(row) => row.logical}
        loading={registry.isPending}
        empty={t('providers.routes.empty')}
        actions={
          <button className="primary" disabled={providers.length === 0} onClick={() => setAddingRoute(true)}>
            {t('providers.addRoute')}
          </button>
        }
      />

      {(adding || editing) && (
        <ProviderDialog
          existing={editing}
          kinds={registry.data?.kinds ?? []}
          classifications={classifications}
          credentialStorage={registry.data?.credentialStorage ?? false}
          onClose={() => {
            setAdding(false);
            setEditing(null);
          }}
          onSaved={() => {
            setAdding(false);
            setEditing(null);
            refresh();
          }}
        />
      )}

      {(addingRoute || routing) && (
        <RouteDialog
          existing={routing}
          providers={providers}
          onClose={() => {
            setAddingRoute(false);
            setRouting(null);
          }}
          onSaved={() => {
            setAddingRoute(false);
            setRouting(null);
            refresh();
          }}
        />
      )}

      {removing && (
        <ConfirmDialog
          title={t('providers.confirmDelete.title')}
          body={t('providers.confirmDelete.body', { name: removing.name || removing.slug })}
          confirmLabel={t('action.delete')}
          destructive
          onCancel={() => setRemoving(null)}
          onConfirm={async () => {
            await deleteProvider(removing.slug);
            setRemoving(null);
            refresh();
          }}
        />
      )}

      {removingRoute && (
        <ConfirmDialog
          title={t('providers.confirmDeleteRoute.title')}
          body={t('providers.confirmDeleteRoute.body', { logical: removingRoute.logical })}
          confirmLabel={t('action.delete')}
          destructive
          onCancel={() => setRemovingRoute(null)}
          onConfirm={async () => {
            await deleteRoute(removingRoute.logical);
            setRemovingRoute(null);
            refresh();
          }}
        />
      )}
    </>
  );
}

function ProviderDialog({
  existing,
  kinds,
  classifications,
  credentialStorage,
  onClose,
  onSaved,
}: {
  existing: ComputeProvider | null;
  kinds: string[];
  classifications: string[];
  credentialStorage: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [slug, setSlug] = useState(existing?.slug ?? '');
  const [name, setName] = useState(existing?.name ?? '');
  const [kind, setKind] = useState(existing?.kind ?? kinds[0] ?? '');
  const [baseUrl, setBaseUrl] = useState(existing?.baseUrl ?? '');
  const [credential, setCredential] = useState('');
  const [ceiling, setCeiling] = useState(existing?.maxClassification ?? classifications[0] ?? 'public');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  // A local provider needs no key; a hosted one is useless without it.
  const needsCredential = kind === 'openai-compatible' || kind === 'anthropic';

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      if (existing) {
        await updateProvider(existing.slug, {
          name,
          baseUrl,
          maxClassification: ceiling,
          // Omitted when left blank: the portal never receives the stored
          // credential, so sending an empty string would erase it.
          ...(credential.trim() === '' ? {} : { credential: credential.trim() }),
        });
      } else {
        await createProvider({
          slug: slug.trim(),
          name: name.trim() || slug.trim(),
          kind,
          baseUrl: baseUrl.trim(),
          maxClassification: ceiling,
          ...(credential.trim() === '' ? {} : { credential: credential.trim() }),
        });
      }
      onSaved();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('providers.error.save'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={existing ? t('providers.edit.title') : t('providers.add.title')} onClose={onClose}>
      <p>{t('providers.dialog.body')}</p>

      {!existing && (
        <>
          <label className="field">
            <span>{t('providers.field.slug')}</span>
            <input
              value={slug}
              placeholder="openai"
              spellCheck={false}
              onChange={(event) => setSlug(event.target.value)}
            />
          </label>
          <SearchableSelect
            label={t('providers.field.kind')}
            value={kind}
            options={kinds.map((entry) => ({
              value: entry,
              label: entry,
              detail: t(`providers.kind.${entry}` as never),
            }))}
            onChange={setKind}
          />
        </>
      )}

      <label className="field">
        <span>{t('providers.field.name')}</span>
        <input value={name} onChange={(event) => setName(event.target.value)} />
      </label>

      <label className="field">
        <span>{t('providers.field.baseUrl')}</span>
        <input
          value={baseUrl}
          spellCheck={false}
          placeholder="https://api.openai.com/v1"
          onChange={(event) => setBaseUrl(event.target.value)}
        />
      </label>

      {needsCredential && (
        <label className="field">
          <span>
            {existing?.hasCredential ? t('providers.field.credentialReplace') : t('providers.field.credential')}
          </span>
          <input
            type="password"
            className="credential-input"
            value={credential}
            spellCheck={false}
            autoComplete="off"
            disabled={!credentialStorage}
            placeholder={credentialStorage ? 'sk-…' : t('providers.noEncryption.title')}
            onChange={(event) => setCredential(event.target.value)}
          />
        </label>
      )}

      <SearchableSelect
        label={t('providers.field.ceiling')}
        value={ceiling}
        options={classifications.map((entry) => ({ value: entry, label: entry }))}
        onChange={setCeiling}
      />
      <p className="caution">{t('providers.field.ceiling.help')}</p>

      {error !== '' && <p className="error-note">{error}</p>}

      <div className="modal-actions">
        <button className="secondary" onClick={onClose}>
          {t('action.cancel')}
        </button>
        <button
          className="primary"
          disabled={busy || baseUrl.trim() === '' || (!existing && slug.trim() === '')}
          onClick={() => void submit()}
        >
          {busy ? t('confirm.working') : t('providers.save')}
        </button>
      </div>
    </Modal>
  );
}

function RouteDialog({
  existing,
  providers,
  onClose,
  onSaved,
}: {
  existing: ModelRoute | null;
  providers: ComputeProvider[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [logical, setLogical] = useState(existing?.logical ?? '');
  const [target, setTarget] = useState(existing?.provider ?? providers[0]?.slug ?? '');
  const [model, setModel] = useState(existing?.model ?? '');
  const [isDefault, setIsDefault] = useState(existing?.default ?? false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const owner = providers.find((entry) => entry.slug === target);

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      await saveRoute({
        logical: logical.trim(),
        provider: target,
        model: model.trim(),
        default: isDefault,
      });
      onSaved();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('providers.error.save'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={existing ? t('providers.route.edit') : t('providers.route.add')} onClose={onClose}>
      <p>{t('providers.route.body')}</p>

      <label className="field">
        <span>{t('providers.column.logical')}</span>
        <input
          value={logical}
          spellCheck={false}
          placeholder="default"
          disabled={existing !== null}
          onChange={(event) => setLogical(event.target.value)}
        />
      </label>

      <SearchableSelect
        label={t('providers.column.provider')}
        value={target}
        options={providers.map((entry) => ({
          value: entry.slug,
          label: entry.name || entry.slug,
          detail: `${entry.kind} · ${entry.maxClassification}`,
        }))}
        onChange={setTarget}
      />

      <label className="field">
        <span>{t('providers.field.upstreamModel')}</span>
        <input
          value={model}
          spellCheck={false}
          placeholder="gpt-4o-mini"
          onChange={(event) => setModel(event.target.value)}
        />
      </label>

      <label className="field inline">
        <input
          type="checkbox"
          checked={isDefault}
          onChange={(event) => setIsDefault(event.target.checked)}
        />
        <span>{t('providers.field.makeDefault')}</span>
      </label>

      {owner && (
        <p className="caution">
          {t('providers.route.ceilingNote', {
            provider: owner.name || owner.slug,
            ceiling: owner.maxClassification,
          })}
        </p>
      )}

      {error !== '' && <p className="error-note">{error}</p>}

      <div className="modal-actions">
        <button className="secondary" onClick={onClose}>
          {t('action.cancel')}
        </button>
        <button
          className="primary"
          disabled={busy || logical.trim() === '' || model.trim() === ''}
          onClick={() => void submit()}
        >
          {busy ? t('confirm.working') : t('providers.save')}
        </button>
      </div>
    </Modal>
  );
}
