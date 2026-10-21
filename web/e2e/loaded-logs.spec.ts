import { expect, test, type Page } from '@playwright/test';

const origin = 'http://127.0.0.1:19096';
const id = 'model.loaded';
const catalog = { schemaVersion: 1, mode: 'DEMO', sources: [
  { id: 'llama.proxy', label: 'Proxy diagnostics', kind: 'LLAMA_PROXY' },
  { id, label: 'Currently loaded model', kind: 'LOADED_MODEL' },
] };
const line = (seq: number, text: string) => ({ schemaVersion: 1, mode: 'DEMO', sourceId: id, seq, text, stream: 'OUTPUT', truncated: false });
const target = (revision: number, model: string | null, state = model === null ? 'WAITING' : 'FOLLOWING') => ({ schemaVersion: 1, mode: 'DEMO', sourceId: id, revision, state, model });
type Source = EventTarget & { closed: boolean; onerror: ((event: Event) => void) | null };
type Harness = Window & { __loadedLogSources: Source[]; __loadedMetricStreams: number };

async function fakeLogs(page: Page) {
  await page.route('**/api/v1/log-sources', route => route.fulfill({ json: catalog }));
  await page.addInitScript(() => {
    const sources: Source[] = [];
    Object.assign(window, { __loadedLogSources: sources, __loadedMetricStreams: 0 });
    const Native = window.EventSource;
    window.EventSource = new Proxy(Native, { construct(ctor, args) {
      if (String(args[0]) === '/api/v1/events') (window as Harness).__loadedMetricStreams++;
      if (!String(args[0]).startsWith('/api/v1/logs/')) return Reflect.construct(ctor, args);
      const source = Object.assign(new EventTarget(), {
        closed: false, readyState: 1, onerror: null as ((event: Event) => void) | null,
        close() { this.closed = true; this.readyState = 2; },
      });
      sources.push(source); return source;
    } });
  });
}
async function emit(page: Page, event: 'log' | 'log-target', data: unknown, index = 0) {
  await page.evaluate(({ event, data, index }) => (window as Harness).__loadedLogSources[index]!.dispatchEvent(new MessageEvent(event, { data: JSON.stringify(data) })), { event, data, index });
}
async function sourceCount(page: Page) { return page.evaluate(() => (window as Harness).__loadedLogSources.length); }
async function closed(page: Page, index = 0) { return page.evaluate(index => (window as Harness).__loadedLogSources[index]!.closed, index); }
async function open(page: Page) {
  await page.goto('/');
  await page.getByRole('button', { name: 'Open logs inspector', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Log source', exact: true })).toHaveValue(id);
}
async function start(page: Page, resume = false) {
  await page.getByRole('button', { name: resume ? 'Resume' : 'Start', exact: true }).click();
  await page.getByRole('button', { name: 'Start log stream', exact: true }).click();
}
function observe(page: Page) {
  const errors: string[] = []; const external: string[] = []; const logs: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  page.on('request', request => {
    const url = new URL(request.url()); if (url.origin !== origin) external.push(url.origin);
    if (url.pathname.startsWith('/api/v1/logs/')) logs.push(url.pathname);
  });
  return { logs, clean() { expect(errors).toEqual([]); expect(external).toEqual([]); } };
}

// Only synthetic demo/test-owned streams; never a workstation log provider.
test('real automatic demo source is manual, defaults to current model and uses synthetic output', async ({ page }) => {
  const seen = observe(page); await page.goto('/'); expect(seen.logs).toHaveLength(0);
  await open(page); expect(seen.logs).toHaveLength(0);
  await start(page);
  const inspector = page.getByRole('region', { name: 'Logs inspector', exact: true });
  await expect(inspector).toContainText('example-chat');
  await expect.poll(() => page.getByLabel('Log output').locator('pre').count()).toBeGreaterThan(0);
  expect(seen.logs).toEqual(['/api/v1/logs/model.loaded']);
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(inspector).toContainText('example-chat'); seen.clean();
});

test('A to waiting to B clears old text/filter without replacing the log or metric connection', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log-target', target(1, 'example-alpha.v1'));
  await emit(page, 'log', line(1, 'SYNTHETIC_ALPHA_FIRST'));
  const output = page.getByLabel('Log output'); await expect(output).toContainText('SYNTHETIC_ALPHA_FIRST');
  await page.getByLabel('Filter lines', { exact: true }).fill('ALPHA');
  await emit(page, 'log-target', target(2, null));
  await expect(output).toBeEmpty(); await expect(page.getByLabel('Filter lines', { exact: true })).toHaveValue('');
  await emit(page, 'log-target', target(3, 'example-beta')); await emit(page, 'log', line(2, 'SYNTHETIC_BETA_SECOND'));
  await expect(output).toContainText('SYNTHETIC_BETA_SECOND'); await expect(output).not.toContainText('ALPHA');
  await expect(page.getByRole('region', { name: 'Logs inspector', exact: true })).toContainText('example-beta');
  expect(await sourceCount(page)).toBe(1); expect(await closed(page)).toBe(false);
  expect(await page.evaluate(() => (window as Harness).__loadedMetricStreams)).toBe(1); seen.clean();
});

test('ambiguity, stale inventory and unsupported targets discard prior output without guessing', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log-target', target(1, 'example-alpha')); await emit(page, 'log', line(1, 'SYNTHETIC_PRIOR_TARGET'));
  for (const [revision, state] of [[2, 'AMBIGUOUS'], [3, 'UNAVAILABLE'], [4, 'UNSUPPORTED']] as const) {
    await emit(page, 'log-target', target(revision, null, state));
    await expect(page.getByLabel('Log output')).toBeEmpty(); expect(await closed(page)).toBe(false);
  }
  await emit(page, 'log-target', target(5, 'example-beta')); await emit(page, 'log', line(2, 'SYNTHETIC_RECOVERED_TARGET'));
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_RECOVERED_TARGET'); expect(await sourceCount(page)).toBe(1); seen.clean();
});

