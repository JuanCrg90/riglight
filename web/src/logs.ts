export type CatalogMode = 'DISABLED' | 'DEMO' | 'LIVE';
export type LogMode = Exclude<CatalogMode, 'DISABLED'>;
export type LogKind = 'LLAMA_PROXY' | 'MODEL' | 'DOCKER' | 'LOADED_MODEL';
export type LogSource = { id: string; label: string; kind: LogKind };
export type LogCatalog = { mode: CatalogMode; sources: LogSource[] };
export type LogFrame = { schemaVersion: 1; mode: LogMode; sourceId: string; seq: number; text: string; stream: 'OUTPUT' | 'STDOUT' | 'STDERR'; truncated: boolean };
export type LogStatus = { schemaVersion: 1; mode: LogMode; sourceId: string; state: 'OPENING' | 'STREAMING' | 'ENDED' | 'UNAVAILABLE' | 'LIMIT_REACHED'; reasonCode: string | null };
export type LogTargetState = 'FOLLOWING' | 'WAITING' | 'UNAVAILABLE' | 'AMBIGUOUS' | 'UNSUPPORTED';
export type LogTarget = { schemaVersion: 1; mode: LogMode; sourceId: 'model.loaded'; revision: number; state: LogTargetState; model: string | null };
export type RetainedLogLine = { frame: LogFrame; key: string };
export type BoundedLines = { lines: RetainedLogLine[]; bytes: number };
const encoder = new TextEncoder();
const bytes = (value: string) => encoder.encode(value).byteLength;
const object = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const exact = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const modes: CatalogMode[] = ['DISABLED', 'DEMO', 'LIVE'];
const kinds: LogKind[] = ['LLAMA_PROXY', 'MODEL', 'DOCKER', 'LOADED_MODEL'];
const streams: LogFrame['stream'][] = ['OUTPUT', 'STDOUT', 'STDERR'];
const reasons = ['DISABLED', 'UNSUPPORTED', 'UPSTREAM_UNAVAILABLE', 'INVALID_RESPONSE', 'READ_FAILED', 'EOF', 'SESSION_LIMIT', 'BYTE_LIMIT', 'LINE_LIMIT'] as const;
const states: LogStatus['state'][] = ['OPENING', 'STREAMING', 'ENDED', 'UNAVAILABLE', 'LIMIT_REACHED'];
const hasControl = (value: string, allowTab: boolean) => Array.from(value).some(char => {
  const code = char.codePointAt(0)!;
  return (code < 32 && !(allowTab && code === 9)) || (code >= 127 && code <= 159);
});
const safeLabel = (label: string) => label.trim().length > 0 && !hasControl(label, false);

export function parseLogCatalog(raw: string, expectedMode?: CatalogMode): LogCatalog | null {
  if (bytes(raw) > 16 * 1024) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sources']) || value.schemaVersion !== 1 || typeof value.mode !== 'string' || !modes.includes(value.mode as CatalogMode) || (expectedMode !== undefined && value.mode !== expectedMode) || !Array.isArray(value.sources) || value.sources.length > 33) return null;
    const ids = new Set<string>(); const sources: LogSource[] = [];
    let proxies = 0; let models = 0; let docker = 0; let loadedModels = 0;
    for (const entry of value.sources) {
      if (!object(entry) || !exact(entry, ['id', 'label', 'kind']) || typeof entry.id !== 'string' || !/^(llama\.proxy|model\.[A-Za-z0-9][A-Za-z0-9._-]*|docker\.[A-Za-z0-9][A-Za-z0-9._-]*)$/.test(entry.id) || bytes(entry.id) > 80 || typeof entry.label !== 'string' || bytes(entry.label) > 128 || !safeLabel(entry.label) || typeof entry.kind !== 'string' || !kinds.includes(entry.kind as LogKind) || ids.has(entry.id)) return null;
      if ((entry.id.startsWith('model.') && !(entry.id === 'model.loaded' && entry.kind === 'LOADED_MODEL') && entry.kind !== 'MODEL') || (entry.kind === 'LOADED_MODEL' && entry.id !== 'model.loaded') || (entry.id.startsWith('docker.') && entry.kind !== 'DOCKER') || (entry.id === 'llama.proxy' && entry.kind !== 'LLAMA_PROXY')) return null;
      ids.add(entry.id); sources.push({ id: entry.id, label: entry.label, kind: entry.kind as LogKind });
      if (entry.kind === 'LLAMA_PROXY' && ++proxies > 1) return null;
      if ((entry.kind === 'MODEL' || entry.kind === 'LOADED_MODEL') && ++models > 16) return null;
      if (entry.kind === 'LOADED_MODEL' && ++loadedModels > 1) return null;
      if (entry.kind === 'DOCKER' && ++docker > 16) return null;
    }
    if (value.mode === 'DISABLED' && sources.length !== 0) return null;
    return { mode: value.mode as CatalogMode, sources };
  } catch { return null; }
}

