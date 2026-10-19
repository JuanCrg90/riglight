import { describe, expect, it } from 'vitest';
import { parseContainerOverview } from './containers';

const time = '2026-01-01T00:00:00Z';
const container = (alias = 'demo-service') => ({
  alias,
  metadata: { status: 'AVAILABLE', sampledAt: time, ageMs: 0, state: 'RUNNING', health: 'HEALTHY', restartCount: 2 },
  cpu: { status: 'AVAILABLE', sampledAt: time, ageMs: 0, percent: 125 },
  memory: { status: 'AVAILABLE', sampledAt: time, ageMs: 0, usedBytes: 256, totalBytes: 1024 },
});
const base = { schemaVersion: 1, sampledAt: time, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, ageMs: 0, containers: [container()] };
const parse = (value: unknown) => parseContainerOverview(JSON.stringify(value));

describe('parseContainerOverview', () => {
  it('accepts the strict projected shape and one-core CPU percentages above 100', () => expect(parse(base)).toEqual(base));
  it.each([
    { ...base, extra: true }, { ...base, schemaVersion: 2 }, { ...base, mode: 'LIVE', status: 'DISABLED' },
    { ...base, sampledAt: '2026-01-01T00:00:00+00:00' }, { ...base, ageMs: Number.MAX_SAFE_INTEGER + 1 },
    { ...base, containers: [container('a'), container('a')] }, { ...base, containers: [container('z'), container('a')] },
    { ...base, containers: [container('../bad')] }, { ...base, containers: Array.from({ length: 17 }, (_, i) => container(`a${i}`)) },
    { ...base, containers: [{ ...container(), metadata: { ...container().metadata, extra: 'secret' } }] },
    { ...base, containers: [{ ...container(), cpu: { ...container().cpu, percent: 409601 } }] },
    { ...base, containers: [{ ...container(), memory: { ...container().memory, usedBytes: 1025 } }] },
    { ...base, status: 'UNAVAILABLE', reasonCode: 'QUERY_FAILED', ageMs: null, containers: [container()] },
    { ...base, status: 'AVAILABLE', containers: [{ ...container(), metadata: { ...container().metadata, health: 'READY' } }] },
  ])('rejects malformed projections', value => expect(parse(value)).toBeNull());
  it('accepts disabled and non-available rows only with empty/null values', () => {
    expect(parse({ ...base, status: 'DISABLED', mode: 'DISABLED', reasonCode: 'DISABLED', ageMs: null, containers: [] })?.containers).toEqual([]);
    const unavailable = { ...container(), metadata: { status: 'NOT_FOUND', sampledAt: null, ageMs: null, state: null, health: null, restartCount: null }, cpu: { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, percent: null }, memory: { status: 'UNSUPPORTED', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null } };
    expect(parse({ ...base, containers: [unavailable] })?.containers).toEqual([unavailable]);
    expect(parse({ ...base, status: 'NOT_READY', ageMs: null, containers: [] })?.status).toBe('NOT_READY');
  });
});
