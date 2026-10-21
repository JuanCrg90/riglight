export type LogMode = 'DEMO' | 'LIVE';
export type LogKind = 'LLAMA_PROXY' | 'MODEL' | 'DOCKER';
export type LogSource = { id: string; label: string; kind: LogKind };
export type LogFrame = { schemaVersion: 1; mode: LogMode; sourceId: string; seq: number; text: string; stream: 'OUTPUT' | 'STDOUT' | 'STDERR'; truncated: boolean };
export type LogStatus = { schemaVersion: 1; mode: LogMode; sourceId: string; state: 'OPENING' | 'STREAMING' | 'ENDED' | 'UNAVAILABLE' | 'LIMIT_REACHED'; reasonCode: string | null };
const encoder = new TextEncoder();
const bytes = (value: string) => encoder.encode(value).byteLength;
const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const exact = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const modes = ['DISABLED', 'DEMO', 'LIVE'];
const reasons = ['DISABLED', 'UNSUPPORTED', 'UPSTREAM_UNAVAILABLE', 'INVALID_RESPONSE', 'READ_FAILED', 'EOF', 'SESSION_LIMIT', 'BYTE_LIMIT', 'LINE_LIMIT'];

export function parseLogCatalog(raw: string, expectedMode?: LogMode): LogSource[] | null {
  if (bytes(raw) > 16 * 1024) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sources']) || value.schemaVersion !== 1 || !modes.includes(String(value.mode)) || (expectedMode && value.mode !== expectedMode) || !Array.isArray(value.sources) || value.sources.length > 33) return null;
    const ids = new Set<string>(); const result: LogSource[] = [];
    for (const entry of value.sources) {
      if (!object(entry) || !exact(entry, ['id', 'label', 'kind']) || typeof entry.id !== 'string' || !/^(llama\.proxy|model\.[A-Za-z0-9][A-Za-z0-9._-]*|docker\.[A-Za-z0-9][A-Za-z0-9._-]*)$/.test(entry.id) || bytes(entry.id) > 80 || typeof entry.label !== 'string' || bytes(entry.label) > 128 || !['LLAMA_PROXY', 'MODEL', 'DOCKER'].includes(String(entry.kind)) || ids.has(entry.id)) return null;
      if ((entry.id.startsWith('model.') && entry.kind !== 'MODEL') || (entry.id.startsWith('docker.') && entry.kind !== 'DOCKER') || (entry.id === 'llama.proxy' && entry.kind !== 'LLAMA_PROXY')) return null;
      ids.add(entry.id); result.push({ id: entry.id, label: entry.label, kind: entry.kind as LogKind });
    }
    return result;
  } catch { return null; }
}
export function parseLogFrame(raw: string, mode: LogMode, sourceId: string, lastSeq: number): LogFrame | null {
  if (bytes(raw) > 32 * 1024) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sourceId', 'seq', 'text', 'stream', 'truncated']) || value.schemaVersion !== 1 || value.mode !== mode || value.sourceId !== sourceId || typeof value.seq !== 'number' || !Number.isSafeInteger(value.seq) || value.seq <= lastSeq || value.seq < 1 || typeof value.text !== 'string' || bytes(value.text) > 4096 || !['OUTPUT', 'STDOUT', 'STDERR'].includes(String(value.stream)) || typeof value.truncated !== 'boolean') return null;
    return value as LogFrame;
  } catch { return null; }
}
export function parseLogStatus(raw: string, mode: LogMode, sourceId: string): LogStatus | null {
  if (bytes(raw) > 32 * 1024) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sourceId', 'state', 'reasonCode']) || value.schemaVersion !== 1 || value.mode !== mode || value.sourceId !== sourceId || !['OPENING', 'STREAMING', 'ENDED', 'UNAVAILABLE', 'LIMIT_REACHED'].includes(String(value.state)) || !(value.reasonCode === null || reasons.includes(String(value.reasonCode)))) return null;
    const valid = value.state === 'OPENING' || value.state === 'STREAMING' ? value.reasonCode === null : value.state === 'ENDED' ? value.reasonCode === 'EOF' : value.state === 'LIMIT_REACHED' ? ['SESSION_LIMIT', 'BYTE_LIMIT', 'LINE_LIMIT'].includes(String(value.reasonCode)) : value.state === 'UNAVAILABLE' && ['DISABLED', 'UNSUPPORTED', 'UPSTREAM_UNAVAILABLE', 'INVALID_RESPONSE', 'READ_FAILED'].includes(String(value.reasonCode));
    return valid ? value as LogStatus : null;
  } catch { return null; }
}
