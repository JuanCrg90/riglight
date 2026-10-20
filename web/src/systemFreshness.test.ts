import { describe, expect, it } from 'vitest';
import { parseSystemOverview } from './system';
import { systemFreshness, type SystemExpiryMap } from './systemFreshness';

const overview = (sampledAt: string, ageMs = 0, mode = 'DEMO', status = 'AVAILABLE') => parseSystemOverview(JSON.stringify({
  schemaVersion: 1, sampledAt, mode,
  swap: { status, sampledAt: status === 'AVAILABLE' ? sampledAt : null, ageMs: status === 'AVAILABLE' ? ageMs : null, usedBytes: status === 'AVAILABLE' ? 0 : null, totalBytes: status === 'AVAILABLE' ? 0 : null },
  disk: { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, usedBytes: null, availableBytes: null, totalBytes: null },
  network: { status: 'NOT_READY', sampledAt: null, ageMs: null, rxBytesPerSecond: null, txBytesPerSecond: null },
}));

describe('per-sensor system freshness', () => {
  it('does not renew repeated or A-B-A timestamps, including after failure', () => {
    const expiries: SystemExpiryMap = { swap: new Map(), disk: new Map(), network: new Map() };
    const a = overview('2026-01-01T00:00:00Z')!;
    const b = overview('2026-01-01T00:00:01Z')!;
    expect(systemFreshness(a, null, expiries, 0).swap.fresh).toBe(true);
    systemFreshness(b, a, expiries, 500);
    systemFreshness(overview('2026-01-01T00:00:00Z', 0, 'DEMO', 'UNAVAILABLE')!, b, expiries, 1000);
    expect(systemFreshness(a, b, expiries, 6000).swap.fresh).toBe(false);
  });

  it('expires sensor timestamps independently and remembers shortened expiry', () => {
    const expiries: SystemExpiryMap = { swap: new Map(), disk: new Map(), network: new Map() };
    const a = overview('2026-01-01T00:00:00Z', 0)!;
    const shortened = overview('2026-01-01T00:00:00Z', 5000)!;
    expect(systemFreshness(a, null, expiries, 0).swap.expiry).toBe(6000);
    expect(systemFreshness(shortened, a, expiries, 1000).swap.expiry).toBe(2000);
    expect(systemFreshness(a, shortened, expiries, 2000).swap.fresh).toBe(false);
  });

  it('expires each sensor from its own timestamp even while other events continue', () => {
    const expiries: SystemExpiryMap = { swap: new Map(), disk: new Map(), network: new Map() };
    const first = overview('2026-01-01T00:00:00Z')!;
    first.disk = { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:01Z', ageMs: 0, usedBytes: 1, availableBytes: 1, totalBytes: 3 };
    first.network = { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:02Z', ageMs: 0, rxBytesPerSecond: 0, txBytesPerSecond: 0 };
    const firstState = systemFreshness(first, null, expiries, 0);
    expect(firstState.swap.expiry).toBe(6000);
    expect(firstState.disk.expiry).toBe(6000);
    expect(firstState.network.expiry).toBe(6000);
    const repeated = overview('2026-01-01T00:00:03Z')!;
    repeated.swap.sampledAt = first.swap.sampledAt;
    repeated.disk = { ...first.disk };
    repeated.network = { ...first.network };
    expect(systemFreshness(repeated, first, expiries, 6000)).toMatchObject({
      swap: { fresh: false }, disk: { fresh: false }, network: { fresh: false },
    });
  });

  it('clears retained identities only on mode changes and bounds each sensor to 32', () => {
    const expiries: SystemExpiryMap = { swap: new Map(), disk: new Map(), network: new Map() };
    let previous = overview('2026-01-01T00:00:00Z')!;
    for (let i = 0; i < 35; i++) {
      const next = overview(`2026-01-01T00:00:${String(i).padStart(2, '0')}Z`)!;
      systemFreshness(next, previous, expiries, i);
      previous = next;
    }
    expect(expiries.swap.size).toBe(32);
    const changed = overview('2025-01-01T00:00:00Z', 0, 'LIVE')!;
    systemFreshness(changed, previous, expiries, 100);
    expect(expiries.swap.size).toBe(1);
  });
});
