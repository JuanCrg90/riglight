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
  send(data: unknown, type = 'snapshot') { this.listeners.get(type)?.({ data: JSON.stringify(data) } as MessageEvent); }
}

const frame = (sampledAt = new Date(Date.now()).toISOString(), ageMs = 0): MetricSample => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt, ageMs, utilizationPercent: 40 },
  memory: { status: 'AVAILABLE', sampledAt, ageMs, usedBytes: 400, totalBytes: 1000 },
});

const gpuFrame = (sampledAt: string, ageMs = 0) => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
  devices: [{ index: 0,
    utilization: { status: 'AVAILABLE', sampledAt, ageMs, percent: 40 },
    memory: { status: 'AVAILABLE', sampledAt, ageMs, usedBytes: 400, totalBytes: 1000 },
    temperature: { status: 'AVAILABLE', sampledAt, ageMs, celsius: 50 },
    power: { status: 'AVAILABLE', sampledAt, ageMs, watts: 100 },
  }],
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

  it('subscribes to named GPU events on the same source and expires sensors independently', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send(gpuFrame('2026-01-01T00:00:00Z'), 'gpus'));
    expect(result.current.gpuFreshness[0]?.utilization).toBe(true);
    act(() => source.send(frame(), 'snapshot'));
    expect(FakeEventSource.instances).toHaveLength(1);
    act(() => { vi.advanceTimersByTime(STALE_AFTER_MS); });
    expect(result.current.gpuFreshness[0]?.utilization).toBe(false);
  });

  it('rejects GPU data whose mode conflicts with the CPU snapshot', () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send(frame(), 'snapshot'));
    act(() => source.send({ ...gpuFrame('2026-01-01T00:00:00Z'), mode: 'LIVE' }, 'gpus'));
    expect(result.current.gpu).toBeNull();
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

  it('adds a history gap when the initial snapshot is already stale', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    act(() => FakeEventSource.instances[0]!.send(frame('2026-01-01T00:00:00Z', STALE_AFTER_MS)));
    expect(result.current.cpuFresh).toBe(false);
    expect(result.current.cpuHistory.at(-1)?.value).toBeNull();
    expect(result.current.memoryHistory.at(-1)?.value).toBeNull();
  });

  it('keeps stale reconnect snapshots as chart gaps', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.onerror?.());
    act(() => source.send(frame('2026-01-01T00:00:00Z', STALE_AFTER_MS + 1)));
    expect(result.current.connection).toBe('CONNECTED');
    expect(result.current.cpuFresh).toBe(false);
    expect(result.current.cpuHistory.at(-1)?.value).toBeNull();
    expect(result.current.memoryHistory.at(-1)?.value).toBeNull();
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

  it('updates cached age without adding another chart sample', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    const metricTime = '2026-01-01T00:00:00Z';
    act(() => source.send(frame(metricTime, 1_000)));
    const cpuLength = result.current.cpuHistory.length;
    const memoryLength = result.current.memoryHistory.length;
    act(() => vi.advanceTimersByTime(1_000));
    const cached = frame('2026-01-01T00:00:02Z', 5_500);
    cached.cpu.sampledAt = metricTime;
    cached.memory.sampledAt = metricTime;
    act(() => source.send(cached));
    expect(result.current.cpuHistory).toHaveLength(cpuLength);
    expect(result.current.memoryHistory).toHaveLength(memoryLength);
    expect(result.current.cpuFresh).toBe(true);
    act(() => vi.advanceTimersByTime(500));
    expect(result.current.cpuFresh).toBe(false);
  });

  it('caps each metric history at 450 distinct samples', () => {
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    const start = Date.parse('2026-01-01T00:00:00Z');
    act(() => {
      for (let index = 0; index < 451; index += 1) {
        source.send(frame(new Date(start + index * 2_000).toISOString()));
      }
    });
    expect(result.current.cpuHistory).toHaveLength(450);
    expect(result.current.memoryHistory).toHaveLength(450);
    expect(result.current.cpuHistory[0]?.at).toBe(new Date(start + 2_000).toISOString());
  });

  it('records unavailable and stale periods as gaps, never healthy zeroes', () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeEventSource);
    const { result } = renderHook(() => useMetrics());
    const source = FakeEventSource.instances[0]!;
    act(() => source.send(frame('2026-01-01T00:00:00Z')));
    const unavailable = frame('2026-01-01T00:00:02Z');
    unavailable.memory = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null };
    act(() => source.send(unavailable));
    expect(result.current.sample?.memory.usedBytes).toBeNull();
    expect(result.current.memoryHistory.at(-1)?.value).toBeNull();
    act(() => vi.advanceTimersByTime(STALE_AFTER_MS));
    expect(result.current.cpuHistory.at(-1)?.value).toBeNull();
    expect(result.current.cpuHistory.some(point => point.value === 0)).toBe(false);
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
