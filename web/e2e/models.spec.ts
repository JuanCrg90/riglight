import { expect, test, type Page } from '@playwright/test';

const localOrigin = 'http://127.0.0.1:19092';
const at = '2026-01-01T00:00:00Z';

type ModelRow = { id: string; state: 'LOADED' | 'UNLOADED' | 'UNKNOWN' };
type ModelEvent = {
  schemaVersion: 1;
  sampledAt: string;
  mode: 'DEMO' | 'LIVE' | 'DISABLED';
  status: 'DISABLED' | 'NOT_READY' | 'AVAILABLE' | 'UNAVAILABLE' | 'UNSUPPORTED';
  reasonCode: string | null;
  ageMs: number | null;
  isTruncated: boolean;
  models: ModelRow[];
};

type Frame = { event: 'snapshot' | 'gpus' | 'models'; data: unknown };

type Diagnostics = { consoleErrors: string[]; pageErrors: string[]; externalRequests: string[]; eventStreams: string[] };

const inventory = (models: ModelRow[], ageMs = 0, isTruncated = false, sampledAt = at): ModelEvent => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, ageMs, isTruncated, models,
});

const overview = (status: ModelEvent['status'], reasonCode: string | null, sampledAt = at): ModelEvent => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO', status, reasonCode, ageMs: null, isTruncated: false, models: [],
});

const cpuSnapshot = (sampledAt = at) => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt, ageMs: 0, utilizationPercent: 24.5 },
  memory: { status: 'AVAILABLE', sampledAt, ageMs: 0, usedBytes: 4_294_967_296, totalBytes: 8_589_934_592 },
});

const gpuOverview = (sampledAt = at) => {
  const metric = (field: string, value: number) => ({ status: 'AVAILABLE', sampledAt, ageMs: 0, [field]: value });
  return {
    schemaVersion: 1, sampledAt, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
    devices: [{ index: 0,
      utilization: metric('percent', 42.5),
      memory: { ...metric('usedBytes', 8 * 1024 ** 3), totalBytes: 24 * 1024 ** 3 },
      temperature: metric('celsius', 55.2),
      power: metric('watts', 125.5),
    }],
  };
};

