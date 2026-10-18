import { act, renderHook } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { type MetricSample } from './metrics';
import { STALE_AFTER_MS, useMetrics } from './useMetrics';

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners = new Map<string, (event: Event) => void>();
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(readonly url: string) { FakeEventSource.instances.push(this); }
  addEventListener(type: string, listener: EventListener) { this.listeners.set(type, listener as (event: Event) => void); }
  removeEventListener(type: string) { this.listeners.delete(type); }
  send(data: unknown) { this.listeners.get('snapshot')?.({ data: JSON.stringify(data) } as MessageEvent); }
}

const frame = (sampledAt = new Date(Date.now()).toISOString()): MetricSample => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt, utilizationPercent: 40 },
  memory: { status: 'AVAILABLE', sampledAt, usedBytes: 400, totalBytes: 1000 },
});

afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); FakeEventSource.instances = []; });

describe('useMetrics', () => {
  it('uses one cleaned-up EventSource per mounted dashboard, including StrictMode cycles', () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const { unmount } = renderHook(() => useMetrics(), { wrapper: StrictMode });
    expect(FakeEventSource.instances.length).toBe(2);
    expect(FakeEventSource.instances[0]?.close).toHaveBeenCalledOnce();
    unmount();
    expect(FakeEventSource.instances[1]?.close).toHaveBeenCalledOnce();
  });

  it('rejects invalid frames and treats duplicate timestamps as non-refreshing', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send({ nope: true }));
    expect(result.current.sample).toBeNull();
    const first = frame();
    act(() => source.send(first));
    expect(result.current.cpuFresh).toBe(true);
    act(() => { vi.advanceTimersByTime(STALE_AFTER_MS); });
    expect(result.current.cpuFresh).toBe(false);
    act(() => source.send(first));
    expect(result.current.cpuFresh).toBe(false);
  });

  it('rejects out-of-order envelopes and marks future timestamps stale under clock skew', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send(frame(new Date(Date.now() + 60_000).toISOString())));
    expect(result.current.cpuFresh).toBe(false);
    act(() => source.send(frame(new Date(Date.now() - 60_000).toISOString())));
    expect(result.current.sample?.sampledAt).not.toBe(new Date(Date.now() - 60_000).toISOString());
  });
});
