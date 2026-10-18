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
    let lastEnvelope = '';
    let cpuArrival = Number.NEGATIVE_INFINITY;
    let memoryArrival = Number.NEGATIVE_INFINITY;
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
      if (!sample || sample.sampledAt === lastEnvelope || (lastEnvelope && Date.parse(sample.sampledAt) <= Date.parse(lastEnvelope))) return;
      const now = Date.now();
      const monotonic = performance.now();
      // A metric is fresh only if its own timestamp advances; envelope updates cannot refresh it.
      const oldSample = current;
      if (sample.cpu.status === 'AVAILABLE' && (!oldSample?.cpu.sampledAt || Date.parse(sample.cpu.sampledAt!) > Date.parse(oldSample.cpu.sampledAt))) {
        const age = now - Date.parse(sample.cpu.sampledAt!);
        cpuFresh = age >= 0 && age < STALE_AFTER_MS;
        cpuArrival = cpuFresh ? monotonic - age : Number.NEGATIVE_INFINITY;
      } else if (sample.cpu.status !== 'AVAILABLE') { cpuFresh = false; cpuArrival = Number.NEGATIVE_INFINITY; }
      if (sample.memory.status === 'AVAILABLE' && (!oldSample?.memory.sampledAt || Date.parse(sample.memory.sampledAt!) > Date.parse(oldSample.memory.sampledAt))) {
        const age = now - Date.parse(sample.memory.sampledAt!);
        memoryFresh = age >= 0 && age < STALE_AFTER_MS;
        memoryArrival = memoryFresh ? monotonic - age : Number.NEGATIVE_INFINITY;
      } else if (sample.memory.status !== 'AVAILABLE') { memoryFresh = false; memoryArrival = Number.NEGATIVE_INFINITY; }
      current = sample;
      lastEnvelope = sample.sampledAt;
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
      const nextCPU = cpuFresh && now - cpuArrival < STALE_AFTER_MS;
      const nextMemory = memoryFresh && now - memoryArrival < STALE_AFTER_MS;
      if (nextCPU !== cpuFresh || nextMemory !== memoryFresh) {
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