test('lines before target and line-sequence replay across target changes fail closed', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log', line(1, 'SYNTHETIC_UNBOUND_LINE'));
  await expect(page.getByLabel('Log output')).not.toContainText('SYNTHETIC_UNBOUND_LINE'); expect(await closed(page)).toBe(true);
  await start(page); await expect.poll(() => sourceCount(page)).toBe(2);
  await emit(page, 'log-target', target(1, 'example-alpha'), 1); await emit(page, 'log', line(1, 'SYNTHETIC_FIRST'), 1);
  await emit(page, 'log-target', target(2, 'example-beta'), 1); await emit(page, 'log', line(1, 'SYNTHETIC_SEQUENCE_REPLAY'), 1);
  await expect(page.getByLabel('Log output')).toBeEmpty(); expect(await closed(page, 1)).toBe(true); seen.clean();
});

test('pause retains its target label; resumed new target clears and old callbacks cannot contaminate it', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log-target', target(1, 'example-alpha')); await emit(page, 'log', line(1, 'SYNTHETIC_PAUSED_ALPHA'));
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Logs inspector', exact: true })).toContainText('example-alpha');
  await start(page, true); await expect.poll(() => sourceCount(page)).toBe(2);
  await emit(page, 'log-target', target(1, 'example-beta'), 1); await emit(page, 'log', line(1, 'SYNTHETIC_RESUMED_BETA'), 1);
  await emit(page, 'log-target', target(2, 'example-stale')); await emit(page, 'log', line(2, 'SYNTHETIC_STALE_CALLBACK'));
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_RESUMED_BETA');
  await expect(page.getByLabel('Log output')).not.toContainText('ALPHA'); await expect(page.getByLabel('Log output')).not.toContainText('STALE');
  await page.getByRole('button', { name: 'Pause', exact: true }).click(); await start(page, true);
  await expect.poll(() => sourceCount(page)).toBe(3);
  await emit(page, 'log-target', target(1, 'example-beta'), 2); await emit(page, 'log', line(1, 'SYNTHETIC_SAME_BETA_NEW_SESSION'), 2);
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_RESUMED_BETA');
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_SAME_BETA_NEW_SESSION'); seen.clean();
});

test('malformed, foreign and replayed target envelopes close without reconnecting', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await open(page);
  const bad = [
    { ...target(1, 'example-alpha'), mode: 'LIVE' }, { ...target(1, 'example-alpha'), sourceId: 'model.other' },
    target(2, 'example-alpha'), target(1, null, 'FOLLOWING'), target(1, 'example-alpha', 'WAITING'),
    { ...target(1, 'example-alpha'), extra: true }, target(1, '../unsafe'),
    ...['proxy', 'upstream', 'http'].map(model => target(1, model)),
  ];
  for (let index = 0; index < bad.length; index++) {
    await start(page); await expect.poll(() => sourceCount(page)).toBe(index + 1);
    await emit(page, 'log-target', bad[index], index); expect(await closed(page, index)).toBe(true);
    await emit(page, 'log', line(1, 'SYNTHETIC_AFTER_INVALID'), index);
    await expect(page.getByLabel('Log output')).not.toContainText('SYNTHETIC_AFTER_INVALID');
  }
  await start(page); await expect.poll(() => sourceCount(page)).toBe(bad.length + 1);
  await emit(page, 'log-target', target(1, 'example-alpha'), bad.length);
  await emit(page, 'log-target', target(1, 'example-alpha'), bad.length);
  expect(await closed(page, bad.length)).toBe(true); expect(await sourceCount(page)).toBe(bad.length + 1); seen.clean();
});
