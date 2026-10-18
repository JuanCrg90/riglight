export type GpuMode = 'DISABLED' | 'DEMO' | 'LIVE';
export type GpuStatus = 'DISABLED' | 'NOT_READY' | 'AVAILABLE' | 'UNAVAILABLE' | 'UNSUPPORTED';
export type GpuReason = 'DISABLED' | 'UNSUPPORTED_PLATFORM' | 'BACKEND_UNAVAILABLE' | 'DRIVER_UNAVAILABLE' | 'NO_DEVICES' | 'QUERY_FAILED' | 'DEVICE_LIMIT';
export type GpuMetricStatus = 'AVAILABLE' | 'NOT_READY' | 'UNAVAILABLE' | 'UNSUPPORTED';
export type GpuMetricBase = { status: GpuMetricStatus; sampledAt: string | null; ageMs: number | null };
export type GpuMetric<T extends string> = GpuMetricBase & { [K in T]: number | null };
export type GpuDevice = {
  index: number;
  utilization: GpuMetric<'percent'>;
  memory: GpuMetric<'usedBytes' | 'totalBytes'>;
  temperature: GpuMetric<'celsius'>;
  power: GpuMetric<'watts'>;
};
export type GpuOverview = {
  schemaVersion: 1; sampledAt: string; mode: GpuMode; status: GpuStatus;
  reasonCode: GpuReason | null; isTruncated: boolean; devices: GpuDevice[];
};

const reasons: GpuReason[] = ['DISABLED', 'UNSUPPORTED_PLATFORM', 'BACKEND_UNAVAILABLE', 'DRIVER_UNAVAILABLE', 'NO_DEVICES', 'QUERY_FAILED', 'DEVICE_LIMIT'];
const statuses: GpuMetricStatus[] = ['AVAILABLE', 'NOT_READY', 'UNAVAILABLE', 'UNSUPPORTED'];
const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const hasKeys = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const validTime = (value: unknown): value is string => {
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(value)) return false;
  const parsed = new Date(value);
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 19) === value.slice(0, 19);
};
const safeInt = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value);

function validMetric(value: unknown, fields: string[], validValues: (metric: Record<string, unknown>) => boolean): value is Record<string, unknown> {
  if (!isRecord(value) || !hasKeys(value, ['status', 'sampledAt', 'ageMs', ...fields]) || !statuses.includes(value.status as GpuMetricStatus)) return false;
  if (value.status !== 'AVAILABLE') return value.sampledAt === null && value.ageMs === null && fields.every(field => value[field] === null);
  return validTime(value.sampledAt) && safeInt(value.ageMs) && (value.ageMs as number) >= 0 && validValues(value);
}

function validDevice(value: unknown): value is GpuDevice {
  if (!isRecord(value) || !hasKeys(value, ['index', 'utilization', 'memory', 'temperature', 'power']) || !safeInt(value.index) || (value.index as number) < 0 || (value.index as number) > 15) return false;
  return validMetric(value.utilization, ['percent'], metric => typeof metric.percent === 'number' && Number.isFinite(metric.percent) && metric.percent >= 0 && metric.percent <= 100) &&
    validMetric(value.memory, ['usedBytes', 'totalBytes'], metric => safeInt(metric.usedBytes) && safeInt(metric.totalBytes) && (metric.usedBytes as number) >= 0 && (metric.totalBytes as number) > 0 && (metric.usedBytes as number) <= (metric.totalBytes as number)) &&
    validMetric(value.temperature, ['celsius'], metric => typeof metric.celsius === 'number' && Number.isFinite(metric.celsius) && metric.celsius >= -273.15 && metric.celsius <= 200) &&
    validMetric(value.power, ['watts'], metric => typeof metric.watts === 'number' && Number.isFinite(metric.watts) && metric.watts >= 0 && metric.watts <= 10_000);
}

function validOverviewState(value: Record<string, unknown>): boolean {
  const empty = Array.isArray(value.devices) && value.devices.length === 0 && value.isTruncated === false;
  switch (value.status) {
    case 'AVAILABLE': return value.reasonCode === null ? value.isTruncated === false : value.reasonCode === 'DEVICE_LIMIT' && value.isTruncated === true;
    case 'DISABLED': return value.reasonCode === 'DISABLED' && empty;
    case 'NOT_READY': return value.reasonCode === null && empty;
    case 'UNAVAILABLE': return value.reasonCode === 'QUERY_FAILED' && empty;
    case 'UNSUPPORTED': return ['UNSUPPORTED_PLATFORM', 'BACKEND_UNAVAILABLE', 'DRIVER_UNAVAILABLE', 'NO_DEVICES'].includes(value.reasonCode as string) && empty;
    default: return false;
  }
}

export function parseGpuOverview(raw: string): GpuOverview | null {
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!isRecord(value) || !hasKeys(value, ['schemaVersion', 'sampledAt', 'mode', 'status', 'reasonCode', 'isTruncated', 'devices']) ||
      value.schemaVersion !== 1 || !validTime(value.sampledAt) || !['DISABLED', 'DEMO', 'LIVE'].includes(value.mode as string) ||
      !['DISABLED', 'NOT_READY', 'AVAILABLE', 'UNAVAILABLE', 'UNSUPPORTED'].includes(value.status as string) ||
      (value.reasonCode !== null && !reasons.includes(value.reasonCode as GpuReason)) || typeof value.isTruncated !== 'boolean' || !Array.isArray(value.devices)) return null;
  const devices: unknown[] = value.devices;
  if (devices.length > 16 || !devices.every(validDevice) || devices.some((device, index) => index > 0 && (devices[index - 1] as GpuDevice).index >= device.index) || !validOverviewState(value)) return null;
  return value as unknown as GpuOverview;
}
