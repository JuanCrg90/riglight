import { useEffect, useState } from 'react';
import { parseMetricSample, type MetricSample } from './metrics';

export const STALE_AFTER_MS = 6_000;
export const HISTORY_LIMIT = 450;
export type Point = { at: string; value: number | null };
type View = {
  sample: MetricSample | null;
  connection: 'CONNECTING' | 'CONNECTED' | 'RECONNECTING';
  cpuFresh: boolean;
  memoryFresh: boolean;
  cpuHistory: Point[];
  memoryHistory: Point[];
};

const initial: View = { sample: null, connection: 'CONNECTING', cpuFresh: false, memoryFresh: false, cpuHistory: [], memoryHistory: [] };

export function useMetrics(): View {
  const [view, setView] = useState(initial);
  useEffect(() => {
    const source = new EventSource('/api/v1/events');
    let current: MetricSample | null = null;
    const recentFrames = new Set<string>();
    let cpuExpiry = Number.NEGATIVE_INFINITY;
    let memoryExpiry = Number.NEGATIVE_INFINITY;
    let cpuFresh = false;
    let memoryFresh = false;
    let cpuHistory: Point[] = [];
    let memoryHistory: Point[] = [];
    let connection: View['connection'] = 'CONNECTING';

    const publish = () => setView({ sample: current, connection, cpuFresh, memoryFresh, cpuHistory, memoryHistory });
    const onOpen = () => { connection = 'CONNECTED'; publish(); };
    const onError = () => { connection = 'RECONNECTING'; publish(); };
    const onSnapshot = (event: Event) => {
      const sample = parseMetricSample((event as MessageEvent<string>).data);
      if (!sample) return;
      const frameKey = JSON.stringify([
        sample.schemaVersion, sample.sampledAt, sample.mode,
        sample.cpu.status, sample.cpu.sampledAt, sample.cpu.ageMs, sample.cpu.utilizationPercent,
        sample.memory.status, sample.memory.sampledAt, sample.memory.ageMs, sample.memory.usedBytes, sample.memory.totalBytes,
      ]);
      if (recentFrames.has(frameKey)) return;
      recentFrames.add(frameKey);
      if (recentFrames.size > HISTORY_LIMIT) recentFrames.delete(recentFrames.values().next().value!);
      const monotonic = performance.now();
      // Server-computed age avoids comparing clocks on the host and browser.
      // A repeated metric timestamp may shorten, but never extend, its expiry.
      const oldSample = current;
      if (sample.cpu.status === 'AVAILABLE') {
        const reportedExpiry = monotonic + Math.max(0, STALE_AFTER_MS - sample.cpu.ageMs!);
        cpuExpiry = oldSample?.cpu.sampledAt === sample.cpu.sampledAt ? Math.min(cpuExpiry, reportedExpiry) : reportedExpiry;
        cpuFresh = monotonic < cpuExpiry;
      } else { cpuFresh = false; cpuExpiry = Number.NEGATIVE_INFINITY; }
      if (sample.memory.status === 'AVAILABLE') {
        const reportedExpiry = monotonic + Math.max(0, STALE_AFTER_MS - sample.memory.ageMs!);
        memoryExpiry = oldSample?.memory.sampledAt === sample.memory.sampledAt ? Math.min(memoryExpiry, reportedExpiry) : reportedExpiry;
        memoryFresh = monotonic < memoryExpiry;
      } else { memoryFresh = false; memoryExpiry = Number.NEGATIVE_INFINITY; }
      current = sample;
      cpuHistory = [...cpuHistory, { at: sample.sampledAt, value: sample.cpu.status === 'AVAILABLE' ? sample.cpu.utilizationPercent : null }].slice(-HISTORY_LIMIT);
      memoryHistory = [...memoryHistory, { at: sample.sampledAt, value: sample.memory.status === 'AVAILABLE' ? sample.memory.usedBytes! / sample.memory.totalBytes! * 100 : null }].slice(-HISTORY_LIMIT);
      connection = 'CONNECTED';
      publish();
    };
    source.addEventListener('snapshot', onSnapshot);
    source.onopen = onOpen;
    source.onerror = onError;
    const timer = window.setInterval(() => {
      const now = performance.now();
      const nextCPU = cpuFresh && now < cpuExpiry;
      const nextMemory = memoryFresh && now < memoryExpiry;
      if (nextCPU !== cpuFresh || nextMemory !== memoryFresh) {
        const gapAt = new Date().toISOString();
        if (cpuFresh && !nextCPU) cpuHistory = [...cpuHistory, { at: gapAt, value: null }].slice(-HISTORY_LIMIT);
        if (memoryFresh && !nextMemory) memoryHistory = [...memoryHistory, { at: gapAt, value: null }].slice(-HISTORY_LIMIT);
        cpuFresh = nextCPU;
        memoryFresh = nextMemory;
        publish();
      }
    }, 100);
    return () => {
      window.clearInterval(timer);
      source.removeEventListener('snapshot', onSnapshot);
      source.close();
    };
  }, []);
  return view;
}
