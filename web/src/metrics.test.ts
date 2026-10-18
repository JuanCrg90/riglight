import { describe, expect, it } from 'vitest';
import { parseMetricSample } from './metrics';

const sample = {
  schemaVersion: 1,
  sampledAt: '2026-01-01T00:00:00Z',
  mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:00Z', ageMs: 0, utilizationPercent: 42.5 },
  memory: { status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:00Z', ageMs: 0, usedBytes: 512, totalBytes: 1024 },
};

describe('parseMetricSample', () => {
  it('accepts the fixed allowlisted schema', () => {
    expect(parseMetricSample(JSON.stringify(sample))).toEqual(sample);
  });

  it.each([
    { ...sample, schemaVersion: 2 },
    { ...sample, secret: 'private' },
    { ...sample, cpu: { ...sample.cpu, utilizationPercent: Infinity } },
    { ...sample, cpu: { ...sample.cpu, utilizationPercent: 101 } },
    { ...sample, cpu: { ...sample.cpu, ageMs: -1 } },
    { ...sample, memory: { ...sample.memory, ageMs: Number.MAX_SAFE_INTEGER + 1 } },
    { ...sample, cpu: { status: 'UNAVAILABLE', sampledAt: null, ageMs: 0, utilizationPercent: null } },
    { ...sample, memory: { ...sample.memory, usedBytes: Number.MAX_SAFE_INTEGER + 1 } },
    { ...sample, memory: { ...sample.memory, usedBytes: 2048 } },
  ])('rejects invalid or private payloads', (value) => {
    expect(parseMetricSample(JSON.stringify(value))).toBeNull();
  });
});
