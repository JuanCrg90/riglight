import { describe, expect, it } from 'vitest';
import { parseLogCatalog, parseLogFrame, parseLogStatus } from './logs';

const catalog = { schemaVersion: 1, mode: 'DEMO', sources: [
  { id: 'llama.proxy', label: 'Proxy', kind: 'LLAMA_PROXY' },
  { id: 'model.example-model', label: 'Model', kind: 'MODEL' },
  { id: 'docker.example-api', label: 'API', kind: 'DOCKER' },
] };
const frame = { schemaVersion: 1, mode: 'DEMO', sourceId: 'model.example-model', seq: 1, text: '<img src=x>', stream: 'STDOUT', truncated: false };

describe('strict log protocol parsers', () => {
  it('accepts bounded catalog and log frames', () => {
    expect(parseLogCatalog(JSON.stringify(catalog), 'DEMO')).toHaveLength(3);
    expect(parseLogFrame(JSON.stringify(frame), 'DEMO', 'model.example-model', 0)?.text).toBe('<img src=x>');
  });
  it('rejects extra keys, duplicate ids, mismatched source/mode and nonincreasing sequence', () => {
    expect(parseLogCatalog(JSON.stringify({ ...catalog, surprise: true }), 'DEMO')).toBeNull();
    expect(parseLogCatalog(JSON.stringify({ ...catalog, sources: [catalog.sources[0], catalog.sources[0]] }), 'DEMO')).toBeNull();
    expect(parseLogFrame(JSON.stringify(frame), 'LIVE', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify({ ...frame, sourceId: 'docker.example-api' }), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(JSON.stringify(frame), 'DEMO', frame.sourceId, 1)).toBeNull();
  });
  it('enforces frame, text UTF-8 and status reason bounds', () => {
    expect(parseLogFrame(JSON.stringify({ ...frame, text: '😀'.repeat(1025) }), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogFrame(' '.repeat(32 * 1024 + 1), 'DEMO', frame.sourceId, 0)).toBeNull();
    expect(parseLogStatus(JSON.stringify({ schemaVersion: 1, mode: 'DEMO', sourceId: frame.sourceId, state: 'ENDED', reasonCode: 'BYTE_LIMIT' }), 'DEMO', frame.sourceId)).toBeNull();
    expect(parseLogStatus(JSON.stringify({ schemaVersion: 1, mode: 'DEMO', sourceId: frame.sourceId, state: 'ENDED', reasonCode: 'EOF' }), 'DEMO', frame.sourceId)?.state).toBe('ENDED');
  });
});
