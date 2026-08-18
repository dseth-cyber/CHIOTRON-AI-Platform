import { useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from '../LanguageContext';

export type Column<Row> = {
  key: string;
  header: string;
  /** Rendered cell. Kept separate from the sort value so a cell may be markup. */
  cell: (row: Row) => ReactNode;
  /** Comparable value. A column without one cannot be sorted. */
  sortValue?: (row: Row) => string | number;
  /** Columns hidden by default still appear in the column picker. */
  hidden?: boolean;
  align?: 'start' | 'end';
};

type Sort = { key: string; direction: 'asc' | 'desc' };

/** ARCHITECTURE-v1 section 38 fixes the page-size choices. */
const PAGE_SIZES = [10, 20, 50, 100];

/**
 * The shared table: loading, sorting, column selection and pagination.
 *
 * ARCHITECTURE-v1 section 10 requires every table to behave the same way. Doing
 * it once means a user learns the behaviour on one page and keeps it on the
 * next, and it stops each page inventing its own idea of what an empty result or
 * a loading state looks like.
 */
export function DataTable<Row>({
  columns,
  rows,
  rowKey,
  loading = false,
  error,
  empty,
  actions,
  trash,
}: {
  columns: Column<Row>[];
  rows: Row[];
  rowKey: (row: Row) => string;
  loading?: boolean;
  error?: string;
  empty?: ReactNode;
  actions?: ReactNode;
  /** Present when the caller has a trash view to toggle into (section 43). */
  trash?: { showing: boolean; onToggle: (showing: boolean) => void; count?: number };
}) {
  const { t, formatNumber } = useTranslation();
  const [sort, setSort] = useState<Sort | null>(null);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZES[0]!);
  const [showPicker, setShowPicker] = useState(false);
  const [visible, setVisible] = useState<Set<string>>(
    () => new Set(columns.filter((column) => !column.hidden).map((column) => column.key)),
  );

  const shown = columns.filter((column) => visible.has(column.key));

  const sorted = useMemo(() => {
    if (!sort) return rows;
    const column = columns.find((entry) => entry.key === sort.key);
    if (!column?.sortValue) return rows;
    const ordered = [...rows].sort((left, right) => {
      const a = column.sortValue!(left);
      const b = column.sortValue!(right);
      if (a === b) return 0;
      return (a < b ? -1 : 1) * (sort.direction === 'asc' ? 1 : -1);
    });
    return ordered;
  }, [rows, sort, columns]);

  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
  // A page that no longer exists after a filter would render blank, so clamp.
  const current = Math.min(page, pageCount - 1);
  const from = current * pageSize;
  const visibleRows = sorted.slice(from, from + pageSize);

  const toggleSort = (column: Column<Row>) => {
    if (!column.sortValue) return;
    setSort((existing) =>
      existing?.key === column.key
        ? { key: column.key, direction: existing.direction === 'asc' ? 'desc' : 'asc' }
        : { key: column.key, direction: 'asc' },
    );
    setPage(0);
  };

  return (
    <section className="table-block">
      <header className="table-bar">
        <div className="table-actions">{actions}</div>
        <div className="table-tools">
          {trash && (
            <button
              className={trash.showing ? 'toggle-button on' : 'toggle-button'}
              aria-pressed={trash.showing}
              onClick={() => {
                trash.onToggle(!trash.showing);
                setPage(0);
              }}
            >
              🗑 {t('table.trash')}
              {trash.count ? ` (${formatNumber(trash.count)})` : ''}
            </button>
          )}
          <button className="text-button" onClick={() => setShowPicker((open) => !open)}>
            {t('table.columns')}
          </button>
        </div>
      </header>

      {showPicker && (
        <div className="column-picker">
          {columns.map((column) => (
            <label key={column.key}>
              <input
                type="checkbox"
                checked={visible.has(column.key)}
                onChange={() =>
                  setVisible((existing) => {
                    const next = new Set(existing);
                    // The last visible column cannot be hidden: a table with no
                    // columns is not a smaller table, it is a broken one.
                    if (next.has(column.key) && next.size > 1) next.delete(column.key);
                    else next.add(column.key);
                    return next;
                  })
                }
              />
              {column.header}
            </label>
          ))}
        </div>
      )}

      {error && <p className="error-note">{error}</p>}

      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              {shown.map((column) => (
                <th
                  key={column.key}
                  className={[column.align === 'end' ? 'end' : '', column.sortValue ? 'sortable' : '']
                    .filter(Boolean)
                    .join(' ')}
                  aria-sort={
                    sort?.key === column.key
                      ? sort.direction === 'asc'
                        ? 'ascending'
                        : 'descending'
                      : undefined
                  }
                >
                  {column.sortValue ? (
                    <button onClick={() => toggleSort(column)}>
                      {column.header}
                      <i aria-hidden="true">
                        {sort?.key === column.key ? (sort.direction === 'asc' ? '▲' : '▼') : '↕'}
                      </i>
                    </button>
                  ) : (
                    column.header
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr className="table-state">
                <td colSpan={shown.length}>{t('table.loading')}</td>
              </tr>
            )}
            {!loading && visibleRows.length === 0 && (
              <tr className="table-state">
                <td colSpan={shown.length}>{empty ?? t('table.empty')}</td>
              </tr>
            )}
            {!loading &&
              visibleRows.map((row) => (
                <tr key={rowKey(row)}>
                  {shown.map((column) => (
                    <td key={column.key} className={column.align === 'end' ? 'end' : ''}>
                      {column.cell(row)}
                    </td>
                  ))}
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      <footer className="table-pager">
        {/* Section 38 wants the range, not just the total: "20 rows" does not
            tell somebody on page three what they are looking at. */}
        <span className="table-count">
          {loading
            ? t('table.loading')
            : t('table.range', {
                from: formatNumber(sorted.length === 0 ? 0 : from + 1),
                to: formatNumber(Math.min(from + pageSize, sorted.length)),
                total: formatNumber(sorted.length),
              })}
        </span>
        <label className="page-size">
          <span>{t('table.perPage')}</span>
          {PAGE_SIZES.map((size) => (
            <button
              key={size}
              className={size === pageSize ? 'size on' : 'size'}
              aria-pressed={size === pageSize}
              onClick={() => {
                setPageSize(size);
                setPage(0);
              }}
            >
              {size}
            </button>
          ))}
        </label>
        <span className="pager-buttons">
          <button className="secondary" disabled={current === 0} onClick={() => setPage(current - 1)}>
            {t('table.previous')}
          </button>
          <span>{t('table.page', { page: current + 1, total: pageCount })}</span>
          <button
            className="secondary"
            disabled={current >= pageCount - 1}
            onClick={() => setPage(current + 1)}
          >
            {t('table.next')}
          </button>
        </span>
      </footer>

    </section>
  );
}
