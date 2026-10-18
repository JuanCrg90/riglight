export type ModelMode = 'DISABLED' | 'DEMO' | 'LIVE';
export type ModelStatus = 'DISABLED' | 'NOT_READY' | 'AVAILABLE' | 'UNAVAILABLE' | 'UNSUPPORTED';
export type ModelReason = 'DISABLED' | 'API_UNSUPPORTED' | 'CONNECT_FAILED' | 'TIMEOUT' | 'AUTH_REQUIRED' | 'QUERY_FAILED' | 'INVALID_RESPONSE';
export type ModelState = 'LOADED' | 'UNLOADED' | 'UNKNOWN';
export type ModelOverview = {
  schemaVersion: 1; sampledAt: string; mode: ModelMode; status: ModelStatus;
  reasonCode: ModelReason | null; ageMs: number | null; isTruncated: boolean;
  models: { id: string; state: ModelState }[];
};

const reasons: ModelReason[] = ['DISABLED', 'API_UNSUPPORTED', 'CONNECT_FAILED', 'TIMEOUT', 'AUTH_REQUIRED', 'QUERY_FAILED', 'INVALID_RESPONSE'];
const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const hasKeys = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const validTime = (value: unknown): value is string => {
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(value)) return false;
  const date = new Date(value);
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 19) === value.slice(0, 19);
};
const safeInt = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value);

function validId(id: unknown): id is string {
  if (typeof id !== 'string' || id.length === 0 || id !== id.trim() || [...id].length > 128 || new TextEncoder().encode(id).length > 512) return false;
  if ([...id].some(char => /[\\/]/u.test(char) && char === '\\' || /[\p{Cc}\p{Cf}]/u.test(char))) return false;
  if (id.startsWith('/') || id.startsWith('~') || /^[A-Za-z]:/.test(id) || /^[A-Za-z][A-Za-z\d+.-]*:/.test(id) || id.split('/').includes('..')) return false;
  return true;
}

export function parseModelOverview(raw: string): ModelOverview | null {
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!isRecord(value) || !hasKeys(value, ['schemaVersion', 'sampledAt', 'mode', 'status', 'reasonCode', 'ageMs', 'isTruncated', 'models']) ||
      value.schemaVersion !== 1 || !validTime(value.sampledAt) || !['DISABLED', 'DEMO', 'LIVE'].includes(value.mode as string) ||
      !['DISABLED', 'NOT_READY', 'AVAILABLE', 'UNAVAILABLE', 'UNSUPPORTED'].includes(value.status as string) ||
      (value.reasonCode !== null && !reasons.includes(value.reasonCode as ModelReason)) || typeof value.isTruncated !== 'boolean' || !Array.isArray(value.models)) return null;
  const models: unknown[] = value.models;
  if (value.status !== 'AVAILABLE') {
    if (value.ageMs !== null || models.length !== 0 || value.isTruncated !== false) return null;
  } else if (!safeInt(value.ageMs) || (value.ageMs as number) < 0 || models.length > 128 || (value.isTruncated && models.length !== 128)) return null;
  if (!models.every(model => isRecord(model) && hasKeys(model, ['id', 'state']) && validId(model.id) && ['LOADED', 'UNLOADED', 'UNKNOWN'].includes(model.state as string))) return null;
  const ids = models.map(model => (model as { id: string }).id);
  if (new Set(ids).size !== ids.length) return null;
  const enabled = value.mode === 'DEMO' || value.mode === 'LIVE';
  const validStatus = value.status === 'DISABLED' ? value.mode === 'DISABLED' && value.reasonCode === 'DISABLED' :
    value.status === 'NOT_READY' ? enabled && value.reasonCode === null :
    value.status === 'AVAILABLE' ? enabled && value.reasonCode === null :
    value.status === 'UNSUPPORTED' ? enabled && value.reasonCode === 'API_UNSUPPORTED' :
    enabled && ['CONNECT_FAILED', 'TIMEOUT', 'AUTH_REQUIRED', 'QUERY_FAILED', 'INVALID_RESPONSE'].includes(value.reasonCode as string);
  return validStatus ? value as unknown as ModelOverview : null;
}
