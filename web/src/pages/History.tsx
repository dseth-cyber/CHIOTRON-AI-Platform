import { useState } from 'react';
import { deleteConversation, type ConversationSummary } from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState } from '../components/EmptyState';
import { FavoriteButton } from '../components/FavoriteButton';
import { useConversations, useFavorites, useRefreshHistory, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { SCOPE_CHAT } from '../Connection';
import type { Navigate } from '../navigation';

/**
 * Every conversation this credential owns.
 *
 * ARCHITECTURE-v1 section 10 gives history its own page rather than a panel
 * beside the transcript: once there are more than a handful, finding one needs
 * sorting and paging, which a sidebar cannot give.
 */
export function History({ onNavigate }: { onNavigate: Navigate }) {
  const { t, formatNumber, formatDate } = useTranslation();
  const { has } = useScopes();
  const history = useConversations(has(SCOPE_CHAT));
  const favorites = useFavorites(has(SCOPE_CHAT));
  const refresh = useRefreshHistory();
  const [error, setError] = useState('');

  if (!has(SCOPE_CHAT)) {
    return <EmptyState title={t('history.scope.title')} body={t('history.scope.body')} />;
  }

  const marked = new Set(
    (favorites.data ?? []).filter((mark) => mark.kind === 'conversation').map((mark) => mark.targetId),
  );

  const remove = async (summary: ConversationSummary) => {
    setError('');
    try {
      await deleteConversation(summary.id);
      refresh();
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('chat.error.deleteFailed'));
    }
  };

  const columns: Column<ConversationSummary>[] = [
    {
      key: 'favorite',
      header: '★',
      cell: (row) => (
        <FavoriteButton
          kind="conversation"
          targetId={row.id}
          marked={marked.has(row.id)}
          label={row.title || t('chat.untitled')}
        />
      ),
    },
    {
      key: 'title',
      header: t('history.column.title'),
      sortValue: (row) => row.title.toLowerCase(),
      cell: (row) => (
        <button
          className="link-button"
          onClick={() => onNavigate('chat', { conversationId: row.id })}
        >
          {row.title || t('chat.untitled')}
        </button>
      ),
    },
    {
      key: 'assistant',
      header: t('chat.assistant'),
      sortValue: (row) => row.assistantName ?? '',
      cell: (row) => row.assistantName ?? t('chat.unknownAssistant'),
    },
    {
      key: 'messages',
      header: t('history.column.messages'),
      align: 'end',
      sortValue: (row) => row.messageCount,
      cell: (row) => formatNumber(row.messageCount),
    },
    {
      key: 'tokens',
      header: t('history.column.tokens'),
      align: 'end',
      sortValue: (row) => row.totalTokens,
      cell: (row) => formatNumber(row.totalTokens),
    },
    {
      key: 'updated',
      header: t('history.column.updated'),
      sortValue: (row) => row.updatedAt,
      cell: (row) => formatDate(row.updatedAt),
    },
    {
      key: 'created',
      header: t('history.column.created'),
      hidden: true,
      sortValue: (row) => row.createdAt,
      cell: (row) => formatDate(row.createdAt),
    },
    {
      key: 'delete',
      header: t('history.column.actions'),
      align: 'end',
      cell: (row) => (
        <button className="danger-button" onClick={() => void remove(row)}>
          {t('action.delete')}
        </button>
      ),
    },
  ];

  return (
    <>
      <section className="page-intro">
        <p>{t('history.intro')}</p>
        {history.data?.promptsPersisted === false && <span>{t('chat.promptsOff')}</span>}
      </section>
      <DataTable
        columns={columns}
        rows={history.data?.conversations ?? []}
        rowKey={(row) => row.id}
        loading={history.isPending}
        error={error || (history.isError ? history.error.message : undefined)}
        empty={t('chat.historyEmpty')}
        pageSize={12}
        actions={
          <button className="primary" onClick={() => onNavigate('chat')}>
            {t('action.newChat')}
          </button>
        }
      />
    </>
  );
}
