export function App() {
  return (
    <main className="smoke-screen">
      <section className="smoke-card" aria-labelledby="smoke-title">
        <p className="product-name">Stackplore</p>
        <h1 id="smoke-title">Workspace is ready.</h1>
        <p className="smoke-copy">The web and API foundations now run from one repository.</p>
        <p className="health-route">
          API liveness: <code>GET /healthz</code>
        </p>
      </section>
    </main>
  );
}
