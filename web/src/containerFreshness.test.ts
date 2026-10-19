import { describe, expect, it } from 'vitest';
import { containerFreshness, expireContainerFreshness, type ContainerExpiry } from './containerFreshness';
import type { ContainerOverview } from './containers';
const at = '2026-01-01T00:00:00Z';
const available = (timestamp = at, age = 0, mode: ContainerOverview['mode'] = 'DEMO'): ContainerOverview => ({ schemaVersion: 1, sampledAt: timestamp, mode, status: 'AVAILABLE', reasonCode: null, ageMs: age, containers: [{ alias: 'service', metadata: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: age, state: 'RUNNING', health: 'HEALTHY', restartCount: 0 }, cpu: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: age, percent: 150 }, memory: { status: 'AVAILABLE', sampledAt: timestamp, ageMs: age, usedBytes: 10, totalBytes: 20 } }] });
describe('container freshness', () => {
  it('expires overview, metadata and sensors independently at fifteen seconds', () => {
    const expiry: ContainerExpiry = new Map(); const first = available();
    const fresh = containerFreshness(first, null, expiry, 100);
    expect(fresh.overview).toBe(true); expect(fresh.rows.service).toEqual({ metadata: true, cpu: true, memory: true });
    expect(containerFreshness(first, first, expiry, 15_099).rows.service?.cpu).toBe(true);
    expect(containerFreshness(first, first, expiry, 15_100).overview).toBe(false);
  });
  it('does not renew replay identities across A-B-A or nonavailable frames', () => {
    const expiry: ContainerExpiry = new Map(); const a = available(at); containerFreshness(a, null, expiry, 0);
    const b = available('2026-01-01T00:00:01Z'); containerFreshness(b, a, expiry, 5_000);
    const down = { ...b, status: 'UNAVAILABLE' as const, reasonCode: 'QUERY_FAILED' as const, ageMs: null, containers: [] };
    containerFreshness(down, b, expiry, 8_000);
    expect(containerFreshness(a, down, expiry, 14_999).overview).toBe(true);
    expect(containerFreshness(a, down, expiry, 15_001).overview).toBe(false);
  });
  it('keeps a 32-identity replay bank for each overview and alias field at the 16-row cap', () => {
    const expiry: ContainerExpiry = new Map();
    const rows = Array.from({ length: 16 }, (_, index) => ({ ...available().containers[0]!, alias: `service-${String(index).padStart(2, '0')}` }));
    const frame = { ...available(), containers: rows };
    const freshness = containerFreshness(frame, null, expiry, 0);
    expect(expiry.size).toBe(49);
    expect(Object.values(freshness.rows).every(row => row.metadata && row.cpu && row.memory)).toBe(true);
    const stale = expireContainerFreshness(freshness, expiry, 15_000);
    expect(stale.overview).toBe(false);
    expect(Object.values(stale.rows).every(row => !row.metadata && !row.cpu && !row.memory)).toBe(true);
    expect(expiry.size).toBe(49);
  });
  it('stores shortened expiries so lower-age replays cannot resurrect overview or fields', () => {
    const expiry: ContainerExpiry = new Map();
    const first = available();
    containerFreshness(first, null, expiry, 0);
    const shorter = available(at, 15_000);
    expect(containerFreshness(shorter, first, expiry, 1_000).overview).toBe(false);
    const replay = available(at, 0);
    expect(containerFreshness(replay, shorter, expiry, 2_000).overview).toBe(false);
    expect(containerFreshness(replay, replay, expiry, 2_000).rows.service?.cpu).toBe(false);
  });
  it('preserves alias histories through unavailable overviews and prunes removed aliases on available selections', () => {
    const expiry: ContainerExpiry = new Map(); const first = available();
    containerFreshness(first, null, expiry, 0);
    const unavailable = { ...first, status: 'UNAVAILABLE' as const, reasonCode: 'QUERY_FAILED' as const, ageMs: null, containers: [] };
    containerFreshness(unavailable, first, expiry, 1_000);
    expect(expiry.size).toBe(4);
    const selectedAgain = { ...available('2026-01-01T00:00:01Z'), containers: [{ ...available().containers[0]!, alias: 'another' }] };
    containerFreshness(selectedAgain, unavailable, expiry, 2_000);
    expect([...expiry.keys()].some(key => key.includes('service'))).toBe(false);
    expect(containerFreshness(first, selectedAgain, expiry, 3_000).rows.service?.cpu).toBe(true);
  });
  it('accepts an unseen earlier server timestamp after rollback and resets identities on mode change', () => {
    const expiry: ContainerExpiry = new Map(); const later = available('2026-01-02T00:00:00Z');
    containerFreshness(later, null, expiry, 0);
    expect(containerFreshness(available('2026-01-01T00:00:00Z'), later, expiry, 1).overview).toBe(true);
    expect(containerFreshness(available(at, 0, 'LIVE'), later, expiry, 2).overview).toBe(true);
    expect(expiry.size).toBe(4);
  });
});
