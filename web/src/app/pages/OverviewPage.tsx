export function OverviewPage() {
  return (
    <div className="page-stack">
      <section className="hero-panel surface-panel">
        <div className="hero-copy">
          <p className="eyebrow">Foundation</p>
          <h2>Intentional, accessible, and ready for the next slice.</h2>
          <p className="lede">
            The scaffold gives the frontend a stable route/layout architecture, a clear validation
            toolchain, and a defined location for generated API clients.
          </p>
        </div>

        <div className="metric-grid" aria-label="Scaffold highlights">
          <article className="metric-card">
            <span className="metric-label">Routing</span>
            <strong>Nested layout shell</strong>
            <p>Home, workspace, settings, and fallback routes.</p>
          </article>
          <article className="metric-card">
            <span className="metric-label">Quality</span>
            <strong>Lint, typecheck, test, build</strong>
            <p>All wired as web-local scripts.</p>
          </article>
          <article className="metric-card">
            <span className="metric-label">API output</span>
            <strong>src/api/generated/</strong>
            <p>Reserved for the future OpenAPI client generator.</p>
          </article>
        </div>
      </section>

      <section className="content-grid">
        <article className="surface-panel">
          <h3>What this shell already covers</h3>
          <ul className="check-list">
            <li>Semantic landmarks, skip link, and visible focus states.</li>
            <li>Responsive layout with a compact nav rail.</li>
            <li>Scaffold content that describes the app rather than filler.</li>
          </ul>
        </article>

        <article className="surface-panel">
          <h3>Validation commands</h3>
          <ul className="command-list">
            <li>
              <code>npm run typecheck</code>
            </li>
            <li>
              <code>npm run lint</code>
            </li>
            <li>
              <code>npm run test</code>
            </li>
            <li>
              <code>npm run build</code>
            </li>
          </ul>
        </article>
      </section>
    </div>
  );
}
