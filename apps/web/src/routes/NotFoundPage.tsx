import { Link } from '@tanstack/react-router';

export function NotFoundPage() {
  return (
    <div className="route-page route-page--centered">
      <section className="not-found" aria-labelledby="not-found-title">
        <p className="route-page__eyebrow">404</p>
        <h1 id="not-found-title">Page not found</h1>
        <p>The requested route does not exist in this workspace.</p>
        <Link className="text-link" to="/overview">
          Return to overview
        </Link>
      </section>
    </div>
  );
}
