import type { GpuDevice, GpuMetricBase, GpuMetricStatus, GpuOverview } from './gpu';

export const GPU_SENSORS = ['utilization', 'memory', 'temperature', 'power'] as const;
export type GpuSensor = typeof GPU_SENSORS[number];
export type GpuFreshness = Record<number, Record<GpuSensor, boolean>>;
type Expiries = Map<string, { sampledAt: string; expiresAt: number }>;

export function gpuMetricFreshness(overview: GpuOverview, previous: GpuOverview | null, expiries: Expiries, now: number): GpuFreshness {
  const result: GpuFreshness = {};
  const oldDevices = new Map(previous?.devices.map(device => [device.index, device]) ?? []);
  const currentIndices = new Set(overview.devices.map(device => device.index));
  for (const key of expiries.keys()) if (!currentIndices.has(Number(key.split(':')[0]))) expiries.delete(key);
  for (const device of overview.devices) {
    const previousDevice = oldDevices.get(device.index);
    result[device.index] = Object.fromEntries(GPU_SENSORS.map(sensor => {
      const metric = device[sensor] as GpuMetricBase;
      const oldMetric = previousDevice?.[sensor] as GpuMetricBase | undefined;
      const key = `${device.index}:${sensor}`;
      if (metric.status !== 'AVAILABLE') { expiries.delete(key); return [sensor, false]; }
      const reportedExpiry = now + Math.max(0, 6_000 - metric.ageMs!);
      const expiry = oldMetric?.status === 'AVAILABLE' && oldMetric.sampledAt === metric.sampledAt
        ? Math.min(expiries.get(key)?.expiresAt ?? reportedExpiry, reportedExpiry)
        : reportedExpiry;
      expiries.set(key, { sampledAt: metric.sampledAt!, expiresAt: expiry });
      return [sensor, now < expiry];
    })) as Record<GpuSensor, boolean>;
  }
  return result;
}

export function expireGpuFreshness(freshness: GpuFreshness, expiries: Expiries, now: number): GpuFreshness {
  let changed = false;
  const next = { ...freshness };
  for (const [indexText, values] of Object.entries(freshness)) {
    const index = Number(indexText);
    const updated = { ...values };
    for (const sensor of GPU_SENSORS) {
      if (updated[sensor] && now >= (expiries.get(`${index}:${sensor}`)?.expiresAt ?? Number.NEGATIVE_INFINITY)) {
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
