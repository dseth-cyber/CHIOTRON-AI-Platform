import { useState } from 'react';
import { deleteDocument, uploadDocument, type Document } from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState, Tag } from '../components/EmptyState';
import { FavoriteButton } from '../components/FavoriteButton';
import { SearchableSelect } from '../components/SearchableSelect';
import { Modal } from '../Modal';
import { useDocuments, useFavorites, useRefreshDocuments, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { classificationTone, statusTone, toneFor } from '../theme';
import { SCOPE_CHAT, SCOPE_KNOWLEDGE_READ, SCOPE_KNOWLEDGE_WRITE } from '../Connection';

/** What the parser understands today; anything else is refused on upload. */
const MIME_TYPES = ['text/plain', 'text/markdown'];

export function Documents() {
  const { t, formatNumber, formatDate } = useTranslation();
  const { has } = useScopes();
  const canRead = has(SCOPE_KNOWLEDGE_READ);
  const canWrite = has(SCOPE_KNOWLEDGE_WRITE);

  const documents = useDocuments(canRead);
  const favorites = useFavorites(has(SCOPE_CHAT));
  const refresh = useRefreshDocuments();
  const [showUpload, setShowUpload] = useState(false);
  const [error, setError] = useState('');

  if (!canRead) {
    return <EmptyState title={t('documents.scope.title')} body={t('documents.scope.body')} />;
  }

  const marked = new Set(
    (favorites.data ?? []).filter((mark) => mark.kind === 'document').map((mark) => mark.targetId),
  );

  const remove = async (document: Document) => {
    setError('');
    try {
      await deleteDocument(document.id);
      refresh();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('documents.error.delete'));
    }
  };

  const columns: Column<Document>[] = [
    {
      key: 'favorite',
      header: '★',
      cell: (row) => (
        <FavoriteButton kind="document" targetId={row.id} marked={marked.has(row.id)} label={row.title} />
      ),
    },
    {
      key: 'title',
      header: t('documents.column.title'),
      sortValue: (row) => row.title.toLowerCase(),
      cell: (row) => (
        <span className="cell-stack">
          <b>{row.title}</b>
          <small>{row.sourceSlug}</small>
        </span>
      ),
    },
    {
      key: 'classification',
      header: t('documents.column.classification'),
      sortValue: (row) => row.classification,
      cell: (row) => (
        <Tag tone={toneFor(classificationTone, row.classification)}>{row.classification}</Tag>
      ),
    },
    {
      key: 'status',
      header: t('documents.column.status'),
      sortValue: (row) => row.status,
      cell: (row) => (
        <span className="cell-stack">
          <Tag tone={toneFor(statusTone, row.status)}>{row.status}</Tag>
          {/* The worker's own message, not a translated one: it says what went
              wrong with this document, and paraphrasing it would lose that. */}
          {row.error && <small className="warn">{row.error}</small>}
        </span>
      ),
    },
    {
      key: 'chunks',
      header: t('documents.column.chunks'),
      align: 'end',
      sortValue: (row) => row.chunkCount,
      cell: (row) => formatNumber(row.chunkCount),
    },
    {
      key: 'size',
      header: t('documents.column.size'),
      align: 'end',
      sortValue: (row) => row.byteSize,
      cell: (row) => t('documents.kb', { size: formatNumber(Math.max(1, Math.round(row.byteSize / 1024))) }),
    },
    {
      key: 'created',
      header: t('documents.column.uploaded'),
      sortValue: (row) => row.createdAt,
      cell: (row) => formatDate(row.createdAt),
    },
    {
      key: 'ingested',
      header: t('documents.column.ingested'),
      hidden: true,
      sortValue: (row) => row.ingestedAt ?? '',
      cell: (row) => (row.ingestedAt ? formatDate(row.ingestedAt) : '—'),
    },
    {
      key: 'checksum',
      header: t('documents.column.checksum'),
      hidden: true,
      cell: (row) => <code>{row.checksum.slice(0, 12)}</code>,
    },
    {
      key: 'delete',
      header: t('history.column.actions'),
      align: 'end',
      cell: (row) => (
        <button className="danger-button" disabled={!canWrite} onClick={() => void remove(row)}>
          {t('action.delete')}
        </button>
      ),
    },
  ];

  const readable = documents.data?.readableClassifications ?? [];

  return (
    <>
      <section className="page-intro">
        <p>{t('documents.intro')}</p>
        <span>
          {readable.length > 0
            ? t('documents.readable', { list: readable.join(', ') })
            : t('documents.readableUnknown')}
        </span>
      </section>

      <DataTable
        columns={columns}
        rows={documents.data?.documents ?? []}
        rowKey={(row) => row.id}
        loading={documents.isPending}
        error={error || (documents.isError ? documents.error.message : undefined)}
        empty={t('documents.empty')}
        pageSize={12}
        actions={
          <button className="primary" disabled={!canWrite} onClick={() => setShowUpload(true)}>
            {canWrite ? t('action.upload') : t('documents.uploadNeedsScope')}
          </button>
        }
      />

      {showUpload && (
        <UploadDialog
          classifications={readable}
          onClose={() => setShowUpload(false)}
          onUploaded={() => {
            setShowUpload(false);
            refresh();
          }}
        />
      )}
    </>
  );
}

