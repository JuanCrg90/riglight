import { expect, test, type Page } from '@playwright/test';

const localOrigins = new Set(['http://127.0.0.1:19090', 'http://127.0.0.1:19091']);
const at = '2026-01-01T00:00:00Z';
const cpuSnapshot = (sampledAt = at) => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt: at, ageMs: 0, utilizationPercent: 24.5 },
  memory: { status: 'AVAILABLE', sampledAt: at, ageMs: 0, usedBytes: 4_294_967_296, totalBytes: 8_589_934_592 },
});
const gpuMetric = (field: string, value: number, ageMs = 0, sampledAt = at) => ({ status: 'AVAILABLE', sampledAt, ageMs, [field]: value });
const gpuOverview = (ageMs = 0, sampledAt = at, envelopeAt = at) => ({
  schemaVersion: 1, sampledAt: envelopeAt, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, isTruncated: false,
  devices: [{ index: 0,
    utilization: gpuMetric('percent', 42.5, ageMs, sampledAt),
    memory: { ...gpuMetric('usedBytes', 8 * 1024 ** 3, ageMs, sampledAt), totalBytes: 24 * 1024 ** 3 },
    temperature: gpuMetric('celsius', 55.2, ageMs, sampledAt),
    power: gpuMetric('watts', 125.5, ageMs, sampledAt),
  }],
});

function events(...frames: Array<{ event: string; data: unknown }>): string {
  return frames.map(({ event, data }) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`).join('');
}

async function serveSyntheticEvents(page: Page, body: string) {
  await page.route('**/api/v1/events', route => route.fulfill({
    status: 200,
    headers: { 'content-type': 'text/event-stream; charset=utf-8', 'cache-control': 'no-cache' },
    body,
  }));
}

test('GPU-enabled demo streams synthetic cards on the single local event connection', async ({ page }) => {
  const errors: string[] = [];
  const externalRequests: string[] = [];
  const eventStreams: string[] = [];
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => {
    const url = new URL(request.url());
    if (!localOrigins.has(url.origin)) externalRequests.push(url.origin);
    if (url.pathname === '/api/v1/events') eventStreams.push(url.href);
  });

  await page.goto('/');
  await expect(page.getByText('DEMO MODE')).toBeVisible();
  await expect(page.getByRole('region', { name: 'GPU' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'GPU 0' })).toBeVisible({ timeout: 15_000 });
  await expect(page.locator('.gpu-primary .gpu-reading strong')).toHaveText([/\d+(?:\.\d+)?%/, /\d+\.\d+ \/ \d+\.\d+ GiB/]);
  await expect(page.locator('.gpu-secondary .gpu-reading strong')).toHaveText([/\d+(?:\.\d+)? °C/, /\d+(?:\.\d+)? W/]);
  expect(await page.locator('.gpu-card').innerText()).not.toMatch(/NVIDIA|GeForce|UUID|serial|PCI|driver|model/i);
  await expect(page.getByRole('region', { name: 'CPU utilization' })).toBeVisible();

  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: 'Riglight home' })).toBeFocused();
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth), `overflow at ${width}px`).toBeLessThanOrEqual(width);
  }
  expect(eventStreams).toHaveLength(1);
  expect(errors).toEqual([]);
  expect(externalRequests).toEqual([]);
});

test('a disabled demo explains the opt-in without inventing GPU values', async ({ page }) => {
  await page.goto('http://127.0.0.1:19090/');
  await expect(page.getByText(/GPU monitoring is disabled.*--gpu/)).toBeVisible();
  await expect(page.getByRole('heading', { name: /^GPU 0$/ })).toHaveCount(0);
  await expect(page.getByText('0.0%', { exact: true })).toHaveCount(0);
});

test('shows partial sensor support and never substitutes zeroes', async ({ page }) => {
  const partial = JSON.parse(JSON.stringify(gpuOverview())) as { devices: Array<Record<string, unknown>> };
  partial.devices[0]!.utilization = { status: 'UNSUPPORTED', sampledAt: null, ageMs: null, percent: null };
  partial.devices[0]!.temperature = { status: 'UNAVAILABLE', sampledAt: null, ageMs: null, celsius: null };
  await serveSyntheticEvents(page, events(
    { event: 'snapshot', data: cpuSnapshot() },
    { event: 'gpus', data: partial },
  ));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'GPU 0' })).toBeVisible();
  await expect(page.getByText('UNSUPPORTED')).toBeVisible();
  await expect(page.getByText('UNAVAILABLE')).toBeVisible();
  await expect(page.getByText('—')).toHaveCount(2);
  await expect(page.getByText('8.0 / 24.0 GiB')).toBeVisible();
  await expect(page.getByText('0.0%', { exact: true })).toHaveCount(0);
});

test('reports a failed GPU query honestly without rendering a card', async ({ page }) => {
  const unavailable = { schemaVersion: 1, sampledAt: at, mode: 'DEMO', status: 'UNAVAILABLE', reasonCode: 'QUERY_FAILED', isTruncated: false, devices: [] };
  await serveSyntheticEvents(page, events(
    { event: 'snapshot', data: cpuSnapshot() },
    { event: 'gpus', data: unavailable },
  ));
  await page.goto('/');
  await expect(page.getByText('GPU readings are unavailable.')).toBeVisible();
  await expect(page.getByRole('heading', { name: /^GPU 0$/ })).toHaveCount(0);
});

test('cached GPU replay and CPU frames cannot renew per-sensor freshness', async ({ page }) => {
  const first = gpuOverview(5_200);
  const replay = gpuOverview(0, at, '2026-01-01T00:00:02Z');
  await serveSyntheticEvents(page, events(
    { event: 'gpus', data: first },
    { event: 'snapshot', data: cpuSnapshot() },
    { event: 'gpus', data: replay },
    { event: 'snapshot', data: cpuSnapshot('2026-01-01T00:00:03Z') },
  ));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'GPU 0' })).toBeVisible();
  await expect(page.locator('.gpu-reading-state', { hasText: 'Stale' }).first()).toBeVisible({ timeout: 5_000 });
  await expect(page.getByRole('region', { name: 'CPU utilization' }).getByText('Fresh')).toBeVisible();
});
