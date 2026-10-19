import type { ContainerFreshness } from './containerFreshness';
import type { ContainerOverview, ContainerRow } from './containers';

export function ContainerSection({ overview, freshness }: { overview: ContainerOverview | null; freshness: ContainerFreshness }) {
  const state = overview?.status ?? 'DISABLED';
  const message = !overview ? 'Not enabled. Container monitoring is opt-in and requires the filtering helper.' :
    state === 'DISABLED' ? 'Not enabled. Container monitoring is opt-in and requires the filtering helper.' :
    state === 'NOT_READY' ? 'Waiting for container data.' :
    state === 'UNSUPPORTED' ? 'The configured proxy does not support this API.' :
    state === 'UNAVAILABLE' ? overview.reasonCode === 'AUTH_REQUIRED' ? 'The proxy requires authorization.' : `Container source unavailable (${overview.reasonCode?.replaceAll('_', ' ').toLowerCase() ?? 'unknown'}).` :
    !freshness.overview ? 'Container overview is stale. Waiting for a new sample.' : null;
  return <section className="container-section" aria-label="Selected containers">
    <div className="section-heading"><h2>Selected containers</h2><span>READ-ONLY</span></div>
    {message ? <p className="container-message" role="status">{message}</p> : <>
      <p className="container-note">CPU is measured on a one-core basis and may exceed 100%. Memory subtracts inactive cache; total is the reported limit, not a quota guarantee.</p>
      <ul className="containers-list">{overview?.containers.map(row => <ContainerCard key={row.alias} row={row} fresh={freshness.rows[row.alias]} />)}</ul>
    </>}
  </section>;
}
function ContainerCard({ row, fresh }: { row: ContainerRow; fresh: ContainerFreshness['rows'][string] | undefined }) {
  const metadata = !fresh?.metadata ? 'Stale' : row.metadata.status === 'AVAILABLE' ? null : row.metadata.status.replaceAll('_', ' ');
  const sensorState = (status: string, isFresh: boolean | undefined) => !isFresh && status === 'AVAILABLE' ? 'Stale' : status.replaceAll('_', ' ');
  return <li className="container-card">
    <h3>{row.alias}</h3>
    <div className="container-readings">
      <div><span>State / health</span><strong>{metadata ?? `${row.metadata.state} · ${row.metadata.health}`}</strong><small>{metadata ?? `Restarts: ${row.metadata.restartCount ?? 'Unknown'}`}</small></div>
      <div><span>CPU · one core = 100%</span><strong>{fresh?.cpu && row.cpu.status === 'AVAILABLE' ? `${row.cpu.percent!.toFixed(1)}%` : '—'}</strong><small>{sensorState(row.cpu.status, fresh?.cpu)}</small></div>
      <div><span>Cache-adjusted memory</span><strong>{fresh?.memory && row.memory.status === 'AVAILABLE' ? `${formatBytes(row.memory.usedBytes!)} / ${formatBytes(row.memory.totalBytes!)}` : '—'}</strong><small>{sensorState(row.memory.status, fresh?.memory)}</small></div>
    </div>
  </li>;
}
function formatBytes(bytes: number): string {
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(0)} KiB`;
  return `${(bytes / 1024 ** 3).toFixed(2)} GiB`;
}
