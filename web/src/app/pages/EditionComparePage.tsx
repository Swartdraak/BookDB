import { useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { compareEditions, type EditionComparison } from '../../api/client';

export function EditionComparePage() {
  const [params] = useSearchParams();
  const left = params.get('left') ?? '';
  const right = params.get('right') ?? '';

  const [comparison, setComparison] = useState<EditionComparison | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!left || !right) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    compareEditions(left, right)
      .then((data) => {
        if (!cancelled) setComparison(data);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to compare editions');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [left, right]);

  const allDifferences = useMemo(() => comparison?.differences ?? [], [comparison]);

  return (
    <div className="page-stack">
      <section className="surface-panel">
        <nav aria-label="Breadcrumb" className="breadcrumb">
          <Link to="/search">Search</Link>
          <span aria-hidden="true">/</span>
          <span aria-current="page">Edition comparison</span>
        </nav>
        <h2>Edition comparison</h2>
        <p className="work-meta">Compare two editions side by side to inspect publication differences.</p>
      </section>

      {!left || !right ? (
        <section className="surface-panel" role="alert">
          <p>Provide both `left` and `right` query parameters to compare editions.</p>
        </section>
      ) : null}

      {loading ? (
        <section className="surface-panel" aria-busy="true">
          <p>Loading comparison…</p>
        </section>
      ) : null}

      {error ? (
        <section className="surface-panel search-error" role="alert">
          <p>{error}</p>
        </section>
      ) : null}

      {comparison ? (
        <section className="surface-panel compare-grid" aria-label="Edition comparison">
          <article className="compare-card">
            <h3>{comparison.left.edition_title}</h3>
            <p className="work-meta">Format: {comparison.left.format ?? 'unknown'}</p>
            <p className="work-meta">ISBN: {comparison.left.isbn13 ?? 'unknown'}</p>
            <p className="work-meta">Publisher: {comparison.left.publisher_name ?? 'unknown'}</p>
            <p className="work-meta">Accessibility: {comparison.left.accessibility.join(', ') || 'none'}</p>
          </article>
          <article className="compare-card">
            <h3>{comparison.right.edition_title}</h3>
            <p className="work-meta">Format: {comparison.right.format ?? 'unknown'}</p>
            <p className="work-meta">ISBN: {comparison.right.isbn13 ?? 'unknown'}</p>
            <p className="work-meta">Publisher: {comparison.right.publisher_name ?? 'unknown'}</p>
            <p className="work-meta">Accessibility: {comparison.right.accessibility.join(', ') || 'none'}</p>
          </article>
          <article className="surface-panel compare-summary" aria-live="polite">
            <h3>Summary</h3>
            <p className="work-meta">Same work: {comparison.same_work ? 'yes' : 'no'}</p>
            <p className="work-meta">Different fields: {allDifferences.length > 0 ? allDifferences.join(', ') : 'none'}</p>
          </article>
        </section>
      ) : null}
    </div>
  );
}

