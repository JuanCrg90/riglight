import type { ModelOverview } from './models';

const IDENTITY_LIMIT = 32;
const STALE_AFTER_MS = 6_000;
export type ModelExpiry = Map<string, number>;
export type ModelFreshness = { fresh: boolean; expiry: number };

function rememberIdentity(expiries: ModelExpiry, identity: string, expiry: number): void {
  expiries.delete(identity);
  expiries.set(identity, expiry);
  while (expiries.size > IDENTITY_LIMIT) expiries.delete(expiries.keys().next().value!);
}

export function modelInventoryFreshness(current: ModelOverview, previous: ModelOverview | null, expiries: ModelExpiry, now: number): ModelFreshness {
  if (previous && previous.mode !== current.mode) expiries.clear();

  const identity = JSON.stringify([current.mode, current.sampledAt]);
  const known = expiries.get(identity);
  if (current.status !== 'AVAILABLE') {
    const expiry = known ?? Number.NEGATIVE_INFINITY;
    rememberIdentity(expiries, identity, expiry);
    return { fresh: false, expiry };
  }

  const reported = now + Math.max(0, STALE_AFTER_MS - current.ageMs!);
  const expiry = known === undefined ? reported : Math.min(known, reported);
  rememberIdentity(expiries, identity, expiry);
  return { fresh: now < expiry, expiry };
}
