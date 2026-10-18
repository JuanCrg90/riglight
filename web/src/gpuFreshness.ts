import type { GpuDevice, GpuMetricBase, GpuMetricStatus, GpuOverview } from './gpu';

export const GPU_SENSORS = ['utilization', 'memory', 'temperature', 'power'] as const;
export type GpuSensor = typeof GPU_SENSORS[number];
export type GpuFreshness = Record<number, Record<GpuSensor, boolean>>;
export type GpuExpiryState = { identities: Map<string, number>; activeExpiry: number };
export type GpuExpiryMap = Map<string, GpuExpiryState>;
const IDENTITY_LIMIT = 32;
const STALE_AFTER_MS = 6_000;

export function gpuMetricFreshness(overview: GpuOverview, previous: GpuOverview | null, expiries: GpuExpiryMap, now: number): GpuFreshness {
  if (previous && previous.mode !== overview.mode) expiries.clear();
  const result: GpuFreshness = {};
  const currentIndices = new Set(overview.devices.map(device => device.index));
  if (overview.status !== 'UNAVAILABLE') {
    for (const key of expiries.keys()) {
      const deviceIndex = Number(key.slice(0, key.indexOf(':')));
      if (!currentIndices.has(deviceIndex)) expiries.delete(key);
    }
  }
  for (const device of overview.devices) {
    result[device.index] = Object.fromEntries(GPU_SENSORS.map(sensor => {
      const metric = device[sensor] as GpuMetricBase & Record<string, unknown>;
      const key = `${device.index}:${sensor}`;
      if (metric.status !== 'AVAILABLE') return [sensor, false];
      let state = expiries.get(key);
      if (!state) {
        state = { identities: new Map(), activeExpiry: Number.NEGATIVE_INFINITY };
        expiries.set(key, state);
      }
      const timestamp = metric.sampledAt!;
      const reportedExpiry = now + Math.max(0, STALE_AFTER_MS - metric.ageMs!);
      const knownExpiry = state.identities.get(timestamp);
      const expiry = knownExpiry === undefined ? reportedExpiry : Math.min(knownExpiry, reportedExpiry);
      state.identities.delete(timestamp);
      state.identities.set(timestamp, expiry);
      while (state.identities.size > IDENTITY_LIMIT) state.identities.delete(state.identities.keys().next().value!);
      state.activeExpiry = expiry;
      return [sensor, now < expiry];
    })) as Record<GpuSensor, boolean>;
  }
  return result;
}

export function expireGpuFreshness(freshness: GpuFreshness, expiries: GpuExpiryMap, now: number): GpuFreshness {
  let changed = false;
  const next = { ...freshness };
  for (const [indexText, values] of Object.entries(freshness)) {
    const index = Number(indexText);
    const updated = { ...values };
    for (const sensor of GPU_SENSORS) {
      if (updated[sensor] && now >= (expiries.get(`${index}:${sensor}`)?.activeExpiry ?? Number.NEGATIVE_INFINITY)) {
        updated[sensor] = false;
        changed = true;
      }
    }
    if (changed) next[index] = updated;
  }
  return changed ? next : freshness;
}

export function sensorStatus(device: GpuDevice, sensor: GpuSensor): GpuMetricStatus {
  return device[sensor].status;
}
