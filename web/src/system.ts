export type SystemMode = 'DISABLED' | 'DEMO' | 'LIVE';
export type SystemStatus = 'DISABLED' | 'NOT_READY' | 'AVAILABLE' | 'UNAVAILABLE' | 'UNSUPPORTED';
type SensorBase = { status: SystemStatus; sampledAt: string | null; ageMs: number | null };
export type SystemSensor<T> = SensorBase & T;
export type SystemOverview = {
  schemaVersion: 1;
  sampledAt: string;
  mode: SystemMode;
  swap: SystemSensor<{ usedBytes: number | null; totalBytes: number | null }>;
  disk: SystemSensor<{ usedBytes: number | null; availableBytes: number | null; totalBytes: number | null }>;
  network: SystemSensor<{ rxBytesPerSecond: number | null; txBytesPerSecond: number | null }>;
};

const MAX_SYSTEM_PAYLOAD_BYTES = 16 * 1024;
const statuses: SystemStatus[] = ['DISABLED', 'NOT_READY', 'AVAILABLE', 'UNAVAILABLE', 'UNSUPPORTED'];
const record = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const exactKeys = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const safeInt = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const validRate = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= Number.MAX_SAFE_INTEGER;
function validTime(value: unknown): value is string {
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(value)) return false;
  const date = new Date(value);
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 19) === value.slice(0, 19);
}

function validSensor(value: unknown, kind: 'swap' | 'disk' | 'network', enabled: boolean): boolean {
  const measurements = kind === 'swap' ? ['usedBytes', 'totalBytes'] : kind === 'disk'
    ? ['usedBytes', 'availableBytes', 'totalBytes'] : ['rxBytesPerSecond', 'txBytesPerSecond'];
  if (!record(value) || !exactKeys(value, ['status', 'sampledAt', 'ageMs', ...measurements]) || !statuses.includes(value.status as SystemStatus)) return false;
  if (!enabled) return value.status === 'DISABLED' && value.sampledAt === null && value.ageMs === null && measurements.every(key => value[key] === null);
  if (value.status !== 'AVAILABLE') return value.status !== 'DISABLED' && value.sampledAt === null && value.ageMs === null && measurements.every(key => value[key] === null);
  if (!validTime(value.sampledAt) || !safeInt(value.ageMs)) return false;
  if (kind === 'network') return validRate(value.rxBytesPerSecond) && validRate(value.txBytesPerSecond);
  const used = value.usedBytes;
  const total = value.totalBytes;
  if (!safeInt(used) || !safeInt(total) || used > total) return false;
  if (kind === 'swap') return total !== 0 || used === 0;
  const available = value.availableBytes;
  return safeInt(available) && total > 0 && available <= total - used;
}

export function parseSystemOverview(raw: string): SystemOverview | null {
  if (raw.length > MAX_SYSTEM_PAYLOAD_BYTES || new TextEncoder().encode(raw).length > MAX_SYSTEM_PAYLOAD_BYTES) return null;
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!record(value) || !exactKeys(value, ['schemaVersion', 'sampledAt', 'mode', 'swap', 'disk', 'network']) ||
      value.schemaVersion !== 1 || !validTime(value.sampledAt) || !['DISABLED', 'DEMO', 'LIVE'].includes(value.mode as string)) return null;
  const enabled = value.mode !== 'DISABLED';
  if (!validSensor(value.swap, 'swap', enabled) || !validSensor(value.disk, 'disk', enabled) || !validSensor(value.network, 'network', enabled)) return null;
  return value as unknown as SystemOverview;
}
