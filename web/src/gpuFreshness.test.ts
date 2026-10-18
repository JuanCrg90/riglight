import { describe, expect, it } from 'vitest';
import { parseGpuOverview } from './gpu';
import { expireGpuFreshness, gpuMetricFreshness } from './gpuFreshness';
const at = '2026-01-01T00:00:00Z';
const metric = (field: string, value: number, time = at, age = 0) => ({ status: 'AVAILABLE', sampledAt: time, ageMs: age, [field]: value });
const payload = (time = at, age = 0) => ({ schemaVersion: 1, sampledAt: time, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
  devices: [{ index: 0, utilization: metric('percent', 40, at, age), memory: metric('usedBytes', 400, at, age) as ReturnType<typeof metric> & { totalBytes: number }, temperature: metric('celsius', 50, at, age), power: metric('watts', 100, at, age) }] });
function parsed(value = payload()) { value.devices[0]!.memory.totalBytes = 1_000; return parseGpuOverview(JSON.stringify(value))!; }

describe('GPU metric freshness', () => {
  it('expires sensors independently and CPU frames are irrelevant', () => {
    const expiries = new Map();
    const first = parsed();
    expect(gpuMetricFreshness(first, null, expiries, 100)[0]?.utilization).toBe(true);
    const repeated = parsed(payload('2026-01-01T00:00:01Z', 5_500));
    const freshness = gpuMetricFreshness(repeated, first, expiries, 1_000);
    expect(freshness[0]?.utilization).toBe(true);
    expect(expireGpuFreshness(freshness, expiries, 6_001)[0]?.utilization).toBe(false);
  });

  it('does not renew a repeated metric timestamp and drops removed devices', () => {
    const expiries = new Map();
    const first = parsed();
    gpuMetricFreshness(first, null, expiries, 0);
    const replay = parsed(payload('2026-01-01T00:00:01Z', 0));
    const freshness = gpuMetricFreshness(replay, first, expiries, 5_000);
    expect(expireGpuFreshness(freshness, expiries, 6_000)[0]?.memory).toBe(false);
    const empty = parseGpuOverview(JSON.stringify({ ...payload(), status: 'UNSUPPORTED', reasonCode: 'NO_DEVICES', devices: [] }))!;
    expect(gpuMetricFreshness(empty, replay, expiries, 6_100)).toEqual({});
    expect(expiries.size).toBe(0);
  });
});
