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

const frame = (sampledAt = new Date(Date.now()).toISOString(), ageMs = 0): MetricSample => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt, ageMs, utilizationPercent: 40 },
  memory: { status: 'AVAILABLE', sampledAt, ageMs, usedBytes: 400, totalBytes: 1000 },
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
    expect(result.current.cpuHistory.at(-1)?.value).toBeNull();
    act(() => source.send(first));
    expect(result.current.cpuFresh).toBe(false);
  });

  it('accepts regressing envelope times and applies new metric states', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send(frame('2026-02-01T00:00:00Z', 0)));
    const regressed = frame('2025-12-31T23:59:59Z', 0);
    regressed.memory = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null };
    act(() => source.send(regressed));
    expect(result.current.sample?.sampledAt).toBe('2025-12-31T23:59:59Z');
    expect(result.current.cpuFresh).toBe(true);
    expect(result.current.sample?.memory.status).toBe('UNAVAILABLE');
    expect(result.current.memoryFresh).toBe(false);
  });

  it('uses server age regardless of clock skew and honors aged cached samples', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send(frame(new Date(Date.now() + 60_000).toISOString(), 0)));
    expect(result.current.cpuFresh).toBe(true);
    act(() => source.send(frame(new Date(Date.now() + 61_000).toISOString(), STALE_AFTER_MS)));
    expect(result.current.cpuFresh).toBe(false);
  });

  it('does not let a host clock behind the browser mark a fresh sample stale', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    act(() => FakeEventSource.instances[0]!.send(frame(new Date(Date.now() - 60_000).toISOString(), 0)));
    expect(result.current.cpuFresh).toBe(true);
  });

  it('does not extend freshness when the same metric sample is replayed', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    const time = new Date(Date.now()).toISOString();
    act(() => source.send(frame(time, 5_000)));
    expect(result.current.cpuFresh).toBe(true);
    act(() => vi.advanceTimersByTime(500));
    const replay = frame(new Date(Date.now() + 1_000).toISOString(), 0);
    replay.cpu.sampledAt = time;
    replay.memory.sampledAt = time;
    act(() => source.send(replay));
    act(() => vi.advanceTimersByTime(500));
    expect(result.current.cpuFresh).toBe(false);
  });
});
