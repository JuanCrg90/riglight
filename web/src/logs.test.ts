import { describe, expect, it } from 'vitest';
import { appendBoundedLine, parseLogCatalog, parseLogFrame, parseLogStatus, parseLogTarget, readBoundedText } from './logs';

const catalog = { schemaVersion: 1, mode: 'DEMO', sources: [
  { id: 'llama.proxy', label: 'Proxy', kind: 'LLAMA_PROXY' },
  { id: 'model.example-model', label: 'Model', kind: 'MODEL' },
  { id: 'docker.example-api', label: 'API', kind: 'DOCKER' },
] };
const frame = { schemaVersion: 1, mode: 'DEMO', sourceId: 'model.example-model', seq: 1, text: '<img src=x>', stream: 'STDOUT', truncated: false } as const;
const status = (state: string, reasonCode: unknown) => JSON.stringify({ schemaVersion: 1, mode: 'DEMO', sourceId: frame.sourceId, state, reasonCode });

describe('strict log protocol parsers', () => {
  it('accepts strict catalog and all configured source-kind mappings', () => {
    expect(parseLogCatalog(JSON.stringify(catalog), 'DEMO')).toEqual({ mode: 'DEMO', sources: catalog.sources });
    expect(parseLogCatalog(JSON.stringify({ schemaVersion: 1, mode: 'DISABLED', sources: [] }))?.mode).toBe('DISABLED');
    expect(parseLogCatalog(JSON.stringify({ schemaVersion: 1, mode: 'LIVE', sources: [] }))?.sources).toEqual([]);
    expect(parseLogFrame(JSON.stringify(frame), 'DEMO', frame.sourceId, 0)?.text).toBe('<img src=x>');
  });
  it('accepts only model.loaded as the unique loaded-model source and shares the model quota', () => {
    const loaded = { id: 'model.loaded', label: 'Currently loaded model', kind: 'LOADED_MODEL' };
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [loaded] }))?.sources).toEqual([loaded]);
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [loaded, { ...loaded, label: 'Again' }] }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [{ ...loaded, id: 'model.other' }] }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [...catalog.sources, ...Array.from({ length: 16 }, (_, i) => ({ id: `model.model-${i}`, label: 'Model', kind: 'MODEL' })), loaded] }))).toBeNull();
  });
  it('rejects extra fields, duplicates, kind mismatch, wrong JSON types and unsafe labels', () => {
    expect(parseLogCatalog(JSON.stringify({ ...catalog, surprise: true }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [catalog.sources[0], catalog.sources[0]] }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [{ id: 'model.example', label: 'Model', kind: ['MODEL'] }] }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [{ id: 'model.example', label: ' ', kind: 'MODEL' }] }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [{ id: 'model.example', label: 'bad\nlabel', kind: 'MODEL' }] }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ schemaVersion: 1, mode: 'DISABLED', sources: catalog.sources }))).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ schemaVersion: 1, mode: 'LIVE', sources: Array.from({ length: 17 }, (_, i) => ({ id: `model.model-${i}`, label: 'Model', kind: 'MODEL' })) }))).toBeNull();
  });
  it('binds frame source/mode and requires strictly increasing safe sequence numbers', () => {
    expect(parseLogFrame(JSON.stringify(frame), 'LIVE', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, seq: 2 }), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, sourceId: 'docker.example-api' }), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify(frame), 'DEMO', frame.sourceId, 1)).toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, seq: Number.MAX_SAFE_INTEGER + 1 }), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, stream: ['STDOUT'] }), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, text: 'line\r\nnext' }), 'DEMO', frame.sourceId, 0)).toBeNull();
  });
  it('enforces UTF-8 text/wire limits including exact accepted boundaries', () => {
    expect(parseLogFrame(JSON.stringify({ ...frame, text: '😀'.repeat(1024) }), 'DEMO', frame.sourceId, 0)).not.toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, text: '😀'.repeat(1025) }), 'DEMO', frame.sourceId, 0)).toBeNull();
    const wireFrame = JSON.stringify(frame).padEnd(32 * 1024, ' ');
    expect(parseLogFrame(wireFrame, 'DEMO', frame.sourceId, 0)).not.toBeNull();
    expect(parseLogFrame(`${wireFrame} `, 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogCatalog(JSON.stringify(catalog).padEnd(16 * 1024, ' '))).not.toBeNull();
    expect(parseLogCatalog(' '.repeat(16 * 1024 + 1))).toBeNull();
  });
  it('strictly validates automatic target envelopes, revisions and target references', () => {
    const target = { schemaVersion: 1, mode: 'DEMO', sourceId: 'model.loaded', revision: 1, state: 'FOLLOWING', model: 'example-chat' };
    expect(parseLogTarget(JSON.stringify(target), 'DEMO', 'model.loaded', 0)).toEqual(target);
    for (const model of ['Qwen3.6-35B-A3B', '.model', '_model', '-model']) {
      expect(parseLogTarget(JSON.stringify({ ...target, model }), 'DEMO', 'model.loaded', 0)?.model).toBe(model);
    }
    for (const model of ['.', '..', 'proxy', 'upstream', 'http', 'unsafe/name', 'unsafe\\\\name', 'unsafe%2Fname', 'mödèl', 'model\nname']) {
      expect(parseLogTarget(JSON.stringify({ ...target, model }), 'DEMO', 'model.loaded', 0)).toBeNull();
    }
    for (const reserved of ['proxy', 'upstream', 'http']) {
      expect(parseLogTarget(JSON.stringify({ ...target, model: reserved }), 'DEMO', 'model.loaded', 0)).toBeNull();
    }
    for (const invalid of [
      { ...target, surprise: true }, { ...target, mode: 'LIVE' }, { ...target, sourceId: 'model.other' },
      { ...target, revision: Number.MAX_SAFE_INTEGER + 1 }, { ...target, revision: 2 },
      { ...target, model: 'x'.repeat(129) }, { ...target, model: 'unsafe/name' },
      { ...target, state: 'WAITING', model: 'example-chat' },
      { ...target, state: 'WAITING', model: null, revision: 0 },
    ]) expect(parseLogTarget(JSON.stringify(invalid), 'DEMO', 'model.loaded', 0)).toBeNull();
    expect(parseLogTarget(JSON.stringify({ ...target, revision: 2 }), 'DEMO', 'model.loaded', 1)?.revision).toBe(2);
    for (const state of ['WAITING', 'UNAVAILABLE', 'AMBIGUOUS', 'UNSUPPORTED']) {
      expect(parseLogTarget(JSON.stringify({ ...target, revision: 1, state, model: null }), 'DEMO', 'model.loaded', 0)?.state).toBe(state);
    }
  });
  it('accepts only exact status reason/state combinations and literal string reasons', () => {
    expect(parseLogStatus(status('ENDED', 'BYTE_LIMIT'), 'DEMO', frame.sourceId)).toBeNull();
    expect(parseLogStatus(status('ENDED', ['EOF']), 'DEMO', frame.sourceId)).toBeNull();
    expect(parseLogStatus(status('ENDED', 'EOF'), 'DEMO', frame.sourceId)?.state).toBe('ENDED');
    expect(parseLogStatus(status('OPENING', 'EOF'), 'DEMO', frame.sourceId)).toBeNull();
    expect(parseLogStatus(JSON.stringify({ ...JSON.parse(status('STREAMING', null)), extra: 1 }), 'DEMO', frame.sourceId)).toBeNull();
  });
});

