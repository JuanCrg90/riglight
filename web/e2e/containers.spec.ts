import { expect, test, type Page } from '@playwright/test';
import type { ContainerOverview } from '../src/containers';

const origin = 'http://127.0.0.1:19093';
const at = '2026-01-01T00:00:00Z';
type Frame = { event: 'snapshot' | 'gpus' | 'models' | 'containers'; data: unknown };

function inventory(ageMs = 0, sampledAt = at, count = 1): ContainerOverview {
  return {
    schemaVersion: 1, sampledAt, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, ageMs,
    containers: Array.from({ length: count }, (_, index) => ({
      alias: `demo-service-${String(index).padStart(2, '0')}`,
      metadata: { status: 'AVAILABLE', sampledAt, ageMs, state: 'RUNNING', health: 'HEALTHY', restartCount: 2 },
      cpu: { status: 'AVAILABLE', sampledAt, ageMs, percent: 125 },
      memory: { status: 'AVAILABLE', sampledAt, ageMs, usedBytes: 268435456, totalBytes: 1073741824 },
    })),
  };
}
function unavailable(status: ContainerOverview['status'], reasonCode: ContainerOverview['reasonCode'], mode: ContainerOverview['mode'] = 'DEMO'): ContainerOverview {
  return { schemaVersion: 1, sampledAt: at, mode, status, reasonCode, ageMs: null, containers: [] };
}
const cpu = (sampledAt = at) => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt, ageMs: 0, utilizationPercent: 24.5 },
  memory: { status: 'AVAILABLE', sampledAt, ageMs: 0, usedBytes: 4294967296, totalBytes: 8589934592 },
});
async function events(page: Page, ...frames: Frame[]) {
  await page.route('**/api/v1/events', route => route.fulfill({
    status: 200,
    headers: { 'content-type': 'text/event-stream; charset=utf-8', 'cache-control': 'no-store' },
    body: frames.map(frame => `event: ${frame.event}\ndata: ${JSON.stringify(frame.data)}\n\n`).join(''),
  }));
}
async function observe(page: Page) {
  const errors: string[] = []; const external: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  page.on('request', request => { if (new URL(request.url()).origin !== origin) external.push(new URL(request.url()).origin); });
  await page.addInitScript(() => {
    const diagnostics = { instances: 0, events: [] as string[] };
    Object.assign(window, { __containersDiagnostics: diagnostics });
    const Native = window.EventSource;
    window.EventSource = new Proxy(Native, { construct(target, args) {
      const source = Reflect.construct(target, args) as EventSource;
      diagnostics.instances++;
      for (const name of ['snapshot', 'gpus', 'models', 'containers']) source.addEventListener(name, () => {
        if (!diagnostics.events.includes(name)) diagnostics.events.push(name);
      });
      return source;
    } });
  });
  return async () => {
    expect(errors).toEqual([]); expect(external).toEqual([]);
    expect(await page.evaluate(() => (window as Window & { __containersDiagnostics: { instances: number } }).__containersDiagnostics.instances)).toBe(1);
  };
}

test('demo shows selected containers alongside other sources on one local EventSource', async ({ page, request }) => {
  const clean = await observe(page);
  const response = await request.get('/api/v1/containers');
  expect(response.status()).toBe(200);
  const sample = await response.json() as ContainerOverview;
  expect(sample.mode).toBe('DEMO'); expect(sample.status).toBe('AVAILABLE');
  expect(sample.containers.length).toBeGreaterThan(0);
  await page.goto('/');
  const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByRole('heading', { name: sample.containers[0]!.alias, exact: true })).toBeVisible();
  await expect(page.getByRole('region', { name: 'CPU utilization' })).toBeVisible();
  await expect(page.getByRole('region', { name: 'GPU', exact: true })).toBeVisible();
  await expect(page.getByRole('region', { name: 'llama-swap' })).toBeVisible();
  await expect(section.getByText(/one-core basis/)).toBeVisible();
  await expect(section.locator('a, button, input, select, textarea, img')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => (window as Window & { __containersDiagnostics: { events: string[] } }).__containersDiagnostics.events.length)).toBe(4);
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }
  await clean();
});

test('disabled data is accepted independently of the active CPU mode', async ({ page }) => {
  const clean = await observe(page);
  await events(page, { event: 'snapshot', data: cpu() }, { event: 'containers', data: unavailable('DISABLED', 'DISABLED', 'DISABLED') });
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Selected containers' }).getByText(/Not enabled/)).toBeVisible();
  await clean();
});

