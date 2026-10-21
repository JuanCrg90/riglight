import { expect, test, type Page } from '@playwright/test';

const origin = 'http://127.0.0.1:19095';
const model = 'model.example-model';
const docker = 'docker.example-api';
const catalog = {
  schemaVersion: 1, mode: 'DEMO', sources: [
    { id: 'llama.proxy', label: 'Proxy diagnostics', kind: 'LLAMA_PROXY' },
    { id: model, label: 'example-model', kind: 'MODEL' },
    { id: docker, label: 'example-api', kind: 'DOCKER' },
  ],
};
const line = (seq: number, text: string, sourceId = model) => ({
  schemaVersion: 1, mode: 'DEMO', sourceId, seq, text, stream: 'OUTPUT', truncated: false,
});
const status = (state: string, reasonCode: string | null, sourceId = model) => ({
  schemaVersion: 1, mode: 'DEMO', sourceId, state, reasonCode,
});
type Source = EventTarget & { closed: boolean; closes: number; onerror: ((event: Event) => void) | null };
type Harness = Window & { __riglightLogSources: Source[] };

async function fakeLogs(page: Page, body: unknown = catalog) {
  await page.route('**/api/v1/log-sources', route => route.fulfill({ json: body }));
  await page.addInitScript(() => {
    const sources: Source[] = [];
    Object.assign(window, { __riglightLogSources: sources });
    const Native = window.EventSource;
    window.EventSource = new Proxy(Native, { construct(target, args) {
      if (!String(args[0]).startsWith('/api/v1/logs/')) return Reflect.construct(target, args);
      const source = Object.assign(new EventTarget(), {
        closed: false, closes: 0, readyState: 1, onerror: null as ((event: Event) => void) | null,
        close() { this.closed = true; this.closes++; this.readyState = 2; },
      });
      sources.push(source);
      return source;
    } });
  });
}
async function emit(page: Page, event: 'log' | 'log-status', data: unknown, index = 0) {
  await page.evaluate(({ event, data, index }) => {
    (window as Harness).__riglightLogSources[index]!.dispatchEvent(new MessageEvent(event, { data: JSON.stringify(data) }));
  }, { event, data, index });
}
async function open(page: Page) {
  await page.getByRole('button', { name: /open logs inspector/i }).click();
  await expect(page.getByRole('combobox', { name: 'Log source', exact: true })).toBeVisible();
}
async function start(page: Page, sourceId = model) {
  await page.getByRole('combobox', { name: 'Log source', exact: true }).selectOption(sourceId);
  await page.getByRole('button', { name: 'Start', exact: true }).click();
  await page.getByRole('button', { name: 'Start log stream', exact: true }).click();
}
async function sourceCount(page: Page) {
  return page.evaluate(() => (window as Harness).__riglightLogSources.length);
}
function observe(page: Page) {
  const errors: string[] = []; const external: string[] = []; const logRequests: string[] = []; const catalogRequests: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  page.on('request', request => {
    const url = new URL(request.url());
    if (url.origin !== origin) external.push(url.origin);
    if (url.pathname.startsWith('/api/v1/logs/')) logRequests.push(url.pathname);
    if (url.pathname === '/api/v1/log-sources') catalogRequests.push(url.pathname);
  });
  return { errors, external, logRequests, catalogRequests, clean() { expect(errors).toEqual([]); expect(external).toEqual([]); } };
}