describe('bounded browser retention and catalog body reader', () => {
  it('evicts oldest frames to satisfy both line and UTF-8 byte limits purely', () => {
    let retained = { lines: [], bytes: 0 } as ReturnType<typeof appendBoundedLine>;
    for (let seq = 1; seq <= 201; seq++) retained = appendBoundedLine(retained, { ...frame, seq, text: '😀' }, `s1-${seq}`, 200, 256 * 1024);
    expect(retained.lines).toHaveLength(200);
    expect(retained.lines[0]?.frame.seq).toBe(2);
    for (let seq = 202; seq <= 267; seq++) retained = appendBoundedLine(retained, { ...frame, seq, text: '😀'.repeat(1024) }, `s1-${seq}`, 200, 256 * 1024);
    expect(retained.bytes).toBeLessThanOrEqual(256 * 1024);
    expect(retained.lines.reduce((sum, line) => sum + new TextEncoder().encode(line.frame.text).byteLength, 0)).toBe(retained.bytes);
  });
  it('reads only bounded response bytes and decodes UTF-8 strictly', async () => {
    const signal = new AbortController().signal;
    await expect(readBoundedText(new Response('x'.repeat(17)), 16, signal)).rejects.toThrow(/large/);
    expect(await readBoundedText(new Response('{"ok":true}'), 32, signal)).toBe('{"ok":true}');
    await expect(readBoundedText(new Response(new Uint8Array([0xff])), 32, signal)).rejects.toThrow();
  });
  it('cancels an in-flight reader when aborted', async () => {
    const controller = new AbortController();
    let canceled = false;
    const stream = new ReadableStream<Uint8Array>({
      start() {},
      cancel() { canceled = true; },
    });
    const pending = readBoundedText(new Response(stream), 32, controller.signal);
    controller.abort();
    await expect(pending).rejects.toThrow();
    expect(canceled).toBe(true);
  });
});
