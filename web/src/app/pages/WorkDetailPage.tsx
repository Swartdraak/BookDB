import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  getRichWork,
  getProvenance,
  type Work,
  type RichEdition,
  type ProvenanceRecord,
  type AudioPerformance,
  type SeriesMembership,
  type Asset,
} from '../../api/client';

export function WorkDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [work, setWork] = useState<Work | null>(null);
  const [editions, setEditions] = useState<RichEdition[]>([]);
  const [seriesMembership, setSeriesMembership] = useState<SeriesMembership[]>([]);
  const [audio, setAudio] = useState<AudioPerformance[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [provenance, setProvenance] = useState<ProvenanceRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      try {
        const [w, p] = await Promise.allSettled([
          getRichWork(id),
          getProvenance('work', id),
        ]);
        if (cancelled) return;
        if (w.status === 'fulfilled') {
          setWork(w.value.work);
          setEditions(w.value.editions);
          setSeriesMembership(w.value.series_membership);
          setAudio(w.value.audio_performances);
          setAssets(w.value.assets);
        }
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

      {seriesMembership.length > 0 && (
        <section className="surface-panel">
          <h3>Series</h3>
          <ul className="edition-list" aria-label="Series membership">
            {seriesMembership.map((series) => (
              <li key={series.series_id} className="edition-item">
                <strong>{series.series_title}</strong>
                {typeof series.series_order === 'number' && (
                  <span className="edition-format">#{series.series_order}</span>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {editions.length > 0 && (
        <section className="surface-panel">
          <h3>Editions ({editions.length})</h3>
          {editions.length >= 2 && (
            <p className="work-meta">
              <Link
                className="button-link"
                to={`/compare?left=${encodeURIComponent(editions[0].edition_id)}&right=${encodeURIComponent(editions[1].edition_id)}`}
              >
                Compare first two editions
              </Link>
            </p>
          )}
          <ul className="edition-list" aria-label="Editions">
            {editions.map((ed) => (
              <li key={ed.edition_id} className="edition-item">
                <strong>{ed.edition_title}</strong>
                {ed.publisher_name && <span className="edition-publisher"> — {ed.publisher_name}</span>}
                {ed.publication_date && <span className="edition-date"> ({ed.publication_date.slice(0, 4)})</span>}
                {ed.format && <span className="edition-format"> [{ed.format}]</span>}
                {ed.isbn13 && <span className="edition-isbn"> ISBN: {ed.isbn13}</span>}
                {ed.accessibility.length > 0 && (
                  <span className="edition-format">Accessibility: {ed.accessibility.join(', ')}</span>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {audio.length > 0 && (
        <section className="surface-panel">
          <h3>Audio performances</h3>
          <ul className="edition-list" aria-label="Audio performances">
            {audio.map((perf) => (
              <li key={perf.performance_id} className="edition-item">
                <strong>{perf.narrator_name ?? perf.narrator_id ?? 'Unknown narrator'}</strong>
                {perf.producer_name && <span className="edition-publisher"> — {perf.producer_name}</span>}
                {perf.is_abridged && <span className="edition-format">[abridged]</span>}
              </li>
            ))}
          </ul>
        </section>
      )}

      {assets.length > 0 && (
        <section className="surface-panel">
          <h3>Eligible assets</h3>
          <ul className="edition-list" aria-label="Public assets">
            {assets.map((asset) => (
              <li key={asset.asset_id} className="edition-item">
                <strong>{asset.asset_kind}</strong>
                <a href={asset.url} target="_blank" rel="noreferrer">
                  {asset.url}
                </a>
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

