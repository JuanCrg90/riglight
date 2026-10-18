import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { Sparkline } from './Sparkline';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('dashboard shell', () => {
  it('draws separate chart segments across missing samples', () => {
    const { container } = render(<Sparkline label="CPU history" values={[10, null, 20]} />);
    expect(screen.getByRole('img', { name: 'CPU history' })).toBeVisible();
    expect(container.querySelectorAll('path.sparkline-path')).toHaveLength(2);
  });

  it.each([
    [52.123456, '52.1%'],
    [0, '0.0%'],
    [100, '100.0%'],
  ])('formats CPU utilization %s as %s', (utilizationPercent, displayValue) => {
    const listeners = new Map<string, (event: Event) => void>();
    vi.stubGlobal('EventSource', class {
      addEventListener(type: string, listener: EventListener) { listeners.set(type, listener as (event: Event) => void); }
      removeEventListener(type: string) { listeners.delete(type); }
      close() {}
    });
    render(<App />);
    const timestamp = '2026-01-01T00:00:00Z';
    act(() => listeners.get('snapshot')?.({ data: JSON.stringify({
      schemaVersion: 1, sampledAt: timestamp, mode: 'DEMO',
      cpu: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: 0, utilizationPercent },
      memory: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: 0, usedBytes: 8_000_000_000, totalBytes: 16_000_000_000 },
    }) } as MessageEvent));
    expect(screen.getByRole('region', { name: 'CPU utilization' })).toHaveTextContent(displayValue);
  });

  it('labels binary memory values in GiB', () => {
    const listeners = new Map<string, (event: Event) => void>();
    vi.stubGlobal('EventSource', class {
      addEventListener(type: string, listener: EventListener) { listeners.set(type, listener as (event: Event) => void); }
      removeEventListener(type: string) { listeners.delete(type); }
      close() {}
    });
    render(<App />);
    const timestamp = '2026-01-01T00:00:00Z';
    act(() => listeners.get('snapshot')?.({ data: JSON.stringify({
      schemaVersion: 1, sampledAt: timestamp, mode: 'DEMO',
      cpu: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: 0, utilizationPercent: 40 },
      memory: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: 0, usedBytes: 8_000_000_000, totalBytes: 16_000_000_000 },
    }) } as MessageEvent));
    expect(screen.getByRole('region', { name: 'Memory used' })).toHaveTextContent('7.5 / 14.9 GiB');
  });

  it('describes generic private workstation telemetry', () => {
    vi.stubGlobal('EventSource', class { addEventListener() {} removeEventListener() {} close() {} });
    render(<App />);
    expect(screen.getByText('READ-ONLY HOST MONITOR')).toBeVisible();
    expect(screen.getByText('Private workstation telemetry, with optional read-only integrations.')).toBeVisible();
  });

  it('renders model inventory from the existing named event', () => {
    const listeners = new Map<string, (event: Event) => void>();
    vi.stubGlobal('EventSource', class {
      addEventListener(type: string, listener: EventListener) { listeners.set(type, listener as (event: Event) => void); }
      removeEventListener(type: string) { listeners.delete(type); }
      close() {}
    });
    render(<App />);
    act(() => listeners.get('models')?.({ data: JSON.stringify({
      schemaVersion: 1, sampledAt: '2026-01-01T00:00:00Z', mode: 'DEMO', status: 'AVAILABLE',
      reasonCode: null, ageMs: 0, isTruncated: false, models: [{ id: 'example-chat', state: 'UNKNOWN' }],
    }) } as MessageEvent));
    expect(screen.getByRole('region', { name: 'llama-swap' })).toBeVisible();
    expect(screen.getByText('example-chat')).toBeVisible();
    expect(screen.getByText('Unknown')).toBeVisible();
  });

  it('describes the pending state without inventing telemetry', () => {
    vi.stubGlobal('EventSource', class { addEventListener() {} removeEventListener() {} close() {} });
    render(<App />);
    expect(screen.getByRole('heading', { name: 'System overview' })).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for the first sample');
    expect(screen.getByText('NO SAMPLE')).toBeVisible();
    expect(screen.queryByText(/%/)).not.toBeInTheDocument();
  });
});