test('missing containers and unsupported sensors are not presented as stale or zero', async ({ page }) => {
  const clean = await observe(page); const sample = inventory();
  sample.containers[0]!.metadata = { status: 'NOT_FOUND', sampledAt: null, ageMs: null, state: null, health: null, restartCount: null };
  sample.containers[0]!.cpu = { status: 'NOT_READY', sampledAt: null, ageMs: null, percent: null };
  sample.containers[0]!.memory = { status: 'UNSUPPORTED', sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null };
  await events(page, { event: 'containers', data: sample }); await page.goto('/');
  const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByText('NOT FOUND', { exact: true }).first()).toBeVisible();
  await expect(section.getByText('NOT READY', { exact: true })).toBeVisible();
  await expect(section.getByText('UNSUPPORTED', { exact: true })).toBeVisible();
  await expect(section.getByText('Stale', { exact: true })).toHaveCount(0);
  await clean();
});

test('metadata can expire while CPU and memory remain fresh', async ({ page }) => {
  const clean = await observe(page); const sample = inventory(); sample.containers[0]!.metadata.ageMs = 15001;
  await events(page, { event: 'containers', data: sample }); await page.goto('/');
  const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByText('125.0%', { exact: true })).toBeVisible();
  await expect(section.getByText('Stale', { exact: true }).first()).toBeVisible();
  await expect(section.getByText(/RUNNING|HEALTHY|Restarts: 2/)).toHaveCount(0);
  await clean();
});

test('all sixteen rows expire without other sources or a cached replay renewing them', async ({ page }) => {
  const clean = await observe(page);
  await events(page,
    { event: 'containers', data: inventory(14000, at, 16) },
    { event: 'containers', data: inventory(0, at, 16) },
    { event: 'snapshot', data: cpu('2026-01-01T00:00:01Z') },
  );
  await page.goto('/'); const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByRole('heading', { level: 3 })).toHaveCount(16);
  await expect(section.getByText(/overview is stale/i)).toBeVisible({ timeout: 4000 });
  await expect(section.getByRole('heading', { level: 3 })).toHaveCount(0); await clean();
});

test('a shortened expiry cannot be extended by a lower-age replay', async ({ page }) => {
  const clean = await observe(page);
  await events(page,
    { event: 'containers', data: inventory(0) },
    { event: 'containers', data: inventory(14500) },
    { event: 'containers', data: inventory(0) },
  );
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Selected containers' }).getByText(/overview is stale/i)).toBeVisible({ timeout: 4000 });
  await clean();
});

test('a pending timestamp cannot become fresh through an available replay', async ({ page }) => {
  const clean = await observe(page);
  await events(page,
    { event: 'containers', data: unavailable('NOT_READY', null) },
    { event: 'containers', data: inventory() },
  );
  await page.goto('/'); const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByText(/overview is stale/i)).toBeVisible();
  await expect(section.getByRole('heading', { level: 3 })).toHaveCount(0); await clean();
});

test('auth and unsupported proxy states use safe generic guidance', async ({ page }) => {
  const clean = await observe(page);
  await events(page, { event: 'containers', data: unavailable('UNAVAILABLE', 'AUTH_REQUIRED') });
  await page.goto('/'); const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByText(/proxy requires authorization/i)).toBeVisible();
  await events(page, { event: 'containers', data: unavailable('UNSUPPORTED', 'API_UNSUPPORTED') });
  await page.reload(); await expect(section.getByText(/does not support this API/i)).toBeVisible(); await clean();
});

test('invalid aliases and extra upstream fields cannot replace valid projected data', async ({ page }) => {
  const clean = await observe(page); const invalid = inventory();
  invalid.containers[0]!.alias = '<img src=x onerror=alert(1)>';
  await events(page,
    { event: 'containers', data: inventory() },
    { event: 'containers', data: invalid },
    { event: 'containers', data: { ...inventory(), environment: ['SYNTHETIC_PRIVATE_SENTINEL'] } },
  );
  await page.goto('/'); const section = page.getByRole('region', { name: 'Selected containers' });
  await expect(section.getByRole('heading', { name: 'demo-service-00' })).toBeVisible();
  await expect(section.locator('img')).toHaveCount(0);
  await expect(section.getByText(/SYNTHETIC_PRIVATE_SENTINEL|onerror/)).toHaveCount(0); await clean();
});
