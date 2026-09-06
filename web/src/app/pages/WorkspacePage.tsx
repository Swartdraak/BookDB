export function WorkspacePage() {
  return (
    <div className="page-stack">
      <section className="surface-panel">
        <p className="eyebrow">Workspace</p>
        <h2>Developer rails and guardrails</h2>
        <p className="lede">
          This page anchors the repository foundation work without pretending to be a catalog or
          data surface.
        </p>
      </section>

      <section className="content-grid">
        <article className="surface-panel">
          <h3>Local workflow</h3>
          <ul className="check-list">
            <li>Run the Vite dev server for iterative UI work.</li>
            <li>Use the test suite for component-level validation.</li>
            <li>Keep generated client code out of source control until it is explicitly emitted.</li>
          </ul>
        </article>

        <article className="surface-panel">
          <h3>Route map</h3>
          <ul className="check-list">
            <li>Overview for the scaffold summary.</li>
            <li>Workspace for developer-oriented guidance.</li>
            <li>Settings for future configuration surfaces.</li>
          </ul>
        </article>
      </section>
    </div>
  );
}