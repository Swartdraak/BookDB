export function SettingsPage() {
  return (
    <div className="page-stack">
      <section className="surface-panel">
        <p className="eyebrow">Settings</p>
        <h2>Reserved configuration surface</h2>
        <p className="lede">
          The scaffold leaves room for web-local configuration without inventing domain behavior.
        </p>
      </section>

      <section className="content-grid">
        <article className="surface-panel">
          <h3>API client generation</h3>
          <p>
            Generated OpenAPI client code is expected to land in <code>src/api/generated/</code>.
          </p>
        </article>

        <article className="surface-panel">
          <h3>Current status</h3>
          <p>The directory exists now, but no client has been generated yet.</p>
        </article>
      </section>
    </div>
  );
}