function UploadDialog({
  classifications,
  onClose,
  onUploaded,
}: {
  classifications: string[];
  onClose: () => void;
  onUploaded: () => void;
}) {
  const { t } = useTranslation();
  const [title, setTitle] = useState('');
  const [source, setSource] = useState('uploads');
  const [mimeType, setMimeType] = useState(MIME_TYPES[0]!);
  // Defaulting to the least sensitive readable level: a wrong guess upward hides
  // a document from colleagues, a wrong guess downward exposes it.
  const [classification, setClassification] = useState(classifications[0] ?? 'internal');
  const [content, setContent] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const pick = async (file: File | undefined) => {
    if (!file) return;
    setContent(await file.text());
    if (title === '') setTitle(file.name.replace(/\.[^.]+$/, ''));
    if (file.name.endsWith('.md')) setMimeType('text/markdown');
  };

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      await uploadDocument({ title: title.trim(), content, classification, sourceSlug: source, mimeType });
      onUploaded();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('documents.error.upload'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={t('documents.upload.title')} onClose={onClose}>
      <p>{t('documents.upload.body')}</p>
      <p className="caution">{t('documents.upload.caution')}</p>

      <label className="field">
        <span>{t('documents.upload.titleLabel')}</span>
        <input value={title} onChange={(event) => setTitle(event.target.value)} />
      </label>

      <div className="field-row">
        <SearchableSelect
          label={t('documents.column.classification')}
          value={classification}
          options={classifications.map((entry) => ({ value: entry, label: entry }))}
          onChange={setClassification}
        />
        <SearchableSelect
          label={t('documents.upload.format')}
          value={mimeType}
          options={MIME_TYPES.map((entry) => ({ value: entry, label: entry }))}
          onChange={setMimeType}
        />
      </div>

      <label className="field">
        <span>{t('documents.upload.source')}</span>
        <input value={source} onChange={(event) => setSource(event.target.value)} />
      </label>

      <label className="field">
        <span>{t('documents.upload.file')}</span>
        <input
          type="file"
          accept=".txt,.md,text/plain,text/markdown"
          onChange={(event) => void pick(event.target.files?.[0])}
        />
      </label>

      <label className="field">
        <span>{t('documents.upload.content')}</span>
        <textarea
          rows={8}
          value={content}
          placeholder={t('documents.upload.contentHint')}
          onChange={(event) => setContent(event.target.value)}
        />
      </label>

      {error !== '' && <p className="error-note">{error}</p>}

      <div className="modal-actions">
        <button className="secondary" onClick={onClose}>
          {t('action.cancel')}
        </button>
        <button
          className="primary"
          disabled={busy || title.trim() === '' || content.trim() === ''}
          onClick={() => void submit()}
        >
          {busy ? t('documents.upload.busy') : t('action.upload')}
        </button>
      </div>
    </Modal>
  );
}
