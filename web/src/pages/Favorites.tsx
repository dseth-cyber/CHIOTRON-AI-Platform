import type { Favorite } from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState, Tag } from '../components/EmptyState';
import { FavoriteButton } from '../components/FavoriteButton';
import { useAssistants, useFavorites, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { SCOPE_ASSISTANTS_READ, SCOPE_CHAT } from '../Connection';
import type { Navigate } from '../navigation';

/**
 * Everything this credential has marked.
 *
 * The server resolves each mark against its record and drops the ones it can no
 * longer read, so a favourite never becomes a way to see the title of something
 * whose classification has since been raised.
 */
export function Favorites({ onNavigate }: { onNavigate: Navigate }) {
  const { t, formatDate } = useTranslation();
  const { has } = useScopes();
  const favorites = useFavorites(has(SCOPE_CHAT));
  const assistants = useAssistants(has(SCOPE_ASSISTANTS_READ));

  if (!has(SCOPE_CHAT)) {
    return <EmptyState title={t('favorites.scope.title')} body={t('favorites.scope.body')} />;
  }

  const slugFor = new Map((assistants.data ?? []).map((entry) => [entry.id, entry.slug]));

  const open = (mark: Favorite) => {
    switch (mark.kind) {
      case 'conversation':
        onNavigate('chat', { conversationId: mark.targetId });
        break;
      case 'assistant':
        onNavigate('chat', { assistant: slugFor.get(mark.targetId) });
        break;
      case 'document':
        onNavigate('documents');
        break;
    }
  };

  const columns: Column<Favorite>[] = [
    {
      key: 'favorite',
      header: '★',
      cell: (row) => (
        <FavoriteButton kind={row.kind} targetId={row.targetId} marked label={row.label} />
      ),
    },
    {
      key: 'label',
      header: t('favorites.column.item'),
      sortValue: (row) => row.label.toLowerCase(),
      cell: (row) => (
        <button className="link-button" onClick={() => open(row)}>
          {row.label}
        </button>
      ),
    },
    {
      key: 'kind',
      header: t('favorites.column.kind'),
      sortValue: (row) => row.kind,
      cell: (row) => <Tag tone="info">{t(`favorite.kind.${row.kind}`)}</Tag>,
    },
    {
      key: 'detail',
      header: t('favorites.column.detail'),
      cell: (row) => row.detail ?? '—',
    },
    {
      key: 'created',
      header: t('favorites.column.marked'),
      sortValue: (row) => row.createdAt,
      cell: (row) => formatDate(row.createdAt),
    },
  ];

  return (
    <>
      <section className="page-intro">
        <p>{t('favorites.intro')}</p>
        <span>{t('favorites.note')}</span>
      </section>
      <DataTable
        columns={columns}
        rows={favorites.data ?? []}
        rowKey={(row) => `${row.kind}:${row.targetId}`}
        loading={favorites.isPending}
        error={favorites.isError ? favorites.error.message : undefined}
        empty={t('favorites.empty.body')}
        onRowClick={open}
      />
    </>
  );
}