test('real demo logs open only manually and preserve the five-source metric stream', async ({ page }) => {
  const seen = observe(page);
  await page.addInitScript(() => {
    const telemetry = { instances: 0, events: [] as string[] };
    Object.assign(window, { __riglightTelemetry: telemetry });
    const Native = window.EventSource;
    window.EventSource = new Proxy(Native, { construct(target, args) {
      const source = Reflect.construct(target, args) as EventSource;
      if (String(args[0]) === '/api/v1/events') {
        telemetry.instances++;
        for (const name of ['snapshot', 'gpus', 'models', 'containers', 'system']) source.addEventListener(name, () => {
          if (!telemetry.events.includes(name)) telemetry.events.push(name);
        });
      }
      return source;
    } });
  });
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'CPU utilization', exact: true })).toBeVisible();
  expect(seen.catalogRequests).toHaveLength(0); expect(seen.logRequests).toHaveLength(0);
  await open(page); expect(seen.logRequests).toHaveLength(0);
  await start(page);
  await expect(page.getByLabel('Log output')).toContainText('demo model log');
  await expect.poll(() => page.evaluate(() => (window as Window & { __riglightTelemetry: { events: string[] } }).__riglightTelemetry.events.length)).toBe(5);
  expect(await page.evaluate(() => (window as Window & { __riglightTelemetry: { instances: number } }).__riglightTelemetry.instances)).toBe(1);
  expect(seen.logRequests).toHaveLength(1);
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }
  await page.getByRole('button', { name: 'Close', exact: true }).click();
  await expect(page.getByText('demo model log', { exact: true })).toHaveCount(0);
  seen.clean();
});

test('disabled catalog is honest and cannot open a log stream', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page, { schemaVersion: 1, mode: 'DISABLED', sources: [] });
  await page.goto('/'); await open(page);
  await expect(page.getByText(/not enabled|disabled/i).last()).toBeVisible();
  expect(await sourceCount(page)).toBe(0); expect(seen.logRequests).toHaveLength(0); seen.clean();
});

test('hostile output stays escaped, filtering stays local and closing discards text', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await page.goto('/'); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  const hostile = '<img src="https://external.example/leak" onerror="alert(1)"><script>unsafe()</script>';
  await emit(page, 'log', line(1, hostile)); await emit(page, 'log', line(2, 'SYNTHETIC_SECOND_LINE'));
  const output = page.getByLabel('Log output'); await expect(output).toContainText(hostile);
  await expect(output.locator('img, script, a, iframe')).toHaveCount(0);
  const requests = seen.catalogRequests.length;
  await page.getByLabel('Filter lines', { exact: true }).fill('SECOND');
  await expect(output).toContainText('SYNTHETIC_SECOND_LINE'); await expect(output).not.toContainText(hostile);
  expect(seen.catalogRequests).toHaveLength(requests);
  await page.getByRole('button', { name: 'Close', exact: true }).click();
  await expect(page.getByText('SYNTHETIC_SECOND_LINE')).toHaveCount(0);
  expect(await page.evaluate(() => (window as Harness).__riglightLogSources[0]!.closed)).toBe(true); seen.clean();
});

test('browser buffer obeys both line and UTF-8 byte ceilings', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await page.goto('/'); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await page.evaluate(() => {
    const source = (window as Harness).__riglightLogSources[0]!;
    for (let seq = 1; seq <= 240; seq++) source.dispatchEvent(new MessageEvent('log', { data: JSON.stringify({
      schemaVersion: 1, mode: 'DEMO', sourceId: 'model.example-model', seq,
      text: `line-${seq} ` + '😀'.repeat(700), stream: 'OUTPUT', truncated: false,
    }) }));
  });
  const rows = page.getByLabel('Log output').locator('pre');
  await expect(rows.last()).toContainText('line-240 ');
  const bound = await rows.evaluateAll(elements => ({ count: elements.length, bytes: elements.reduce((sum, element) => sum + new TextEncoder().encode(element.textContent ?? '').byteLength, 0) }));
  expect(bound.count).toBeGreaterThan(0); expect(bound.count).toBeLessThanOrEqual(200); expect(bound.bytes).toBeLessThanOrEqual(256 * 1024);
  await expect(rows.first()).not.toHaveText(/^line-1\s/); seen.clean();
});

