import { describe, expect, it } from 'vitest';
import { parseGpuOverview } from './gpu';

const at = '2026-01-01T00:00:00Z';
const available = (fields: Record<string, number>) => ({ status: 'AVAILABLE', sampledAt: at, ageMs: 0, ...fields });
const metricUnavailable = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, percent: null };
const device = overviewDevice();
function overviewDevice() { return { index: 0, utilization: available({ percent: 42 }), memory: available({ usedBytes: 8_589_934_592, totalBytes: 25_769_803_776 }), temperature: available({ celsius: 55 }), power: available({ watts: 125.5 }) }; }
const overview = {
  schemaVersion: 1, sampledAt: at, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
  devices: [overviewDevice()],
};

describe('parseGpuOverview', () => {
  it('accepts available and partial metric states', () => {
    expect(parseGpuOverview(JSON.stringify(overview))).toEqual(overview);
    const partial = structuredClone(overview);
    partial.devices[0]!.utilization = metricUnavailable as unknown as typeof partial.devices[number]['utilization'];
    expect(parseGpuOverview(JSON.stringify(partial))?.devices[0]?.utilization.status).toBe('UNAVAILABLE');
  });

  it.each([
    { ...overview, secret: 'private' },
    { ...overview, mode: 'LIVE', devices: [{ ...device, uuid: 'private' }] },
    { ...overview, status: 'DISABLED' },
    { ...overview, reasonCode: 'RAW_DRIVER_ERROR' },
    { ...overview, sampledAt: '2026-02-31T00:00:00Z' },
    { ...overview, isTruncated: true },
    { ...overview, status: 'DISABLED', mode: 'DEMO', reasonCode: 'DISABLED', devices: [] },
    { ...overview, status: 'AVAILABLE', mode: 'DISABLED' },
    { ...overview, devices: [] },
    { ...overview, isTruncated: true, reasonCode: 'DEVICE_LIMIT' },
    { ...overview, devices: [{ ...device, index: 16 }] },
    { ...overview, devices: [device, device] },
    { ...overview, devices: [{ ...device, utilization: { ...device.utilization, percent: 101 } }] },
    { ...overview, devices: [{ ...device, memory: { ...device.memory, usedBytes: Number.MAX_SAFE_INTEGER + 1 } }] },
    { ...overview, devices: [{ ...device, temperature: { ...device.temperature, celsius: -274 } }] },
    { ...overview, devices: [{ ...device, power: { ...device.power, watts: 10001 } }] },
    { ...overview, devices: [{ ...device, utilization: { ...device.utilization, sampledAt: '2026-01-01T00:00:00+00:00' } }] },
    { ...overview, devices: [{ ...device, utilization: { ...metricUnavailable, ageMs: 0 } }] },
    { ...overview, devices: [...overview.devices, { ...device, index: 0 }] },
  ])('rejects malformed or private GPU payloads', value => {
    expect(parseGpuOverview(JSON.stringify(value))).toBeNull();
  });

  it.each([
    [{ ...overview, status: 'DISABLED', mode: 'DISABLED', reasonCode: 'DISABLED', devices: [], isTruncated: false }, true],
    [{ ...overview, status: 'UNSUPPORTED', reasonCode: 'NO_DEVICES', devices: [], isTruncated: false }, true],
    [{ ...overview, status: 'NOT_READY', reasonCode: null, devices: [], isTruncated: false }, true],
    [{ ...overview, status: 'UNAVAILABLE', reasonCode: 'QUERY_FAILED', devices: [], isTruncated: false }, true],
    [{ ...overview, status: 'UNSUPPORTED', reasonCode: 'QUERY_FAILED', devices: [], isTruncated: false }, false],
    [{ ...overview, status: 'DISABLED', mode: 'DISABLED', reasonCode: 'DISABLED', devices: [], isTruncated: false }, true],
    [{ ...overview, status: 'AVAILABLE', devices: Array.from({ length: 16 }, (_, index) => ({ ...device, index })) }, true],
    [{ ...overview, status: 'AVAILABLE', isTruncated: true, reasonCode: 'DEVICE_LIMIT', devices: Array.from({ length: 16 }, (_, index) => ({ ...device, index })) }, true],
    [{ ...overview, status: 'AVAILABLE', isTruncated: true, reasonCode: 'DEVICE_LIMIT', devices: Array.from({ length: 15 }, (_, index) => ({ ...device, index })) }, false],
    [{ ...overview, status: 'AVAILABLE', isTruncated: true, reasonCode: 'DEVICE_LIMIT', devices: Array.from({ length: 17 }, (_, index) => ({ ...device, index })) }, false],
  ])('validates overview state contract', (value, valid) => {
    expect(parseGpuOverview(JSON.stringify(value)) !== null).toBe(valid);
  });
});
