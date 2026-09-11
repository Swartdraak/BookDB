import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { searchWorks, type SearchResult } from '../../api/client';

export function SearchPage() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const doSearch = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const res = await searchWorks(query.trim());
      setResults(res.results);
      setTotal(res.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
    } finally {
      setLoading(false);
    }
  }, [query]);

  return (
    <div className="page-stack">
      <section className="surface-panel">
        <h2>Search the Catalog</h2>
        <p className="lede">
          Search across works, editions, and people in the BookDB catalog.
        </p>
        <form onSubmit={doSearch} className="search-form" role="search">
          <label htmlFor="search-input" className="visually-hidden">
            Search query
          </label>
          <input
            id="search-input"
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search by title, author, or keyword…"
            className="search-input"
            aria-label="Search query"
          />
          <button type="submit" className="button-link" disabled={loading || !query.trim()}>
            {loading ? 'Searching…' : 'Search'}
          </button>
        </form>
      </section>

      {error && (
        <section className="surface-panel search-error" role="alert">
          <p>{error}</p>
        </section>
      )}

      {results.length > 0 && (
        <section className="surface-panel">
          <h3>
            {total} result{total !== 1 ? 's' : ''} for &ldquo;{query}&rdquo;
          </h3>
          <ul className="search-results" aria-label="Search results">
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

      {!loading && !error && results.length === 0 && query && (
        <section className="surface-panel">
          <p>No results found.</p>
        </section>
      )}
    </div>
  );
}

