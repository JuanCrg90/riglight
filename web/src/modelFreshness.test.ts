import { describe, expect, it } from 'vitest';
import { modelInventoryFreshness, type ModelExpiry } from './modelFreshness';
import type { ModelOverview } from './models';

const snapshot = (sampledAt: string, ageMs = 0, mode: 'DEMO' | 'LIVE' = 'DEMO', id = 'example-chat'): ModelOverview => ({
  schemaVersion: 1, sampledAt, mode, status: 'AVAILABLE', reasonCode: null, ageMs, isTruncated: false,
  models: [{ id, state: 'LOADED' }],
});

describe('model inventory freshness', () => {
  it('expires at six seconds and never renews a timestamp identity', () => {
    const expiries: ModelExpiry = new Map(); const first = snapshot('2026-01-01T00:00:00Z');
    expect(modelInventoryFreshness(first, null, expiries, 100).fresh).toBe(true);
    expect(modelInventoryFreshness(first, first, expiries, 5_900).fresh).toBe(true);
    expect(modelInventoryFreshness(first, first, expiries, 6_100).fresh).toBe(false);
  });

  it('does not renew an identity when content changes at the same timestamp', () => {
    const expiries: ModelExpiry = new Map(); const first = snapshot('2026-01-01T00:00:00Z');
    modelInventoryFreshness(first, null, expiries, 0);
    const changed = snapshot(first.sampledAt, 0, 'DEMO', 'different-content');
    expect(modelInventoryFreshness(changed, first, expiries, 5_500).fresh).toBe(true);
    expect(modelInventoryFreshness(changed, first, expiries, 6_000).fresh).toBe(false);
    expect(expiries.size).toBe(1);
  });

  it('preserves identities across unavailable states and A→B→A replays', () => {
    const expiries: ModelExpiry = new Map(); const a = snapshot('2026-01-01T00:00:00Z'); const b = snapshot('2026-01-01T00:00:01Z');
    modelInventoryFreshness(a, null, expiries, 0);
    const unavailable = { ...a, status: 'UNAVAILABLE' as const, reasonCode: 'QUERY_FAILED' as const, ageMs: null, models: [] };
    modelInventoryFreshness(unavailable, a, expiries, 1_000);
    modelInventoryFreshness(b, unavailable, expiries, 2_000);
    expect(modelInventoryFreshness(a, b, expiries, 5_500).fresh).toBe(true);
    expect(modelInventoryFreshness(a, b, expiries, 6_001).fresh).toBe(false);
  });

  it('accepts unseen earlier timestamps after clock rollback and clears on mode change', () => {
    const expiries: ModelExpiry = new Map(); const later = snapshot('2026-01-02T00:00:00Z');
    modelInventoryFreshness(later, null, expiries, 0);
    const earlier = snapshot('2026-01-01T00:00:00Z');
    expect(modelInventoryFreshness(earlier, later, expiries, 100).fresh).toBe(true);
    const live = snapshot(earlier.sampledAt, 0, 'LIVE');
    expect(modelInventoryFreshness(live, earlier, expiries, 200).fresh).toBe(true);
    expect(expiries.size).toBe(1);
  });

  it('bounds accepted timestamp identities to the most recent 32', () => {
    const expiries: ModelExpiry = new Map();
    for (let index = 0; index < 33; index += 1) {
      modelInventoryFreshness(snapshot(new Date(Date.UTC(2026, 0, 1, 0, 0, index)).toISOString()), null, expiries, index * 10);
    }
    expect(expiries.size).toBe(32);
  });

  it('marks aged or unavailable source snapshots stale without discarding replay history', () => {
    const expiries: ModelExpiry = new Map();
    const aged = snapshot('2026-01-01T00:00:00Z', 6_001);
    expect(modelInventoryFreshness(aged, null, expiries, 100).fresh).toBe(false);
    expect(expiries.size).toBe(1);
  });
});
