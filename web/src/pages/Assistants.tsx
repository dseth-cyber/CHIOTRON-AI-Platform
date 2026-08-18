import type { Assistant } from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState, Tag } from '../components/EmptyState';
import { FavoriteButton } from '../components/FavoriteButton';
import { useAssistants, useFavorites, useModels, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { SCOPE_ASSISTANTS_READ, SCOPE_CHAT, SCOPE_MODELS_READ } from '../Connection';
import type { Navigate } from '../navigation';

/**
 * The assistant catalogue.
 *
 * Instructions are policy and the catalogue never returns them, so this page
 * shows what a user needs to choose between assistants — purpose, model and
 * whether the model behind it is actually loaded — and nothing more.
 */
export function Assistants({ onNavigate }: { onNavigate: Navigate }) {
  const { t, formatNumber } = useTranslation();
  const { has } = useScopes();
  const canRead = has(SCOPE_ASSISTANTS_READ);
  const assistants = useAssistants(canRead);
  const models = useModels(has(SCOPE_MODELS_READ));
  const favorites = useFavorites(has(SCOPE_CHAT));

  if (!canRead) {
    return <EmptyState title={t('chat.error.scope.title')} body={t('chat.error.scope.body')} />;
  }

  const marked = new Set(
    (favorites.data ?? []).filter((mark) => mark.kind === 'assistant').map((mark) => mark.targetId),
  );
  // A logical route the compute plane has not loaded still routes; it just fails
  // at call time. Saying so here is cheaper than finding out mid-conversation.
  const available = new Map(
    (models.data?.models ?? []).map((entry) => [entry.logical, entry.available]),
  );

  const columns: Column<Assistant>[] = [
    {
      key: 'favorite',
      header: '★',
      cell: (row) => (
        <FavoriteButton kind="assistant" targetId={row.id} marked={marked.has(row.id)} label={row.name} />
      ),
    },
    {
      key: 'name',
      header: t('assistants.column.name'),
      sortValue: (row) => row.name.toLowerCase(),
      cell: (row) => (
        <span className="cell-stack">
          <b>{row.name}</b>
          <small>{row.description}</small>
        </span>
      ),
    },
    {
      key: 'slug',
      header: t('assistants.column.slug'),
      hidden: true,
      sortValue: (row) => row.slug,
      cell: (row) => <code>{row.slug}</code>,
    },
    {
      key: 'model',
      header: t('assistants.column.model'),
      sortValue: (row) => row.logicalModel,
      cell: (row) => (
        <span className="cell-stack">
          <code>{row.logicalModel}</code>
          {available.get(row.logicalModel) === false && (
            <small className="warn">{t('assistants.modelUnavailable')}</small>
          )}
        </span>
      ),
    },
    {
      key: 'temperature',
      header: t('assistants.column.temperature'),
      align: 'end',
      hidden: true,
      sortValue: (row) => row.temperature ?? 0,
      cell: (row) => (row.temperature === undefined ? '—' : row.temperature.toFixed(2)),
    },
    {
      key: 'maxTokens',
      header: t('assistants.column.maxTokens'),
      align: 'end',
      hidden: true,
      sortValue: (row) => row.maxTokens ?? 0,
      cell: (row) => (row.maxTokens ? formatNumber(row.maxTokens) : '—'),
    },
    {
      key: 'scope',
      header: t('assistants.column.scope'),
      sortValue: (row) => row.companyId ?? '',
      cell: (row) =>
        row.companyId ? (
          <Tag tone="info">{t('assistants.companyOnly')}</Tag>
        ) : (
          <Tag tone="ok">{t('assistants.platformWide')}</Tag>
        ),
    },
    {
      key: 'start',
      header: t('history.column.actions'),
      align: 'end',
      cell: (row) => (
        <button
          className="secondary"
          disabled={!has(SCOPE_CHAT) || !row.enabled}
          onClick={() => onNavigate('chat', { assistant: row.slug })}
        >
          {t('action.startChat')}
        </button>
      ),
    },
  ];

  return (
    <>
      <section className="page-intro">
        <p>{t('assistants.intro')}</p>
        <span>{t('assistants.note')}</span>
      </section>
      <DataTable
        columns={columns}
        rows={assistants.data ?? []}
        rowKey={(row) => row.id}
        loading={assistants.isPending}
        error={assistants.isError ? assistants.error.message : undefined}
        empty={t('assistants.empty')}
      />
    </>
  );
}
