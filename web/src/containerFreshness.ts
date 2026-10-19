import type { ContainerOverview, ContainerRow } from './containers';

const IDENTITY_LIMIT = 32;
const STALE_AFTER_MS = 15_000;
export type ContainerExpiry = Map<string, number>;
export type RowFreshness = { metadata: boolean; cpu: boolean; memory: boolean };
export type ContainerFreshness = { overview: boolean; rows: Record<string, RowFreshness> };

function remember(expiries: ContainerExpiry, identity: string, expiry: number): void {
  if (!expiries.has(identity)) expiries.set(identity, expiry);
  while (expiries.size > IDENTITY_LIMIT) expiries.delete(expiries.keys().next().value!);
}
function sensorExpiry(expiries: ContainerExpiry, mode: string, alias: string, sensor: string, sampledAt: string, ageMs: number, now: number): number {
  const identity = JSON.stringify([mode, alias, sensor, sampledAt]);
  const calculated = now + Math.max(0, STALE_AFTER_MS - ageMs);
  const known = expiries.get(identity);
  const expiry = known === undefined ? calculated : Math.min(known, calculated);
  remember(expiries, identity, expiry);
  return expiry;
}

export function containerFreshness(current: ContainerOverview, previous: ContainerOverview | null, expiries: ContainerExpiry, now: number): ContainerFreshness {
  if (previous && previous.mode !== current.mode) expiries.clear();
  const overviewIdentity = JSON.stringify([current.mode, current.sampledAt]);
  const knownOverview = expiries.get(overviewIdentity);
  const available = current.status === 'AVAILABLE';
  const calculatedOverview = available ? now + Math.max(0, STALE_AFTER_MS - current.ageMs!) : Number.NEGATIVE_INFINITY;
  const overviewExpiry = knownOverview === undefined ? calculatedOverview : Math.min(knownOverview, calculatedOverview);
  remember(expiries, overviewIdentity, overviewExpiry);
  const overview = available && now < overviewExpiry;
  const rows: Record<string, RowFreshness> = {};
  if (available) for (const row of current.containers) {
    rows[row.alias] = {
      metadata: fresh(expiries, current.mode, row.alias, 'metadata', row.metadata, now),
      cpu: fresh(expiries, current.mode, row.alias, 'cpu', row.cpu, now),
      memory: fresh(expiries, current.mode, row.alias, 'memory', row.memory, now),
    };
  }
  return { overview, rows };
}
function fresh(expiries: ContainerExpiry, mode: string, alias: string, key: string, sample: ContainerRow['metadata'] | ContainerRow['cpu'] | ContainerRow['memory'], now: number): boolean {
  if (sample.status !== 'AVAILABLE' || sample.sampledAt === null || sample.ageMs === null) {
    if (sample.sampledAt !== null) remember(expiries, JSON.stringify([mode, alias, key, sample.sampledAt]), Number.NEGATIVE_INFINITY);
    return false;
  }
  return now < sensorExpiry(expiries, mode, alias, key, sample.sampledAt, sample.ageMs, now);
}