test('pause retains text, resume creates a new session, source switches reject old callbacks', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await page.goto('/'); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log', line(1, 'SYNTHETIC_OLD_SESSION'));
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_OLD_SESSION');
  const priorCatalogs = seen.catalogRequests.length;
  await page.getByRole('button', { name: 'Resume', exact: true }).click();
  await page.getByRole('button', { name: 'Start log stream', exact: true }).click();
  await expect.poll(() => sourceCount(page)).toBe(2);
  expect(seen.catalogRequests.length).toBeGreaterThan(priorCatalogs);
  await emit(page, 'log', line(1, 'SYNTHETIC_RESUMED_SESSION'), 1);
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_OLD_SESSION');
  await expect(page.getByLabel('Log output')).toContainText('SYNTHETIC_RESUMED_SESSION');
  await page.getByRole('combobox', { name: 'Log source', exact: true }).selectOption(docker);
  await expect(page.getByLabel('Log output')).toBeEmpty();
  await emit(page, 'log', line(2, 'SYNTHETIC_STALE_CALLBACK'), 1);
  await expect(page.getByLabel('Log output')).not.toContainText('SYNTHETIC_STALE_CALLBACK');
  expect(await sourceCount(page)).toBe(2); seen.clean();
});

test('terminal status explicitly closes the EventSource without retries', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await page.goto('/'); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log-status', status('LIMIT_REACHED', 'BYTE_LIMIT'));
  expect(await page.evaluate(() => (window as Harness).__riglightLogSources[0]!.closed)).toBe(true);
  await expect(page.getByText(/BYTE_LIMIT|byte.limit/i)).toBeVisible();
  await emit(page, 'log', line(1, 'SYNTHETIC_AFTER_END'));
  await expect(page.getByLabel('Log output')).not.toContainText('SYNTHETIC_AFTER_END');
  expect(await sourceCount(page)).toBe(1); seen.clean();
});

test('foreign mode or source frames are never displayed', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page); await page.goto('/'); await open(page); await start(page);
  await expect.poll(() => sourceCount(page)).toBe(1);
  await emit(page, 'log', { ...line(1, 'SYNTHETIC_FOREIGN_MODE'), mode: 'LIVE' });
  await expect(page.getByLabel('Log output')).not.toContainText('SYNTHETIC_FOREIGN_MODE');
  expect(await page.evaluate(() => (window as Harness).__riglightLogSources[0]!.closed)).toBe(true);
  await start(page); await expect.poll(() => sourceCount(page)).toBe(2);
  await emit(page, 'log', line(1, 'SYNTHETIC_FOREIGN_SOURCE', docker), 1);
  await expect(page.getByLabel('Log output')).not.toContainText('SYNTHETIC_FOREIGN_SOURCE');
  expect(await page.evaluate(() => (window as Harness).__riglightLogSources[1]!.closed)).toBe(true); seen.clean();
});

test('oversized catalogs cannot instantiate a provider connection', async ({ page }) => {
  const seen = observe(page); await fakeLogs(page, { schemaVersion: 1, mode: 'DEMO', sources: Array.from({ length: 34 }, (_, index) => ({ id: `model.example${index}`, label: 'Example', kind: 'MODEL' })) });
  await page.goto('/'); await open(page);
  await expect(page.getByText(/unavailable|invalid|too large/i).last()).toBeVisible();
  expect(await sourceCount(page)).toBe(0); seen.clean();
});

test('log API retains admission and input guards', async ({ request }) => {
  for (const path of ['/api/v1/log-sources', '/api/v1/logs/model.example-model']) {
    expect((await request.get(path, { headers: { Origin: 'https://foreign.example' } })).status()).toBe(403);
    expect((await request.get(path, { headers: { Host: 'foreign.example' } })).status()).toBe(403);
    expect((await request.post(path)).status()).toBe(405);
    expect((await request.get(path + '?x=1')).status()).toBe(400);
    expect((await request.get(path, { headers: { 'Last-Event-ID': 'old-session' } })).status()).toBe(400);
  }
  expect((await request.get('/api/v1/logs/model.not-selected')).status()).toBe(404);
  expect((await request.get('/api/v1/logs/model.%65xample-model')).status()).toBe(400);
});
