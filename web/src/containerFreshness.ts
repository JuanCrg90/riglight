import type { ContainerOverview, ContainerRow } from './containers';

const IDENTITY_LIMIT = 32;
const STALE_AFTER_MS = 15_000;
type ExpiryBank = { identities: Map<string, number>; activeExpiry: number };
export type ContainerExpiry = Map<string, ExpiryBank>;
export type RowFreshness = { metadata: boolean; cpu: boolean; memory: boolean };
export type ContainerFreshness = { overview: boolean; rows: Record<string, RowFreshness> };

function bankFor(expiries: ContainerExpiry, key: string): ExpiryBank {
  let bank = expiries.get(key);
  if (!bank) {
    bank = { identities: new Map(), activeExpiry: Number.NEGATIVE_INFINITY };
    expiries.set(key, bank);
  }
  return bank;
}
function update(bank: ExpiryBank, identity: string, ageMs: number, now: number): boolean {
  const reported = now + Math.max(0, STALE_AFTER_MS - ageMs);
  const known = bank.identities.get(identity);
  const expiry = known === undefined ? reported : Math.min(known, reported);
  bank.identities.delete(identity);
  bank.identities.set(identity, expiry);
  while (bank.identities.size > IDENTITY_LIMIT) bank.identities.delete(bank.identities.keys().next().value!);
  bank.activeExpiry = expiry;
  return now < expiry;
}
function updateSensor(expiries: ContainerExpiry, mode: string, alias: string, sensor: string, sample: ContainerRow['metadata'] | ContainerRow['cpu'] | ContainerRow['memory'], now: number): boolean {
  if (sample.status !== 'AVAILABLE' || sample.sampledAt === null || sample.ageMs === null) return false;
  const bank = bankFor(expiries, JSON.stringify([alias, sensor]));
  return update(bank, JSON.stringify([mode, sample.sampledAt]), sample.ageMs, now);
}

export function containerFreshness(current: ContainerOverview, previous: ContainerOverview | null, expiries: ContainerExpiry, now: number): ContainerFreshness {
  if (previous && previous.mode !== current.mode) expiries.clear();
  if (current.status !== 'AVAILABLE') return { overview: false, rows: {} };

  const aliases = new Set(current.containers.map(row => row.alias));
  for (const key of expiries.keys()) {
    if (key !== 'overview') {
      const alias = JSON.parse(key)[0] as string;
      if (!aliases.has(alias)) expiries.delete(key);
    }
  }
  const overview = update(bankFor(expiries, 'overview'), JSON.stringify([current.mode, current.sampledAt]), current.ageMs!, now);
  const rows: Record<string, RowFreshness> = {};
  for (const row of current.containers) {
    rows[row.alias] = {
      metadata: updateSensor(expiries, current.mode, row.alias, 'metadata', row.metadata, now),
      cpu: updateSensor(expiries, current.mode, row.alias, 'cpu', row.cpu, now),
      memory: updateSensor(expiries, current.mode, row.alias, 'memory', row.memory, now),
    };
  }
  return { overview, rows };
}

export function expireContainerFreshness(freshness: ContainerFreshness, expiries: ContainerExpiry, now: number): ContainerFreshness {
  let overview = freshness.overview;
  let changed = overview && now >= (expiries.get('overview')?.activeExpiry ?? Number.NEGATIVE_INFINITY);
  if (changed) overview = false;
  let rows = freshness.rows;
  for (const [alias, current] of Object.entries(freshness.rows)) {
    const next = { ...current };
    for (const sensor of ['metadata', 'cpu', 'memory'] as const) {
      const key = JSON.stringify([alias, sensor]);
      if (next[sensor] && now >= (expiries.get(key)?.activeExpiry ?? Number.NEGATIVE_INFINITY)) {
        next[sensor] = false;
        changed = true;
      }
    }
    if (next.metadata !== current.metadata || next.cpu !== current.cpu || next.memory !== current.memory) {
      if (rows === freshness.rows) rows = { ...rows };
      rows[alias] = next;
    }
  }
  return changed ? { overview, rows } : freshness;
}
