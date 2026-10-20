import type { SystemOverview } from './system';

const IDENTITY_LIMIT = 32;
const STALE_AFTER_MS = 6_000;
export type SystemSensorName = 'swap' | 'disk' | 'network';
export type SystemExpiryMap = Record<SystemSensorName, Map<string, number>>;
export type SystemFreshness = Record<SystemSensorName, { fresh: boolean; expiry: number }>;

function remember(expiries: Map<string, number>, identity: string, expiry: number): void {
  expiries.delete(identity);
  expiries.set(identity, expiry);
  while (expiries.size > IDENTITY_LIMIT) expiries.delete(expiries.keys().next().value!);
}

export function systemFreshness(current: SystemOverview, previous: SystemOverview | null, expiries: SystemExpiryMap, now: number): SystemFreshness {
  if (previous && previous.mode !== current.mode) {
    expiries.swap.clear();
    expiries.disk.clear();
    expiries.network.clear();
  }
  const result = {} as SystemFreshness;
  for (const name of ['swap', 'disk', 'network'] as const) {
    const sensor = current[name];
    if (sensor.status !== 'AVAILABLE') {
      result[name] = { fresh: false, expiry: Number.NEGATIVE_INFINITY };
      continue;
    }
    const identity = JSON.stringify([current.mode, sensor.sampledAt]);
    const known = expiries[name].get(identity);
    const reported = now + Math.max(0, STALE_AFTER_MS - sensor.ageMs!);
    const expiry = known === undefined ? reported : Math.min(known, reported);
    remember(expiries[name], identity, expiry);
    result[name] = { fresh: now < expiry, expiry };
  }
  return result;
}

export function expireSystemFreshness(freshness: SystemFreshness, now: number): SystemFreshness {
  let changed = false;
  const next = { ...freshness };
  for (const name of ['swap', 'disk', 'network'] as const) {
    if (next[name].fresh && now >= next[name].expiry) {
      next[name] = { fresh: false, expiry: next[name].expiry };
      changed = true;
    }
  }
  return changed ? next : freshness;
}