export function parseLogFrame(raw: string, mode: LogMode, sourceId: string, lastSeq: number): LogFrame | null {
  if (bytes(raw) > 32 * 1024) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sourceId', 'seq', 'text', 'stream', 'truncated']) || value.schemaVersion !== 1 || value.mode !== mode || value.sourceId !== sourceId || typeof value.seq !== 'number' || !Number.isSafeInteger(value.seq) || value.seq <= lastSeq || value.seq < 1 || (lastSeq === 0 && value.seq !== 1) || typeof value.text !== 'string' || bytes(value.text) > 4096 || hasControl(value.text, true) || value.text.includes('\r') || value.text.includes('\n') || typeof value.stream !== 'string' || !streams.includes(value.stream as LogFrame['stream']) || typeof value.truncated !== 'boolean') return null;
    return value as LogFrame;
  } catch { return null; }
}

export function parseLogStatus(raw: string, mode: LogMode, sourceId: string): LogStatus | null {
  if (bytes(raw) > 32 * 1024) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sourceId', 'state', 'reasonCode']) || value.schemaVersion !== 1 || value.mode !== mode || value.sourceId !== sourceId || typeof value.state !== 'string' || !states.includes(value.state as LogStatus['state']) || !(value.reasonCode === null || (typeof value.reasonCode === 'string' && (reasons as readonly string[]).includes(value.reasonCode)))) return null;
    const valid = value.state === 'OPENING' || value.state === 'STREAMING' ? value.reasonCode === null : value.state === 'ENDED' ? value.reasonCode === 'EOF' : value.state === 'LIMIT_REACHED' ? ['SESSION_LIMIT', 'BYTE_LIMIT', 'LINE_LIMIT'].includes(value.reasonCode as string) : value.state === 'UNAVAILABLE' && ['DISABLED', 'UNSUPPORTED', 'UPSTREAM_UNAVAILABLE', 'INVALID_RESPONSE', 'READ_FAILED'].includes(value.reasonCode as string);
    return valid ? value as LogStatus : null;
  } catch { return null; }
}

export function parseLogTarget(raw: string, mode: LogMode, sourceId: string, lastRevision: number): LogTarget | null {
  if (sourceId !== 'model.loaded' || bytes(raw) > 32 * 1024 || !Number.isSafeInteger(lastRevision) || lastRevision < 0) return null;
  try {
    const value: unknown = JSON.parse(raw);
    const targetStates: LogTargetState[] = ['FOLLOWING', 'WAITING', 'UNAVAILABLE', 'AMBIGUOUS', 'UNSUPPORTED'];
    if (!object(value) || !exact(value, ['schemaVersion', 'mode', 'sourceId', 'revision', 'state', 'model']) || value.schemaVersion !== 1 || value.mode !== mode || value.sourceId !== sourceId || typeof value.revision !== 'number' || !Number.isSafeInteger(value.revision) || value.revision <= lastRevision || value.revision < 1 || (lastRevision === 0 && value.revision !== 1) || typeof value.state !== 'string' || !targetStates.includes(value.state as LogTargetState)) return null;
    if (value.state === 'FOLLOWING') {
      if (typeof value.model !== 'string' || bytes(value.model) > 128 || !/^[A-Za-z0-9_.-]+$/.test(value.model) || value.model === '.' || value.model === '..' || ['proxy', 'upstream', 'http'].includes(value.model)) return null;
    } else if (value.model !== null) return null;
    return value as LogTarget;
  } catch { return null; }
}

export function appendBoundedLine(current: BoundedLines, frame: LogFrame, key: string, maxLines = 200, maxBytes = 256 * 1024): BoundedLines {
  const lineBytes = bytes(frame.text);
  const lines = [...current.lines, { frame, key }];
  let retainedBytes = current.bytes + lineBytes;
  let first = 0;
  while (lines.length - first > maxLines || retainedBytes > maxBytes) retainedBytes -= bytes(lines[first++]!.frame.text);
  return { lines: lines.slice(first), bytes: retainedBytes };
}

export async function readBoundedText(response: Response, maxBytes: number, signal: AbortSignal): Promise<string> {
  if (!response.body) throw new Error('Missing response body');
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = []; let total = 0;
  const cancel = () => { void reader.cancel().catch(() => undefined); };
  signal.addEventListener('abort', cancel, { once: true });
  try {
    while (true) {
      if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
      const { done, value } = await reader.read();
      if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
      if (done) break;
      total += value.byteLength;
      if (total > maxBytes) throw new Error('Response too large');
      chunks.push(value);
    }
    const combined = new Uint8Array(total); let offset = 0;
    for (const chunk of chunks) { combined.set(chunk, offset); offset += chunk.byteLength; }
    return new TextDecoder('utf-8', { fatal: true }).decode(combined);
  } finally { signal.removeEventListener('abort', cancel); await reader.cancel().catch(() => undefined); }
}
