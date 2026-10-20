import { Sparkline } from './Sparkline';
import { GpuSection } from './GpuSection';
import { ModelSection } from './ModelSection';
import { ContainerSection } from './ContainerSection';
import { SystemSection } from './SystemSection';
import { useMetrics } from './useMetrics';

export function App() {
  const { sample, connection, cpuFresh, memoryFresh, cpuHistory, memoryHistory, gpu, gpuFreshness, models, modelsFresh, containers, containersFreshness, system, systemFreshness } = useMetrics();
  const hasSample = sample !== null;
  return (
    <main className="dashboard">
      <header className="topbar">
        <a className="wordmark" href="#main" aria-label="Riglight home">R<span>·</span></a>
        <div className="product-name">RIGLIGHT <span>/ SYSTEM</span></div>
        <div className="connection"><span className="status-dot" /> {connection === 'RECONNECTING' ? 'Reconnecting' : connection === 'CONNECTED' ? 'Connected' : 'Connecting'}</div>
      </header>
      <section id="main" className="overview" aria-labelledby="page-title">
        <p className="eyebrow">READ-ONLY HOST MONITOR</p>
        <h1 id="page-title">System overview</h1>
        <p className="lede">Private workstation telemetry, with optional read-only integrations.</p>
        {hasSample ? <>
          <p className="mode-label">{sample.mode} MODE</p>
          <div className="metric-row">
            <Metric label="CPU utilization" value={sample.cpu.status === 'AVAILABLE' ? `${sample.cpu.utilizationPercent!.toFixed(1)}%` : '—'} state={sample.cpu.status === 'AVAILABLE' ? cpuFresh ? 'Fresh' : 'Stale' : sample.cpu.status.replace('_', ' ')}>
              <Sparkline label="CPU utilization history" values={cpuHistory.map(point => point.value)} />
            </Metric>
            <Metric label="Memory used" value={sample.memory.status === 'AVAILABLE' ? `${(sample.memory.usedBytes! / 1024 ** 3).toFixed(1)} / ${(sample.memory.totalBytes! / 1024 ** 3).toFixed(1)} GiB` : '—'} state={sample.memory.status === 'AVAILABLE' ? memoryFresh ? 'Fresh' : 'Stale' : sample.memory.status.replace('_', ' ')}>
              <Sparkline label="Memory utilization history" values={memoryHistory.map(point => point.value)} />
            </Metric>
          </div>
        </> : <div className="empty-state" role="status">
          <span className="empty-mark" aria-hidden="true">···</span>
          <div><strong>Waiting for the first sample</strong><p>Metrics appear when the local Riglight service is connected.</p></div>
          <span className="sample-label">NO SAMPLE</span>
        </div>}
        <SystemSection overview={system} freshness={systemFreshness} />
        <GpuSection overview={gpu} freshness={gpuFreshness} />
        <ModelSection overview={models} fresh={modelsFresh} />
        <ContainerSection overview={containers} freshness={containersFreshness} />
        <p className="footer-note">Private by design <span>·</span> No history leaves this browser</p>
      </section>
    </main>
  );
}

function Metric({ label, value, state, children }: { label: string; value: string; state: string; children: React.ReactNode }) {
  return <section className="metric" aria-label={label}>
    <p>{label}</p><strong>{value}</strong><span data-state={state}>{state}</span>{children}
  </section>;
}
