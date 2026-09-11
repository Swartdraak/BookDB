import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getWork, getEditions, getProvenance, type Work, type Edition, type ProvenanceRecord } from '../../api/client';

export function WorkDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [work, setWork] = useState<Work | null>(null);
  const [editions, setEditions] = useState<Edition[]>([]);
  const [provenance, setProvenance] = useState<ProvenanceRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      try {
        const [w, e, p] = await Promise.allSettled([
          getWork(id),
          getEditions(id),
          getProvenance('work', id),
        ]);
        if (cancelled) return;
        if (w.status === 'fulfilled') setWork(w.value);
        if (e.status === 'fulfilled') setEditions(e.value.editions);
        if (p.status === 'fulfilled') setProvenance(p.value.source_records);
        if (w.status === 'rejected') {
          setError(w.reason instanceof Error ? w.reason.message : 'Failed to load work');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [id]);

  if (loading) {
    return (
      <div className="page-stack">
        <section className="surface-panel" aria-busy="true">
          <p>Loading work…</p>
        </section>
      </div>
    );
  }

  if (error || !work) {
    return (
      <div className="page-stack">
        <section className="surface-panel" role="alert">
          <h2>Work not found</h2>
          <p>{error ?? 'This work does not exist in the catalog.'}</p>
          <Link to="/search" className="button-link">Back to search</Link>
        </section>
      </div>
    );
  }

  return (
    <div className="page-stack">
      <section className="surface-panel">
        <nav aria-label="Breadcrumb" className="breadcrumb">
          <Link to="/search">Search</Link>
          <span aria-hidden="true">/</span>
          <span aria-current="page">{work.canonical_title}</span>
        </nav>
        <h2>{work.canonical_title}</h2>
        {work.language_code && (
          <p className="work-meta">Language: {work.language_code}</p>
        )}
        <p className="work-meta">
          Work ID: <code>{work.work_id}</code>
        </p>
      </section>

      {editions.length > 0 && (
        <section className="surface-panel">
          <h3>Editions ({editions.length})</h3>
          <ul className="edition-list" aria-label="Editions">
            {editions.map((ed) => (
              <li key={ed.edition_id} className="edition-item">
                <strong>{ed.edition_title}</strong>
                {ed.publisher_name && <span className="edition-publisher"> — {ed.publisher_name}</span>}
                {ed.publication_date && <span className="edition-date"> ({ed.publication_date.slice(0, 4)})</span>}
                {ed.format && <span className="edition-format"> [{ed.format}]</span>}
                {ed.isbn13 && <span className="edition-isbn"> ISBN: {ed.isbn13}</span>}
              </li>
            ))}
          </ul>
        </section>
      )}

      {provenance.length > 0 && (
        <section className="surface-panel">
          <h3>Source Provenance</h3>
          <ul className="provenance-list" aria-label="Source provenance">
            {provenance.map((p, i) => (
              <li key={i} className="provenance-item">
                <span className="provenance-source">{p.source_name}</span>
                <code className="provenance-key">{p.source_key}</code>
                <time className="provenance-date" dateTime={p.created_at}>
                  {new Date(p.created_at).toLocaleDateString()}
                </time>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

