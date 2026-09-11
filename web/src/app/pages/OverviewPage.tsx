import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getQualityCoverage, type QualityReport } from '../../api/client';

export function OverviewPage() {
  const [report, setReport] = useState<QualityReport | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getQualityCoverage()
      .then((next) => {
        if (!cancelled) setReport(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load quality coverage');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="page-stack">
      <section className="hero-panel surface-panel">
        <div className="hero-copy">
          <p className="eyebrow">Catalog quality</p>
          <h2>Coverage by language, format, and source.</h2>
          <p className="lede">
            This dashboard reports known metadata fields and source evidence. Unknown values remain unknown.
          </p>
        </div>

        <div className="metric-grid" aria-label="Quality summary">
          <article className="metric-card">
            <span className="metric-label">Format groups</span>
            <strong>{report?.by_format_language.length ?? 0}</strong>
            <p>Language/format rows currently observed.</p>
          </article>
          <article className="metric-card">
            <span className="metric-label">Sources</span>
            <strong>{report?.by_source.length ?? 0}</strong>
            <p>Evidence sources in the current snapshot.</p>
          </article>
          <article className="metric-card">
            <span className="metric-label">Discover</span>
            <strong>Search and compare</strong>
            <p>
              Use <Link to="/search">search</Link> to open a work and compare editions.
            </p>
          </article>
        </div>
      </section>

      {error ? (
        <section className="surface-panel search-error" role="alert">
          <p>{error}</p>
        </section>
      ) : null}

      {report ? (
        <section className="content-grid">
          <article className="surface-panel">
            <h3>Coverage by format and language</h3>
            <table className="quality-table">
              <thead>
                <tr>
                  <th>Format</th>
                  <th>Language</th>
                  <th>Editions</th>
                  <th>ISBN known</th>
                  <th>Publication date known</th>
                  <th>Publisher known</th>
                </tr>
              </thead>
              <tbody>
                {report.by_format_language.map((row) => (
                  <tr key={`${row.format}-${row.language_code}`}>
                    <td>{row.format}</td>
                    <td>{row.language_code}</td>
                    <td>{row.editions}</td>
                    <td>{row.known_isbn}</td>
                    <td>{row.known_publication_date}</td>
                    <td>{row.known_publisher}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </article>

          <article className="surface-panel">
            <h3>Coverage by source</h3>
            <table className="quality-table">
              <thead>
                <tr>
                  <th>Source</th>
                  <th>Records</th>
                </tr>
              </thead>
              <tbody>
                {report.by_source.map((row) => (
                  <tr key={row.source_name}>
                    <td>{row.source_name}</td>
                    <td>{row.records}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </article>
        </section>
      ) : null}
    </div>
  );
}
