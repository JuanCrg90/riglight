export function App() {
  return (
    <main className="dashboard">
      <header className="topbar">
        <a className="wordmark" href="#main" aria-label="Riglight home">R<span>·</span></a>
        <div className="product-name">RIGLIGHT <span>/ SYSTEM</span></div>
        <div className="connection"><span className="status-dot" /> Awaiting data</div>
      </header>
      <section id="main" className="overview" aria-labelledby="page-title">
        <p className="eyebrow">HOST MONITOR <span>·</span> MILESTONE 01</p>
        <h1 id="page-title">System overview</h1>
        <p className="lede">Live CPU and memory telemetry, in one quiet place.</p>
        <div className="empty-state" role="status">
          <span className="empty-mark" aria-hidden="true">···</span>
          <div><strong>Waiting for the first sample</strong><p>Metrics appear when the local Riglight service is connected.</p></div>
          <span className="sample-label">NO SAMPLE</span>
        </div>
        <p className="footer-note">Private by design <span>·</span> No history leaves this browser</p>
      </section>
    </main>
  );
}
