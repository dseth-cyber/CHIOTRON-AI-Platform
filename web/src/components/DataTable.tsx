import { useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from '../LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';

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
  const { themeConfig } = useTheme();
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
    <section className={`rounded-xl border ${themeConfig.border} ${themeConfig.card} overflow-hidden`}>
      <header className={`p-4 border-b ${themeConfig.border} flex items-center justify-between flex-wrap gap-3`}>
        <div className="flex items-center gap-2 flex-wrap">{actions}</div>
        <div className="flex items-center gap-2">
          {trash && (
            <button
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all ${
                trash.showing
                  ? `${themeConfig.buttonGradient} text-white border-transparent`
                  : `${themeConfig.border} ${themeConfig.inputBg} ${themeConfig.text.primary}`
              }`}
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
          <button
            className={`px-3 py-1.5 rounded-lg text-xs font-medium border ${themeConfig.border} ${themeConfig.inputBg} ${themeConfig.text.secondary} hover:${themeConfig.text.primary}`}
            onClick={() => setShowPicker((open) => !open)}
          >
            {t('table.columns')}
          </button>
        </div>
      </header>

      {showPicker && (
        <div className={`p-3 border-b ${themeConfig.border} ${themeConfig.inputBg} flex items-center gap-4 flex-wrap text-xs`}>
          {columns.map((column) => (
            <label key={column.key} className={`flex items-center gap-1.5 cursor-pointer ${themeConfig.text.primary}`}>
              <input
                type="checkbox"
                className="rounded border-gray-400"
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

      {error && <p className="p-4 text-xs text-red-400 bg-red-500/10 border-b border-red-500/20">{error}</p>}

      <div className="overflow-x-auto">
        <table className="w-full text-left">
          <thead className={themeConfig.tableHeader}>
            <tr>
              {shown.map((column) => (
                <th
                  key={column.key}
                  className={`px-4 py-3 text-xs font-semibold tracking-wider ${themeConfig.text.primary} ${
                    column.align === 'end' ? 'text-right' : 'text-left'
                  }`}
                  aria-sort={
                    sort?.key === column.key
                      ? sort.direction === 'asc'
                        ? 'ascending'
                        : 'descending'
                      : undefined
                  }
                >
                  {column.sortValue ? (
                    <button
                      className="inline-flex items-center gap-1.5 focus:outline-none"
                      onClick={() => toggleSort(column)}
                    >
                      <span>{column.header}</span>
                      <span className={`text-[10px] ${themeConfig.text.secondary}`}>
                        {sort?.key === column.key ? (sort.direction === 'asc' ? '▲' : '▼') : '↕'}
                      </span>
                    </button>
                  ) : (
                    column.header
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className={`divide-y ${themeConfig.tableDivide}`}>
            {loading && (
              <tr className={`border-b ${themeConfig.tableBorder}`}>
                <td colSpan={shown.length} className={`p-8 text-center text-sm ${themeConfig.text.secondary}`}>
                  {t('table.loading')}
                </td>
              </tr>
            )}
            {!loading && visibleRows.length === 0 && (
              <tr className={`border-b ${themeConfig.tableBorder}`}>
                <td colSpan={shown.length} className={`p-8 text-center text-sm ${themeConfig.text.secondary}`}>
                  {empty ?? t('table.empty')}
                </td>
              </tr>
            )}
            {!loading &&
              visibleRows.map((row) => (
                <tr key={rowKey(row)} className={`border-b ${themeConfig.tableBorder} ${themeConfig.tableRow}`}>
                  {shown.map((column) => (
                    <td
                      key={column.key}
                      className={`px-4 py-3 text-sm ${themeConfig.text.primary} ${
                        column.align === 'end' ? 'text-right' : 'text-left'
                      }`}
                    >
                      {column.cell(row)}
                    </td>
                  ))}
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      <footer className={`p-4 border-t ${themeConfig.border} flex items-center justify-between flex-wrap gap-3 text-xs`}>
        <span className={themeConfig.text.secondary}>
          {loading
            ? t('table.loading')
            : t('table.range', {
                from: formatNumber(sorted.length === 0 ? 0 : from + 1),
                to: formatNumber(Math.min(from + pageSize, sorted.length)),
                total: formatNumber(sorted.length),
              })}
        </span>
        <div className="flex items-center gap-2">
          <span className={themeConfig.text.secondary}>{t('table.perPage')}</span>
          <div className="flex items-center gap-1">
            {PAGE_SIZES.map((size) => (
              <button
                key={size}
                className={`px-2.5 py-1 rounded-md text-xs font-semibold border transition-all ${
                  size === pageSize
                    ? `${themeConfig.buttonGradient} text-white border-transparent`
                    : `${themeConfig.border} ${themeConfig.inputBg} ${themeConfig.text.secondary} hover:${themeConfig.text.primary}`
                }`}
                aria-pressed={size === pageSize}
                onClick={() => {
                  setPageSize(size);
                  setPage(0);
                }}
              >
                {size}
              </button>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            className={`px-3 py-1.5 rounded-lg text-xs font-medium border ${themeConfig.border} ${themeConfig.inputBg} ${themeConfig.text.primary} disabled:opacity-40 disabled:cursor-not-allowed`}
            disabled={current === 0}
            onClick={() => setPage(current - 1)}
          >
            {t('table.previous')}
          </button>
          <span className={themeConfig.text.secondary}>
            {t('table.page', { page: current + 1, total: pageCount })}
          </span>
          <button
            className={`px-3 py-1.5 rounded-lg text-xs font-medium border ${themeConfig.border} ${themeConfig.inputBg} ${themeConfig.text.primary} disabled:opacity-40 disabled:cursor-not-allowed`}
            disabled={current >= pageCount - 1}
            onClick={() => setPage(current + 1)}
          >
            {t('table.next')}
          </button>
        </div>
      </footer>
    </section>
  );
}
