import { describe, expect, it } from 'vitest';
import { parseSystemOverview } from './system';

const sensor = (values: Record<string, unknown> = {}) => ({
  status: 'AVAILABLE', sampledAt: '2026-01-01T00:00:00Z', ageMs: 0, ...values,
});
const fixture = () => ({
  schemaVersion: 1,
  sampledAt: '2026-01-01T00:00:00Z',
  mode: 'DEMO',
  swap: sensor({ usedBytes: 0, totalBytes: 0 }),
  disk: sensor({ usedBytes: 1, availableBytes: 2, totalBytes: 4 }),
  network: sensor({ rxBytesPerSecond: 0.25, txBytesPerSecond: 0 }),
});

const parse = (value: unknown) => parseSystemOverview(JSON.stringify(value));

describe('parseSystemOverview', () => {
  it('accepts the exact DTO and preserves genuine no-swap and fractional rates', () => {
    const result = parse(fixture());
    expect(result?.swap).toMatchObject({ status: 'AVAILABLE', usedBytes: 0, totalBytes: 0 });
    expect(result?.network.rxBytesPerSecond).toBe(0.25);
  });

  it('rejects payloads larger than the bounded flat DTO allowance before parsing', () => {
    const raw = `${' '.repeat(16 * 1024)}${JSON.stringify(fixture())}`;
    expect(raw.length).toBeGreaterThan(16 * 1024);
    expect(parseSystemOverview(raw)).toBeNull();
  });

  it('requires exact object keys and all sensor measurements', () => {
    const missing = fixture();
    delete (missing as Record<string, unknown>).mode;
    expect(parse(missing)).toBeNull();
    const extra = fixture() as Record<string, unknown>;
    extra.host = 'private';
    expect(parse(extra)).toBeNull();
    const noNulls = fixture();
    noNulls.swap = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null } as never;
    expect(parse(noNulls)).toBeNull();
  });

  it('rejects invalid status/mode combinations and malformed numbers', () => {
    const disabled = fixture();
    disabled.mode = 'DISABLED';
    expect(parse(disabled)).toBeNull();
    const broken = fixture();
    broken.network = sensor({ rxBytesPerSecond: Number.MAX_SAFE_INTEGER + 1, txBytesPerSecond: 0 }) as never;
    expect(parse(broken)).toBeNull();
    const badDisk = fixture();
    badDisk.disk = sensor({ usedBytes: 3, availableBytes: 2, totalBytes: 4 }) as never;
    expect(parse(badDisk)).toBeNull();
    const badSwap = fixture();
    badSwap.swap = sensor({ usedBytes: 1, totalBytes: 0 }) as never;
    expect(parse(badSwap)).toBeNull();
  });

  it('accepts each partial sensor status with exact null measurements', () => {
    const value = fixture();
    value.swap = { status: 'NOT_READY', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null } as never;
    value.disk = { status: 'UNSUPPORTED', sampledAt: null, ageMs: null, usedBytes: null, availableBytes: null, totalBytes: null } as never;
    value.network = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, rxBytesPerSecond: null, txBytesPerSecond: null } as never;
    expect(parse(value)?.swap.status).toBe('NOT_READY');
    expect(parse(value)?.disk.status).toBe('UNSUPPORTED');
    expect(parse(value)?.network.status).toBe('UNAVAILABLE');
    const disabled = { ...fixture(), mode: 'DISABLED',
      swap: { status: 'DISABLED', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null },
      disk: { status: 'DISABLED', sampledAt: null, ageMs: null, usedBytes: null, availableBytes: null, totalBytes: null },
      network: { status: 'DISABLED', sampledAt: null, ageMs: null, rxBytesPerSecond: null, txBytesPerSecond: null } };
    expect(parse(disabled)?.mode).toBe('DISABLED');
  });

  it('requires unavailable sensors to contain null timestamp, age, and measurements', () => {
    const value = fixture();
    value.swap = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null } as never;
    expect(parse(value)?.swap.status).toBe('UNAVAILABLE');
    value.swap = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, usedBytes: 0, totalBytes: null } as never;
    expect(parse(value)).toBeNull();
  });
});
