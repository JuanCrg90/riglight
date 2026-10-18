export type MetricStatus = 'AVAILABLE' | 'NOT_READY' | 'UNAVAILABLE';
export type Mode = 'DEMO' | 'LIVE';

export interface MetricSample {
  schemaVersion: 1;
  sampledAt: string;
  mode: Mode;
  cpu: {
    status: MetricStatus;
    sampledAt: string | null;
    ageMs: number | null;
    utilizationPercent: number | null;
  };
  memory: {
    status: MetricStatus;
    sampledAt: string | null;
    ageMs: number | null;
    usedBytes: number | null;
    totalBytes: number | null;
  };
}

const MAX_BYTES = Number.MAX_SAFE_INTEGER;
const validTime = (value: unknown): value is string =>
  typeof value === 'string' && /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?(?:Z|[+-]\d\d:\d\d)$/.test(value) &&
  Number.isFinite(Date.parse(value));
const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);
const hasKeys = (value: Record<string, unknown>, keys: string[]) =>
  Object.keys(value).length === keys.length && keys.every((key) => Object.hasOwn(value, key));
const statusOK = (value: unknown): value is MetricStatus =>
  value === 'AVAILABLE' || value === 'NOT_READY' || value === 'UNAVAILABLE';

function validCpu(value: unknown): value is MetricSample['cpu'] {
  if (!isRecord(value) || !hasKeys(value, ['status', 'sampledAt', 'ageMs', 'utilizationPercent']) || !statusOK(value.status)) return false;
  if (value.status !== 'AVAILABLE') return value.sampledAt === null && value.ageMs === null && value.utilizationPercent === null;
  return validTime(value.sampledAt) && Number.isSafeInteger(value.ageMs) && (value.ageMs as number) >= 0 && typeof value.utilizationPercent === 'number' &&
    Number.isFinite(value.utilizationPercent) && value.utilizationPercent >= 0 && value.utilizationPercent <= 100;
}

function validMemory(value: unknown): value is MetricSample['memory'] {
  if (!isRecord(value) || !hasKeys(value, ['status', 'sampledAt', 'ageMs', 'usedBytes', 'totalBytes']) || !statusOK(value.status)) return false;
  if (value.status !== 'AVAILABLE') return value.sampledAt === null && value.ageMs === null && value.usedBytes === null && value.totalBytes === null;
  return validTime(value.sampledAt) && Number.isSafeInteger(value.ageMs) && (value.ageMs as number) >= 0 && Number.isSafeInteger(value.usedBytes) && Number.isSafeInteger(value.totalBytes) &&
    (value.usedBytes as number) >= 0 && (value.usedBytes as number) <= MAX_BYTES && (value.totalBytes as number) > 0 && (value.totalBytes as number) <= MAX_BYTES && (value.usedBytes as number) <= (value.totalBytes as number);
}

export function parseMetricSample(raw: string): MetricSample | null {
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!isRecord(value) || !hasKeys(value, ['schemaVersion', 'sampledAt', 'mode', 'cpu', 'memory']) ||
      value.schemaVersion !== 1 || !validTime(value.sampledAt) ||
      (value.mode !== 'DEMO' && value.mode !== 'LIVE') || !validCpu(value.cpu) || !validMemory(value.memory)) return null;
  return value as unknown as MetricSample;
}
