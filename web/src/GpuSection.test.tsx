import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { parseGpuOverview } from './gpu';
import { GpuSection } from './GpuSection';

const at = '2026-01-01T00:00:00Z';
const metric = (field: string, value: number) => ({ status: 'AVAILABLE', sampledAt: at, ageMs: 0, [field]: value });
const overview = parseGpuOverview(JSON.stringify({ schemaVersion: 1, sampledAt: at, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
  devices: [{ index: 0, utilization: metric('percent', 42.5), memory: { ...metric('usedBytes', 8 * 1024 ** 3), totalBytes: 24 * 1024 ** 3 }, temperature: metric('celsius', 55.25), power: metric('watts', 125.5) }] }))!;
const fresh = { 0: { utilization: true, memory: true, temperature: true, power: true } };

describe('GPU cards', () => {
  it('shows indexed readings using contract units and generic labels', () => {
    render(<GpuSection overview={overview} freshness={fresh} />);
    expect(screen.getByRole('heading', { name: 'GPU 0' })).toBeVisible();
    expect(screen.getByText('42.5%')).toBeVisible();
    expect(screen.getByText('8.0 / 24.0 GiB')).toBeVisible();
    expect(screen.getByText('55.3 °C')).toBeVisible();
    expect(screen.getByText('125.5 W')).toBeVisible();
  });

  it('shows stale or unavailable values without substituting zero', () => {
    const partial = structuredClone(overview);
    partial.devices[0]!.utilization = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, percent: null };
    render(<GpuSection overview={partial} freshness={{ 0: { utilization: false, memory: false, temperature: false, power: false } }} />);
    expect(screen.getAllByText('—')).toHaveLength(4);
    expect(screen.getByText('UNAVAILABLE')).toBeInTheDocument();
    expect(screen.getAllByText('Stale')).toHaveLength(3);
    expect(screen.queryByText('0.0%')).not.toBeInTheDocument();
  });

  it('explains how to enable disabled monitoring', () => {
    render(<GpuSection overview={null} freshness={{}} />);
    expect(screen.getByText(/--gpu/)).toBeVisible();
  });
});
