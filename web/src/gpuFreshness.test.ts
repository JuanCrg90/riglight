import { describe, expect, it } from 'vitest';
import { parseGpuOverview } from './gpu';
import { expireGpuFreshness, gpuMetricFreshness } from './gpuFreshness';
const at = '2026-01-01T00:00:00Z';
const metric = (field: string, value: number, time = at, age = 0) => ({ status: 'AVAILABLE', sampledAt: time, ageMs: age, [field]: value });
const payload = (time = at, age = 0) => ({ schemaVersion: 1, sampledAt: time, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
  devices: [{ index: 0, utilization: metric('percent', 40, time, age), memory: { ...metric('usedBytes', 400, time, age), totalBytes: 1_000 }, temperature: metric('celsius', 50, time, age), power: metric('watts', 100, time, age) }] });
function parsed(value = payload()) { return parseGpuOverview(JSON.stringify(value))!; }
function setUtilizationTime(value: ReturnType<typeof payload>, sampledAt: string) { value.devices[0]!.utilization.sampledAt = sampledAt; return value; }
function setAllMetricTimes(value: ReturnType<typeof payload>, sampledAt: string) {
  const device = value.devices[0]!;
  device.utilization.sampledAt = sampledAt;
  device.memory.sampledAt = sampledAt;
  device.temperature.sampledAt = sampledAt;
  device.power.sampledAt = sampledAt;
  return value;
}

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

  it('does not renew A across unavailable readings or an A→B→A replay', () => {
    const expiries = new Map();
    const first = parsed();
    gpuMetricFreshness(first, null, expiries, 0);
    const unavailable = parsed(payload('2026-01-01T00:00:01Z'));
    unavailable.devices[0]!.utilization = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, percent: null };
    gpuMetricFreshness(unavailable, first, expiries, 2_000);
    const replayA = parsed(setUtilizationTime(payload('2026-01-01T00:00:02Z', 0), at));
    const afterUnavailable = gpuMetricFreshness(replayA, unavailable, expiries, 5_000);
    expect(expireGpuFreshness(afterUnavailable, expiries, 6_000)[0]?.utilization).toBe(false);

    const b = parsed(payload('2026-01-01T00:00:03Z'));
    gpuMetricFreshness(b, replayA, expiries, 7_000);
    const replayAAgain = parsed(setUtilizationTime(payload('2026-01-01T00:00:04Z'), at));
    const afterAAgain = gpuMetricFreshness(replayAAgain, b, expiries, 8_000);
    expect(expireGpuFreshness(afterAAgain, expiries, 10_000)[0]?.utilization).toBe(false);
  });

  it('clears per-sensor identity metadata when the GPU mode changes', () => {
    const expiries = new Map();
    const demo = parsed();
    gpuMetricFreshness(demo, null, expiries, 0);
    expect(expiries.size).toBe(4);
    const live = structuredClone(demo);
    live.mode = 'LIVE';
    const freshness = gpuMetricFreshness(live, demo, expiries, 5_000);
    expect(freshness[0]?.utilization).toBe(true);
    expect(expiries.size).toBe(4);
    expect(expireGpuFreshness(freshness, expiries, 6_000)[0]?.utilization).toBe(true);
    expect(expireGpuFreshness(freshness, expiries, 11_000)[0]?.utilization).toBe(false);
  });

  it('accepts a distinct earlier server timestamp after clock rollback', () => {
    const expiries = new Map();
    const first = parsed(payload('2026-01-01T00:00:10Z'));
    gpuMetricFreshness(first, null, expiries, 0);
    const earlier = parsed(payload('2026-01-01T00:00:05Z'));
    const freshness = gpuMetricFreshness(earlier, first, expiries, 5_000);
    expect(freshness[0]?.utilization).toBe(true);
    expect(expireGpuFreshness(freshness, expiries, 11_000)[0]?.utilization).toBe(false);
  });

  it('preserves identity expiry across query failure but frees state on known removal', () => {
    const expiries = new Map();
    const first = parsed();
    gpuMetricFreshness(first, null, expiries, 0);
    const queryFailed = parseGpuOverview(JSON.stringify({ ...payload(), status: 'UNAVAILABLE', reasonCode: 'QUERY_FAILED', devices: [] }))!;
    gpuMetricFreshness(queryFailed, first, expiries, 1_000);
    const recovered = parsed(setUtilizationTime(payload('2026-01-01T00:00:02Z'), at));
    const freshness = gpuMetricFreshness(recovered, queryFailed, expiries, 5_000);
    expect(expireGpuFreshness(freshness, expiries, 6_000)[0]?.utilization).toBe(false);
    const removed = parseGpuOverview(JSON.stringify({ ...payload(), status: 'UNSUPPORTED', reasonCode: 'NO_DEVICES', devices: [] }))!;
    gpuMetricFreshness(removed, recovered, expiries, 6_100);
    expect(expiries.size).toBe(0);
  });

  it('does not renew a repeated metric timestamp and drops removed devices', () => {
    const expiries = new Map();
    const first = parsed();
    gpuMetricFreshness(first, null, expiries, 0);
    const replay = parsed(setAllMetricTimes(payload('2026-01-01T00:00:01Z', 0), at));
    const freshness = gpuMetricFreshness(replay, first, expiries, 5_000);
    expect(expireGpuFreshness(freshness, expiries, 6_000)[0]?.memory).toBe(false);
    const empty = parseGpuOverview(JSON.stringify({ ...payload(), status: 'UNSUPPORTED', reasonCode: 'NO_DEVICES', devices: [] }))!;
    expect(gpuMetricFreshness(empty, replay, expiries, 6_100)).toEqual({});
    expect(expiries.size).toBe(0);
  });
});
