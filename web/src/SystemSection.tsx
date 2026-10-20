import type { SystemOverview } from './system';
import type { SystemFreshness, SystemSensorName } from './systemFreshness';

type Props = { overview: SystemOverview | null; freshness: SystemFreshness };
type ReadingProps = { name: SystemSensorName; title: string; overview: SystemOverview | null; freshness: SystemFreshness };
const GIB = 1024 ** 3;

export function SystemSection({ overview, freshness }: Props) {
  return <section className="system-section" aria-label="System detail">
    <div className="section-heading"><h2>System detail</h2><span>Read-only</span></div>
    <div className="system-grid">
      <Reading name="swap" title="Swap" overview={overview} freshness={freshness} />
      <Reading name="disk" title="Root filesystem" overview={overview} freshness={freshness} />
      <Reading name="network" title="Selected uplinks" overview={overview} freshness={freshness} />
    </div>
  </section>;
}

function Reading({ name, title, overview, freshness }: ReadingProps) {
  let state: string = overview ? overview[name].status : 'PENDING';
  let value = '—';
  if (overview && overview[name].status === 'AVAILABLE') {
    if (!freshness[name].fresh) state = 'STALE';
    else if (name === 'swap' && overview.swap.totalBytes === 0) value = 'No swap configured';
    else if (name === 'swap') value = `${gib(overview.swap.usedBytes!)} / ${gib(overview.swap.totalBytes!)} GiB`;
    else if (name === 'disk') value = `${gib(overview.disk.usedBytes!)} / ${gib(overview.disk.availableBytes!)} / ${gib(overview.disk.totalBytes!)} GiB`;
    else value = `RX ${rate(overview.network.rxBytesPerSecond!)} · TX ${rate(overview.network.txBytesPerSecond!)}`;
    if (state === 'STALE') value = '—';
  }
  return <article className="system-card">
    <h3>{title}</h3>
    <p className="system-reading" aria-label={readingLabel(name)}>{value}</p>
    <span className="system-state" data-state={state}>{state}</span>
    {name === 'disk' && <p className="system-labels">Used / Available / Total (GiB)</p>}
    {name === 'swap' && overview?.swap.status === 'AVAILABLE' && overview.swap.totalBytes !== 0 && <p className="system-labels">Used / Total (GiB)</p>}
    {name === 'network' && <p className="system-labels">Receive / Transmit (B/s)</p>}
  </article>;
}

function gib(bytes: number): string { return (bytes / GIB).toFixed(1); }
function rate(bytesPerSecond: number): string { return `${Number(bytesPerSecond.toFixed(2))} B/s`; }
function readingLabel(name: SystemSensorName): string {
  return name === 'swap' ? 'Swap used and total GiB' : name === 'disk' ? 'Root filesystem used available and total GiB' : 'Selected uplinks RX and TX byte-rates';
}

