import { useEffect, useState } from 'react';
import { parseMetricSample, type MetricSample } from './metrics';
import { parseGpuOverview, type GpuOverview } from './gpu';
import { expireGpuFreshness, gpuMetricFreshness, type GpuFreshness } from './gpuFreshness';

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
  gpu: GpuOverview | null;
  gpuFreshness: GpuFreshness;
};

const initial: View = { sample: null, connection: 'CONNECTING', cpuFresh: false, memoryFresh: false, cpuHistory: [], memoryHistory: [], gpu: null, gpuFreshness: {} };

function appendPoint(history: Point[], identities: string[], identity: string, point: Point): { history: Point[]; identities: string[] } {
  if (identities.includes(identity)) return { history, identities };
  return {
    history: [...history, point].slice(-HISTORY_LIMIT),
    identities: [...identities, identity].slice(-HISTORY_LIMIT),
  };
}

export function useMetrics(): View {
  const [view, setView] = useState(initial);
  useEffect(() => {
    const source = new EventSource('/api/v1/events');
    let current: MetricSample | null = null;
    const recentFrames = new Set<string>();
    const gpuExpiries = new Map<string, { sampledAt: string; expiresAt: number }>();
    let gpu: GpuOverview | null = null;
    let gpuFreshness: GpuFreshness = {};
    let cpuExpiry = Number.NEGATIVE_INFINITY;
    let memoryExpiry = Number.NEGATIVE_INFINITY;
    let cpuFresh = false;
    let memoryFresh = false;
    let cpuHistory: Point[] = [];
    let memoryHistory: Point[] = [];
    let cpuHistoryIdentities: string[] = [];
    let memoryHistoryIdentities: string[] = [];
    let connection: View['connection'] = 'CONNECTING';

    const publish = () => setView({ sample: current, connection, cpuFresh, memoryFresh, cpuHistory, memoryHistory, gpu, gpuFreshness });
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
      if (gpu && gpu.mode !== 'DISABLED' && gpu.mode !== sample.mode) {
        gpu = null;
        gpuFreshness = {};
        gpuExpiries.clear();
      }
      const cpuIdentity = sample.cpu.status === 'AVAILABLE'
        ? JSON.stringify([sample.cpu.status, sample.cpu.sampledAt, sample.cpu.utilizationPercent])
        : JSON.stringify([sample.cpu.status, sample.sampledAt]);
      const cpuPoint = appendPoint(cpuHistory, cpuHistoryIdentities,
        sample.cpu.status === 'AVAILABLE' && !cpuFresh ? `stale:${cpuIdentity}` : cpuIdentity,
        { at: sample.sampledAt, value: sample.cpu.status === 'AVAILABLE' && cpuFresh ? sample.cpu.utilizationPercent : null });
      cpuHistory = cpuPoint.history;
      cpuHistoryIdentities = cpuPoint.identities;
      const memoryIdentity = sample.memory.status === 'AVAILABLE'
        ? JSON.stringify([sample.memory.status, sample.memory.sampledAt, sample.memory.usedBytes, sample.memory.totalBytes])
        : JSON.stringify([sample.memory.status, sample.sampledAt]);
      const memoryPoint = appendPoint(memoryHistory, memoryHistoryIdentities,
        sample.memory.status === 'AVAILABLE' && !memoryFresh ? `stale:${memoryIdentity}` : memoryIdentity,
        { at: sample.sampledAt, value: sample.memory.status === 'AVAILABLE' && memoryFresh ? sample.memory.usedBytes! / sample.memory.totalBytes! * 100 : null });
      memoryHistory = memoryPoint.history;
      memoryHistoryIdentities = memoryPoint.identities;
      connection = 'CONNECTED';
      publish();
    };
    const onGpus = (event: Event) => {
      const overview = parseGpuOverview((event as MessageEvent<string>).data);
      if (!overview || (current && overview.mode !== 'DISABLED' && overview.mode !== current.mode)) return;
      const monotonic = performance.now();
      gpuFreshness = gpuMetricFreshness(overview, gpu, gpuExpiries, monotonic);
      gpu = overview;
      connection = 'CONNECTED';
      publish();
    };
    source.addEventListener('snapshot', onSnapshot);
    source.addEventListener('gpus', onGpus);
    source.onopen = onOpen;
    source.onerror = onError;
    const timer = window.setInterval(() => {
      const now = performance.now();
      const nextGpuFreshness = expireGpuFreshness(gpuFreshness, gpuExpiries, now);
      const gpuChanged = nextGpuFreshness !== gpuFreshness;
      gpuFreshness = nextGpuFreshness;
      const nextCPU = cpuFresh && now < cpuExpiry;
      const nextMemory = memoryFresh && now < memoryExpiry;
      if (nextCPU !== cpuFresh || nextMemory !== memoryFresh || gpuChanged) {
        const gapAt = new Date().toISOString();
        if (cpuFresh && !nextCPU) {
          const point = appendPoint(cpuHistory, cpuHistoryIdentities, `stale:${now}`, { at: gapAt, value: null });
          cpuHistory = point.history;
          cpuHistoryIdentities = point.identities;
        }
        if (memoryFresh && !nextMemory) {
          const point = appendPoint(memoryHistory, memoryHistoryIdentities, `stale:${now}`, { at: gapAt, value: null });
          memoryHistory = point.history;
          memoryHistoryIdentities = point.identities;
        }
        cpuFresh = nextCPU;
        memoryFresh = nextMemory;
        publish();
      }
    }, 100);
    return () => {
      window.clearInterval(timer);
      source.removeEventListener('snapshot', onSnapshot);
      source.removeEventListener('gpus', onGpus);
      source.close();
    };
  }, []);
  return view;
}
