import { useState } from 'react';
import { fetchGraph, searchKnowledge, type SearchHit, type Subgraph } from '../api';
import { EmptyState, Tag } from '../components/EmptyState';
import { useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import { classificationTone, toneFor } from '../theme';
import { SCOPE_KNOWLEDGE_READ } from '../Connection';

type Mode = 'passages' | 'graph';

/**
 * Knowledge search.
 *
 * Both modes read the same corpus through the same ACL predicates, so the page
 * always says which classifications the answer was drawn from. An empty result
 * with no clearance line would be indistinguishable from an empty corpus.
 */
export function Search() {
  const { t, formatNumber } = useTranslation();
  const { has } = useScopes();
  const [mode, setMode] = useState<Mode>('passages');
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<SearchHit[] | null>(null);
  const [subgraph, setSubgraph] = useState<Subgraph | null>(null);
  const [readable, setReadable] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  if (!has(SCOPE_KNOWLEDGE_READ)) {
    return <EmptyState title={t('search.scope.title')} body={t('search.scope.body')} />;
  }

  const run = async () => {
    const term = query.trim();
    if (term === '' || busy) return;
    setBusy(true);
    setError('');
    setHits(null);
    setSubgraph(null);
    try {
      if (mode === 'passages') {
        const result = await searchKnowledge(term, 10);
        setHits(result?.hits ?? []);
        setReadable(result?.readableClassifications ?? []);
      } else {
        const result = await fetchGraph(term);
        setSubgraph(result.subgraph ?? {});
      }
    } catch (failed) {
      setError(failed instanceof Error ? failed.message : t('search.error'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <section className="page-intro">
        <p>{t('search.intro')}</p>
        <span>{t('search.note')}</span>
      </section>

      <section className="search-bar">
        <div className="mode-switch" role="tablist" aria-label={t('search.mode')}>
          <button
            role="tab"
            aria-selected={mode === 'passages'}
            className={mode === 'passages' ? 'active' : ''}
            onClick={() => setMode('passages')}
          >
            {t('search.mode.passages')}
          </button>
          <button
            role="tab"
            aria-selected={mode === 'graph'}
            className={mode === 'graph' ? 'active' : ''}
            onClick={() => setMode('graph')}
          >
            {t('search.mode.graph')}
          </button>
        </div>
        <input
          value={query}
          placeholder={mode === 'passages' ? t('search.placeholder') : t('search.placeholderGraph')}
          aria-label={t('nav.search')}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') void run();
          }}
        />
        <button className="primary" disabled={busy || query.trim() === ''} onClick={() => void run()}>
          {busy ? t('search.busy') : t('nav.search')}
        </button>
      </section>

      {error !== '' && <p className="error-note">{error}</p>}

      {readable.length > 0 && mode === 'passages' && (
        <p className="source-note">{t('documents.readable', { list: readable.join(', ') })}</p>
      )}

      {hits !== null && hits.length === 0 && (
        <EmptyState title={t('search.empty.title')} body={t('search.empty.body')} />
      )}

      {hits !== null && hits.length > 0 && (
        <section className="hit-list">
          {hits.map((hit, index) => (
            <article className="hit" key={hit.chunkId}>
              <header>
                <span className="hit-index">{String(index + 1).padStart(2, '0')}</span>
                <b>{hit.documentTitle}</b>
                <Tag tone={toneFor(classificationTone, hit.classification)}>{hit.classification}</Tag>
              </header>
              <p>{hit.content}</p>
              <footer>
                {t('search.hitScore', {
                  score: hit.score.toFixed(4),
                  ordinal: formatNumber(hit.ordinal + 1),
                })}
                {hit.vectorScore !== undefined &&
                  ` · ${t('search.vector', { score: hit.vectorScore.toFixed(4) })}`}
                {hit.keywordScore !== undefined &&
                  ` · ${t('search.keyword', { score: hit.keywordScore.toFixed(4) })}`}
              </footer>
            </article>
          ))}
        </section>
      )}

      {subgraph !== null && <GraphResult subgraph={subgraph} />}
    </>
  );
}

function GraphResult({ subgraph }: { subgraph: Subgraph }) {
  const { t, formatNumber } = useTranslation();
  const nodes = subgraph.nodes ?? [];
  const edges = subgraph.edges ?? [];
  const seeds = subgraph.seeds ?? [];

  if (nodes.length === 0) {
    return <EmptyState title={t('search.graphEmpty.title')} body={t('search.graphEmpty.body')} />;
  }

  const byId = new Map(nodes.map((node) => [node.id, node]));
  const name = (id: string) => byId.get(id)?.name ?? id;

  return (
    <section className="graph-result">
      <p className="source-note">
        {t('search.graphSummary', {
          nodes: formatNumber(nodes.length),
          edges: formatNumber(edges.length),
          seeds: formatNumber(seeds.length),
        })}
        {subgraph.truncated ? ` · ${t('search.graphTruncated')}` : ''}
      </p>
      <div className="graph-columns">
        <section className="panel">
          <span className="panel-label">{t('search.graph.entities')}</span>
          <ul className="plain-list">
            {nodes.map((node) => (
              <li key={node.id}>
                <span className="list-row static">
                  <b>
                    {node.name}{' '}
                    {seeds.some((seed) => seed.id === node.id) && <Tag tone="ok">{t('search.graph.seed')}</Tag>}
                  </b>
                  <small>
                    {node.kind} · {t('search.graph.mentions', { count: formatNumber(node.mentionCount) })}{' '}
                    <Tag tone={toneFor(classificationTone, node.classification)}>{node.classification}</Tag>
                  </small>
                </span>
              </li>
            ))}
          </ul>
        </section>
        <section className="panel">
          <span className="panel-label">{t('search.graph.relations')}</span>
          {edges.length === 0 && <p className="history-hint">{t('search.graph.noRelations')}</p>}
          <ul className="plain-list">
            {edges.map((edge) => (
              <li key={`${edge.sourceId}:${edge.relation}:${edge.targetId}`}>
                <span className="list-row static">
                  <b>
                    {name(edge.sourceId)} → {name(edge.targetId)}
                  </b>
                  <small>
                    {edge.relation} · {t('search.graph.weight', { weight: formatNumber(edge.weight) })}
                  </small>
                </span>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </section>
  );
}
