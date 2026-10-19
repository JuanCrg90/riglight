export type ContainerMode = 'DISABLED' | 'DEMO' | 'LIVE';
export type ContainerStatus = 'DISABLED' | 'NOT_READY' | 'AVAILABLE' | 'UNAVAILABLE' | 'UNSUPPORTED';
export type ContainerReason = 'DISABLED' | 'CONNECT_FAILED' | 'TIMEOUT' | 'AUTH_REQUIRED' | 'QUERY_FAILED' | 'INVALID_RESPONSE' | 'API_UNSUPPORTED';
export type ContainerMetadataStatus = 'AVAILABLE' | 'NOT_READY' | 'UNAVAILABLE' | 'NOT_FOUND' | 'UNSUPPORTED';
export type ContainerSensorStatus = 'AVAILABLE' | 'NOT_READY' | 'UNAVAILABLE' | 'UNSUPPORTED';
export type ContainerRow = {
  alias: string;
  metadata: { status: ContainerMetadataStatus; sampledAt: string | null; ageMs: number | null; state: 'CREATED' | 'RUNNING' | 'PAUSED' | 'RESTARTING' | 'REMOVING' | 'EXITED' | 'DEAD' | 'UNKNOWN' | null; health: 'STARTING' | 'HEALTHY' | 'UNHEALTHY' | 'NOT_CONFIGURED' | 'UNKNOWN' | null; restartCount: number | null };
  cpu: { status: ContainerSensorStatus; sampledAt: string | null; ageMs: number | null; percent: number | null };
  memory: { status: ContainerSensorStatus; sampledAt: string | null; ageMs: number | null; usedBytes: number | null; totalBytes: number | null };
};
export type ContainerOverview = { schemaVersion: 1; sampledAt: string; mode: ContainerMode; status: ContainerStatus; reasonCode: ContainerReason | null; ageMs: number | null; containers: ContainerRow[] };

const overviewStatuses: ContainerStatus[] = ['DISABLED', 'NOT_READY', 'AVAILABLE', 'UNAVAILABLE', 'UNSUPPORTED'];
const reasons: ContainerReason[] = ['DISABLED', 'CONNECT_FAILED', 'TIMEOUT', 'AUTH_REQUIRED', 'QUERY_FAILED', 'INVALID_RESPONSE', 'API_UNSUPPORTED'];
const metadataStatuses: ContainerMetadataStatus[] = ['AVAILABLE', 'NOT_READY', 'UNAVAILABLE', 'NOT_FOUND', 'UNSUPPORTED'];
const sensorStatuses: ContainerSensorStatus[] = ['AVAILABLE', 'NOT_READY', 'UNAVAILABLE', 'UNSUPPORTED'];
const states = ['CREATED', 'RUNNING', 'PAUSED', 'RESTARTING', 'REMOVING', 'EXITED', 'DEAD', 'UNKNOWN'];
const health = ['STARTING', 'HEALTHY', 'UNHEALTHY', 'NOT_CONFIGURED', 'UNKNOWN'];
const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const hasKeys = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const safeInt = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value);
const validTime = (value: unknown): value is string => {
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(value)) return false;
  const date = new Date(value);
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 19) === value.slice(0, 19);
};
const validAge = (value: unknown): value is number => safeInt(value) && value >= 0;
const aliasPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/;

function validMetadata(value: unknown): value is ContainerRow['metadata'] {
  if (!isRecord(value) || !hasKeys(value, ['status', 'sampledAt', 'ageMs', 'state', 'health', 'restartCount']) || !metadataStatuses.includes(value.status as ContainerMetadataStatus)) return false;
  if (value.status !== 'AVAILABLE') return value.sampledAt === null && value.ageMs === null && value.state === null && value.health === null && value.restartCount === null;
  return validTime(value.sampledAt) && validAge(value.ageMs) && states.includes(value.state as string) && health.includes(value.health as string) && (value.restartCount === null || validAge(value.restartCount));
}
function validSensor(value: unknown, kind: 'cpu' | 'memory'): boolean {
  if (!isRecord(value)) return false;
  const keys = kind === 'cpu' ? ['status', 'sampledAt', 'ageMs', 'percent'] : ['status', 'sampledAt', 'ageMs', 'usedBytes', 'totalBytes'];
  if (!hasKeys(value, keys) || !sensorStatuses.includes(value.status as ContainerSensorStatus)) return false;
  if (value.status !== 'AVAILABLE') return value.sampledAt === null && value.ageMs === null && keys.slice(3).every(key => value[key] === null);
  if (!validTime(value.sampledAt) || !validAge(value.ageMs)) return false;
  if (kind === 'cpu') return typeof value.percent === 'number' && Number.isFinite(value.percent) && value.percent >= 0 && value.percent <= 409600;
  return validAge(value.usedBytes) && validAge(value.totalBytes) && value.totalBytes > 0 && value.usedBytes <= value.totalBytes;
}

export function parseContainerOverview(raw: string): ContainerOverview | null {
  if (raw.length > 32_768) return null;
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!isRecord(value) || !hasKeys(value, ['schemaVersion', 'sampledAt', 'mode', 'status', 'reasonCode', 'ageMs', 'containers']) || value.schemaVersion !== 1 || !validTime(value.sampledAt) || !['DISABLED', 'DEMO', 'LIVE'].includes(value.mode as string) || !overviewStatuses.includes(value.status as ContainerStatus) || (value.reasonCode !== null && !reasons.includes(value.reasonCode as ContainerReason)) || !Array.isArray(value.containers)) return null;
  const enabled = value.mode !== 'DISABLED';
  const status = value.status as ContainerStatus;
  if (status === 'AVAILABLE') {
    if (!enabled || value.reasonCode !== null || !validAge(value.ageMs) || value.containers.length < 1 || value.containers.length > 16) return null;
  } else if (value.ageMs !== null || value.containers.length !== 0) return null;
  const validStatus = status === 'DISABLED' ? value.mode === 'DISABLED' && value.reasonCode === 'DISABLED' :
    status === 'NOT_READY' ? enabled && value.reasonCode === null :
    status === 'AVAILABLE' ? true :
    status === 'UNSUPPORTED' ? enabled && value.reasonCode === 'API_UNSUPPORTED' :
    enabled && ['CONNECT_FAILED', 'TIMEOUT', 'AUTH_REQUIRED', 'QUERY_FAILED', 'INVALID_RESPONSE'].includes(value.reasonCode as string);
  if (!validStatus) return null;
  const aliases: string[] = [];
  for (const row of value.containers) {
    if (!isRecord(row) || !hasKeys(row, ['alias', 'metadata', 'cpu', 'memory']) || typeof row.alias !== 'string' || !aliasPattern.test(row.alias) || !validMetadata(row.metadata) || !validSensor(row.cpu, 'cpu') || !validSensor(row.memory, 'memory')) return null;
    aliases.push(row.alias);
  }
  if (new Set(aliases).size !== aliases.length || aliases.some((alias, index) => index > 0 && aliases[index - 1]! >= alias)) return null;
  return value as unknown as ContainerOverview;
}
