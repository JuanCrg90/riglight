import type { ModelOverview } from './models';

const IDENTITY_LIMIT = 32;
const STALE_AFTER_MS = 6_000;
export type ModelExpiry = Map<string, number>;
export type ModelFreshness = { fresh: boolean; expiry: number };

export function modelInventoryFreshness(current: ModelOverview, previous: ModelOverview | null, expiries: ModelExpiry, now: number): ModelFreshness {
  if (previous && previous.mode !== current.mode) expiries.clear();
  if (current.status !== 'AVAILABLE') return { fresh: false, expiry: Number.NEGATIVE_INFINITY };

  const identity = JSON.stringify([current.mode, current.sampledAt]);
  const reported = now + Math.max(0, STALE_AFTER_MS - current.ageMs!);
  const known = expiries.get(identity);
  const expiry = known === undefined ? reported : Math.min(known, reported);
  expiries.delete(identity);
  expiries.set(identity, expiry);
  while (expiries.size > IDENTITY_LIMIT) expiries.delete(expiries.keys().next().value!);
  return { fresh: now < expiry, expiry };
}