function encodeEvents(...frames: Frame[]): string {
  return frames.map(({ event, data }) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`).join('');
}

async function observePage(page: Page): Promise<Diagnostics> {
  const diagnostics: Diagnostics = { consoleErrors: [], pageErrors: [], externalRequests: [], eventStreams: [] };
  page.on('console', message => { if (message.type() === 'error') diagnostics.consoleErrors.push(message.text()); });
  page.on('pageerror', error => diagnostics.pageErrors.push(error.message));
  page.on('request', request => {
    const url = new URL(request.url());
    if (url.origin !== localOrigin) diagnostics.externalRequests.push(url.origin);
    if (url.pathname === '/api/v1/events') diagnostics.eventStreams.push(url.href);
  });
  await page.addInitScript(() => {
    const trackedWindow = window as Window & { __riglightEventSources?: EventSource[] };
    trackedWindow.__riglightEventSources = [];
    const NativeEventSource = window.EventSource;
    const TrackedEventSource = new Proxy(NativeEventSource, {
      construct(target, args) {
        const source = Reflect.construct(target, args) as EventSource;
        trackedWindow.__riglightEventSources?.push(source);
        return source;
      },
    });
    Object.defineProperty(window, 'EventSource', { configurable: true, writable: true, value: TrackedEventSource });
  });
  return diagnostics;
}

async function serveSyntheticEvents(page: Page, ...frames: Frame[]): Promise<void> {
  await page.route('**/api/v1/events', route => route.fulfill({
    status: 200,
    headers: { 'content-type': 'text/event-stream; charset=utf-8', 'cache-control': 'no-cache' },
    body: encodeEvents(...frames),
  }));
}

async function expectCleanLocalPage(page: Page, diagnostics: Diagnostics): Promise<void> {
  expect(diagnostics.consoleErrors).toEqual([]);
  expect(diagnostics.pageErrors).toEqual([]);
  expect(diagnostics.externalRequests).toEqual([]);
  expect(await page.evaluate(() => (window as Window & { __riglightEventSources?: EventSource[] }).__riglightEventSources?.length ?? 0)).toBe(1);
}

test('demo serves CPU, GPU, and model inventory over one local EventSource', async ({ page }) => {
  const diagnostics = await observePage(page);
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'CPU utilization' })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Memory used' })).toBeVisible();
  await expect(page.getByRole('region', { name: 'GPU' })).toBeVisible();
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models).toBeVisible();
  await expect(models.getByText('example-chat', { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(models.getByText('example-embed', { exact: true })).toBeVisible();
  await expect(models.getByText('Loaded', { exact: true })).toBeVisible();
  await expect(models.getByText('Unloaded', { exact: true })).toBeVisible();
  await expect(models.getByText(/Loaded does not imply ready/)).toBeVisible();
  await expect(models.locator('a, button, input, select, textarea')).toHaveCount(0);

  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: 'Riglight home' })).toBeFocused();
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth), `overflow at ${width}px`).toBeLessThanOrEqual(width);
  }
  expect(diagnostics.eventStreams).toHaveLength(1);
  await expectCleanLocalPage(page, diagnostics);
});

test('an empty model inventory is distinct from pending or disabled', async ({ page }) => {
  const diagnostics = await observePage(page);
  await serveSyntheticEvents(page, { event: 'models', data: inventory([]) });
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText('No models are listed in the current inventory.')).toBeVisible();
  await expect(models.getByText(/Waiting|disabled/i)).toHaveCount(0);
  await expectCleanLocalPage(page, diagnostics);
});

test('unknown state and hostile-looking Unicode aliases remain escaped plain text', async ({ page }) => {
  const diagnostics = await observePage(page);
  const alias = '<img src=x onerror=alert(1)>-聊天-😀';
  await serveSyntheticEvents(page, { event: 'models', data: inventory([{ id: alias, state: 'UNKNOWN' }]) });
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText(alias, { exact: true })).toBeVisible();
  await expect(models.getByText('Unknown', { exact: true })).toBeVisible();
  await expect(models.getByText('Unloaded', { exact: true })).toHaveCount(0);
  await expect(models.locator('img, a, button')).toHaveCount(0);
  await expectCleanLocalPage(page, diagnostics);
});

test('unavailable authentication state uses safe generic guidance', async ({ page }) => {
  const diagnostics = await observePage(page);
  await serveSyntheticEvents(page, { event: 'models', data: overview('UNAVAILABLE', 'AUTH_REQUIRED') });
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText(/requires authentication/i)).toBeVisible();
  await expect(models.getByText(/token|password|8080|9090|\/v1\/models/i)).toHaveCount(0);
  await expectCleanLocalPage(page, diagnostics);
});

test('unsupported inventory is explained without exposing upstream details', async ({ page }) => {
  const diagnostics = await observePage(page);
  await serveSyntheticEvents(page, { event: 'models', data: overview('UNSUPPORTED', 'API_UNSUPPORTED') });
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText(/does not support model inventory/i)).toBeVisible();
  await expectCleanLocalPage(page, diagnostics);
});

test('stale inventory hides aliases rather than presenting old states as current', async ({ page }) => {
  const diagnostics = await observePage(page);
  await serveSyntheticEvents(page, { event: 'models', data: inventory([{ id: 'example-chat', state: 'LOADED' }], 6_001) });
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText(/inventory is stale/i)).toBeVisible();
  await expect(models.getByText('example-chat', { exact: true })).toHaveCount(0);
  await expectCleanLocalPage(page, diagnostics);
});

test('unavailable timestamp replay stays stale and CPU/GPU frames cannot renew it', async ({ page }) => {
  const diagnostics = await observePage(page);
  const first = inventory([{ id: 'example-chat', state: 'LOADED' }], 5_000);
  const unavailableB = overview('UNAVAILABLE', 'CONNECT_FAILED', '2026-01-01T00:00:01Z');
  const replayA = inventory([{ id: 'example-chat', state: 'LOADED' }], 0);
  await serveSyntheticEvents(page,
    { event: 'models', data: first },
    { event: 'models', data: unavailableB },
    { event: 'models', data: replayA },
    { event: 'snapshot', data: cpuSnapshot('2026-01-01T00:00:02Z') },
    { event: 'gpus', data: gpuOverview('2026-01-01T00:00:02Z') },
  );
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText('example-chat', { exact: true })).toBeVisible();
  await page.waitForTimeout(1_200);
  await expect(models.getByText(/inventory is stale/i)).toBeVisible();
  await expect(models.getByText('example-chat', { exact: true })).toHaveCount(0);
  await expectCleanLocalPage(page, diagnostics);
});

test('a NOT_READY timestamp cannot become fresh when replayed as AVAILABLE', async ({ page }) => {
  const diagnostics = await observePage(page);
  const pending = overview('NOT_READY', null);
  await serveSyntheticEvents(page,
    { event: 'models', data: pending },
    { event: 'models', data: inventory([{ id: 'example-chat', state: 'LOADED' }]) },
  );
  await page.goto('/');
  const models = page.getByRole('region', { name: 'llama-swap' });
  await expect(models.getByText(/inventory is stale/i)).toBeVisible();
  await expect(models.getByText('example-chat', { exact: true })).toHaveCount(0);
  await expectCleanLocalPage(page, diagnostics);
});
