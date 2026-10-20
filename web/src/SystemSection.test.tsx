import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { parseSystemOverview } from './system';
import { SystemSection } from './SystemSection';

const sample = parseSystemOverview(JSON.stringify({ schemaVersion: 1, sampledAt: '2026-01-01T00:00:00Z', mode: 'DEMO',
  swap: { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:00Z', ageMs: 0, usedBytes: 0, totalBytes: 0 },
  disk: { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:00Z', ageMs: 0, usedBytes: 10 * 1024 ** 3, availableBytes: 4 * 1024 ** 3, totalBytes: 20 * 1024 ** 3 },
  network: { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:00Z', ageMs: 0, rxBytesPerSecond: 0, txBytesPerSecond: 0.5 },
}))!;
const freshness = { swap: { fresh: true, expiry: 6000 }, disk: { fresh: true, expiry: 6000 }, network: { fresh: true, expiry: 6000 } };

afterEach(() => cleanup());

describe('SystemSection', () => {
  it('renders an accessible system detail region with honest units and zero states', () => {
    render(<SystemSection overview={sample} freshness={freshness} />);
    expect(screen.getByRole('region', { name: 'System detail' })).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Swap' })).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Root filesystem' })).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Selected uplinks' })).toBeVisible();
    expect(screen.getByText('No swap configured')).toBeVisible();
    expect(screen.getByText('10.0 / 4.0 / 20.0 GiB')).toBeVisible();
    expect(screen.getByText('RX 0 B/s · TX 0.5 B/s')).toBeVisible();
    expect(screen.queryByText(/internet|link health/i)).not.toBeInTheDocument();
  });

  it('preserves tiny positive rates without changing exact zero or normal values', () => {
    const low = parseSystemOverview(JSON.stringify({ ...sample, network: { ...sample.network, rxBytesPerSecond: 0.004, txBytesPerSecond: 0 } }))!;
    const normal = parseSystemOverview(JSON.stringify({ ...sample, network: { ...sample.network, rxBytesPerSecond: 1_234.5, txBytesPerSecond: 0 } }))!;
    const { rerender } = render(<SystemSection overview={low} freshness={freshness} />);
    expect(screen.getByText('RX <0.01 B/s · TX 0 B/s')).toBeVisible();
    rerender(<SystemSection overview={normal} freshness={freshness} />);
    expect(screen.getByText('RX 1234.5 B/s · TX 0 B/s')).toBeVisible();
  });

  it('explains disabled and initial pending states without inventing configuration details', () => {
    const disabled = parseSystemOverview(JSON.stringify({ schemaVersion: 1, sampledAt: sample.sampledAt, mode: 'DISABLED',
      swap: { status: 'DISABLED', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null },
      disk: { status: 'DISABLED', sampledAt: null, ageMs: null, usedBytes: null, availableBytes: null, totalBytes: null },
      network: { status: 'DISABLED', sampledAt: null, ageMs: null, rxBytesPerSecond: null, txBytesPerSecond: null } }))!;
    const { rerender } = render(<SystemSection overview={disabled} freshness={freshness} />);
    expect(screen.getByText(/disabled.*enable.*system detail/i)).toBeVisible();
    expect(screen.getAllByText('DISABLED')).toHaveLength(3);
    rerender(<SystemSection overview={null} freshness={freshness} />);
    expect(screen.getByText(/Waiting for system detail/)).toBeVisible();
    expect(screen.getAllByText('PENDING')).toHaveLength(3);
  });

  it('hides expired readings and uses dashes for unavailable values', () => {
    const unavailable = parseSystemOverview(JSON.stringify({ ...sample, swap: { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null } }))!;
    render(<SystemSection overview={unavailable} freshness={{ ...freshness, disk: { fresh: false, expiry: 0 } }} />);
    expect(screen.getAllByText('—')).toHaveLength(2);
    expect(screen.getByText('UNAVAILABLE')).toBeVisible();
    expect(screen.getByText('STALE')).toBeVisible();
  });
});
