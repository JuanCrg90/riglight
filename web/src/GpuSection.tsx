import type { GpuDevice, GpuMetricBase, GpuOverview } from './gpu';
import type { GpuFreshness } from './gpuFreshness';

type Props = { overview: GpuOverview | null; freshness: GpuFreshness };

export function GpuSection({ overview, freshness }: Props) {
  return <section className="gpu-section" aria-labelledby="gpu-title">
    <div className="section-heading"><h2 id="gpu-title">GPU</h2><span>Current readings only</span></div>
    {!overview ? <p className="gpu-message">Waiting for GPU telemetry from the local Riglight service.</p> : <>
      {overview.status !== 'AVAILABLE' && overview.status !== 'DISABLED' && <p className="gpu-message" data-state="UNAVAILABLE">{overviewMessage(overview)}</p>}
      {overview.status === 'DISABLED' && <p className="gpu-message">GPU monitoring is disabled. Start Riglight with <code>--gpu</code> to enable read-only GPU readings.</p>}
      {overview.status === 'AVAILABLE' && overview.isTruncated && <p className="gpu-message" data-state="UNAVAILABLE">More than 16 GPU devices were detected; showing the first 16 only.</p>}
      {overview.devices.length > 0 && <div className="gpu-grid">{overview.devices.map(device => <GpuCard key={device.index} device={device} freshness={freshness[device.index]} />)}</div>}
    </>}
  </section>;
}

function overviewMessage(overview: GpuOverview): string {
  if (overview.status === 'UNSUPPORTED') return overview.reasonCode === 'NO_DEVICES' ? 'No supported GPU devices were found.' : 'GPU monitoring is unsupported on this system.';
  if (overview.status === 'NOT_READY') return 'GPU readings are not ready yet.';
  return 'GPU readings are unavailable.';
}

function GpuCard({ device, freshness }: { device: GpuDevice; freshness: GpuFreshness[number] | undefined }) {
  return <article className="gpu-card" aria-labelledby={`gpu-${device.index}-title`}>
    <h3 id={`gpu-${device.index}-title`}>GPU {device.index}</h3>
    <div className="gpu-primary">
      <Reading label="Utilization" metric={device.utilization} fresh={freshness?.utilization} value={device.utilization.percent} format={value => `${value.toFixed(1)}%`} />
      <Reading label="VRAM" metric={device.memory} fresh={freshness?.memory} value={device.memory.usedBytes} format={value => `${(value / 1024 ** 3).toFixed(1)} / ${(device.memory.totalBytes! / 1024 ** 3).toFixed(1)} GiB`} />
    </div>
    <div className="gpu-secondary">
      <Reading label="Temperature" metric={device.temperature} fresh={freshness?.temperature} value={device.temperature.celsius} format={value => `${value.toFixed(1)} °C`} />
      <Reading label="Power" metric={device.power} fresh={freshness?.power} value={device.power.watts} format={value => `${value.toFixed(1)} W`} />
    </div>
  </article>;
}

function Reading({ label, metric, fresh, format, value }: { label: string; metric: GpuMetricBase; fresh?: boolean; format: (value: number) => string; value?: number | null }) {
  const amount = value;
  const state = metric.status === 'AVAILABLE' ? fresh ? 'Fresh' : 'Stale' : metric.status.replace('_', ' ');
  return <div className="gpu-reading">
    <span className="gpu-reading-label">{label}</span>
    <strong>{metric.status === 'AVAILABLE' && fresh && amount !== undefined && amount !== null ? format(amount) : '—'}</strong>
    <span className="gpu-reading-state" data-state={state}>{state}</span>
  </div>;
}