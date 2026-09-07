import { Link } from 'react-router-dom';

export function NotFoundPage() {
  return (
    <section className="surface-panel not-found-panel">
      <p className="eyebrow">Not found</p>
      <h2>That route is not part of the scaffold.</h2>
      <p>Use the primary navigation to return to the layout shell.</p>
      <Link className="button-link" to="/">
        Back to overview
      </Link>
    </section>
  );
}
