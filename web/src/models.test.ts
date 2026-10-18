import { describe, expect, it } from 'vitest';
import { parseModelOverview } from './models';

const base = { schemaVersion: 1, sampledAt: '2026-01-01T00:00:00Z', mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, ageMs: 0, isTruncated: false, models: [{ id: 'example-org/example-chat', state: 'LOADED' }] };
const parse = (value: unknown) => parseModelOverview(JSON.stringify(value));
const parseRawId = (encodedId: string) => parseModelOverview(JSON.stringify({ ...base, models: [{ id: '__MODEL_ID__', state: 'LOADED' }] }).replace('"__MODEL_ID__"', encodedId));

describe('parseModelOverview', () => {
  it('accepts a strict synthetic available inventory', () => expect(parse(base)).toEqual(base));
  it.each([
    { ...base, extra: true }, { ...base, mode: 'DISABLED' }, { ...base, status: 'UNSUPPORTED', reasonCode: null },
    { ...base, sampledAt: '2026-01-01T00:00:00+00:00' }, { ...base, ageMs: -1 },
    { ...base, models: [{ id: '../secret', state: 'LOADED' }] }, { ...base, models: [{ id: 'bad\\alias', state: 'LOADED' }] },
    { ...base, models: [{ id: 'example-chat', state: 'READY' }] },
    { ...base, models: [{ id: 'x', state: 'UNKNOWN', metadata: 'private' }] },
    { ...base, models: [{ id: 'x', state: 'LOADED' }, { id: 'x', state: 'UNLOADED' }] },
    { ...base, status: 'UNAVAILABLE', reasonCode: 'AUTH_REQUIRED', ageMs: null, models: [{ id: 'x', state: 'LOADED' }] },
  ])('rejects invalid or non-contract payloads', value => expect(parse(value)).toBeNull());
  it('accepts empty, disabled, and exactly capped truncated inventories', () => {
    expect(parse({ ...base, models: [] })?.models).toEqual([]);
    expect(parse({ ...base, status: 'DISABLED', mode: 'DISABLED', reasonCode: 'DISABLED', ageMs: null, models: [] })?.status).toBe('DISABLED');
    expect(parse({ ...base, isTruncated: true, models: Array.from({ length: 128 }, (_, i) => ({ id: `model-${i}`, state: 'UNKNOWN' })) })?.models).toHaveLength(128);
  });
  it.each(['"\\ud800"', '"\\udc00"', '"\\ud800x"', '"\\udc00\\ud800"'])('rejects IDs with unpaired UTF-16 surrogates: %s', encodedId => {
    expect(parseRawId(encodedId)).toBeNull();
  });
  it('accepts valid escaped surrogate pairs and literal replacement characters', () => {
    expect(parseRawId('"\\ud800\\udc00"')?.models[0]?.id).toBe('𐀀');
    expect(parse({ ...base, models: [{ id: 'astral-😀-�', state: 'LOADED' }] })).not.toBeNull();
    expect(parse({ ...base, models: [{ id: String.raw`literal\\uD800`, state: 'LOADED' }] })).toBeNull();
  });
  it('preserves Unicode aliases while enforcing control and length caps', () => {
    expect(parse({ ...base, models: [{ id: 'mødel/聊天', state: 'UNLOADED' }] })).not.toBeNull();
    expect(parse({ ...base, models: [{ id: `x${'a'.repeat(128)}`, state: 'LOADED' }] })).toBeNull();
    expect(parse({ ...base, models: [{ id: 'bad\u200Bname', state: 'LOADED' }] })).toBeNull();
  });
});
