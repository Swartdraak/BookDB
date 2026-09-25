import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { searchWorks, type SearchResult } from '../../api/client';

/**
 * SearchPage implements the S2 real-data search surface (#23):
 * honest loading / empty / error states, Unicode input, and keyboard
 * navigation (ArrowUp/ArrowDown/Home/End move focus between results,
 * Enter opens the focused result). Results are real API data — there is no
 * mock fallback; when the search backend is down the page says so.
 */
export function SearchPage() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);
  const navigate = useNavigate();
  const resultsRef = useRef<HTMLUListElement | null>(null);

  const doSearch = async () => {
    const q = query.trim();
    if (!q) return;
    setLoading(true);
    setError(null);
    setSearched(true);
    try {
      const res = await searchWorks(q);
      setResults(res.results);
      setTotal(res.total);
    } catch (err) {
      setResults([]);
      setTotal(0);
      setError(err instanceof Error ? err.message : 'Search failed');
    } finally {
      setLoading(false);
    }
  };

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void doSearch();
  };

  const focusResult = (index: number) => {
    const list = resultsRef.current;
    if (!list) return;
    const buttons = Array.from(list.querySelectorAll<HTMLButtonElement>('button.result-link'));
    const clamped = Math.max(0, Math.min(index, buttons.length - 1));
    if (buttons[clamped]) buttons[clamped].focus();
  };

  const onInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      focusResult(0);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      focusResult(results.length - 1);
    }
  };

  const onListKeyDown = (e: React.KeyboardEvent<HTMLUListElement>) => {
    const buttons = Array.from(
      e.currentTarget.querySelectorAll<HTMLButtonElement>('button.result-link'),
    );
    const currentIndex = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (currentIndex >= 0) focusResult(currentIndex + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (currentIndex > 0) focusResult(currentIndex - 1);
    } else if (e.key === 'Home') {
      e.preventDefault();
      focusResult(0);
    } else if (e.key === 'End') {
      e.preventDefault();
      focusResult(results.length - 1);
    } else if (e.key === 'Enter' && currentIndex >= 0) {
      // Enter on a focused result button submits naturally; nothing to do.
    }
  };

  // When results arrive, move focus to the first result for keyboard flow.
  useEffect(() => {
    if (searched && results.length > 0) focusResult(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [results]);

  const statusText = loading
    ? 'Searching…'
    : error
      ? `Search error: ${error}`
      : searched
        ? `${total} result${total !== 1 ? 's' : ''} for ${query.trim()}`
        : '';

  return (
    <div className="page-stack">
      <section className="surface-panel">
        <h2>Search the Catalog</h2>
        <p className="lede">Search across works, editions, and people in the BookDB catalog.</p>
        <form onSubmit={onSubmit} className="search-form" role="search">
          <label htmlFor="search-input" className="visually-hidden">
            Search query
          </label>
          <input
            id="search-input"
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onInputKeyDown}
            placeholder="Search by title, author, or keyword…"
            className="search-input"
            aria-label="Search query"
            aria-describedby="search-status"
            autoComplete="off"
          />
          <button type="submit" className="button-link" disabled={loading || !query.trim()}>
            {loading ? 'Searching…' : 'Search'}
          </button>
        </form>
        <p id="search-status" className="search-status" aria-live="polite" role="status">
          {statusText}
        </p>
      </section>

      {error && (
        <section className="surface-panel search-error" role="alert">
          <h3>Search unavailable</h3>
          <p>{error}</p>
          <p className="work-meta">
            The search backend may be offline. Try again, or open a known work by ID.
          </p>
        </section>
      )}

      {loading && (
        <section className="surface-panel" aria-busy="true">
          <p className="search-loading">Loading results…</p>
        </section>
      )}

      {!loading && !error && searched && results.length > 0 && (
        <section className="surface-panel">
          <h3>
            {total} result{total !== 1 ? 's' : ''} for &ldquo;{query.trim()}&rdquo;
          </h3>
          <ul
            ref={resultsRef}
            className="search-results"
            aria-label="Search results"
            onKeyDown={onListKeyDown}
          >
            {results.map((r) => (
              <li key={r._id} className="search-result-item">
                <button
                  type="button"
                  className="result-link"
                  onClick={() => navigate(`/works/${r._id}`)}
                >
                  <strong>{r.title || r._id}</strong>
                  {r.authors && <span className="result-authors"> — {r.authors}</span>}
                  {r.language && <span className="result-lang"> [{r.language}]</span>}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}

      {!loading && !error && searched && results.length === 0 && (
        <section className="surface-panel search-empty">
          <h3>No results</h3>
          <p>
            No works match &ldquo;{query.trim()}&rdquo;. Check the spelling or try a different title
            or author.
          </p>
        </section>
      )}
    </div>
  );
}
