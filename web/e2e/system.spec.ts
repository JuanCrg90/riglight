import { expect, test, type Page } from '@playwright/test';

const origin = 'http://127.0.0.1:19094';
const at = '2026-01-01T00:00:00Z';
type Frame = { event: 'snapshot' | 'gpus' | 'models' | 'containers' | 'system'; data: unknown };
function sample(ageMs = 0, sampledAt = at) {
  return {
    schemaVersion: 1, sampledAt, mode: 'DEMO',
    swap: { status: 'AVAILABLE', sampledAt, ageMs, usedBytes: 1073741824, totalBytes: 4294967296 },
    disk: { status: 'AVAILABLE', sampledAt, ageMs, usedBytes: 214748364800, availableBytes: 268435456000, totalBytes: 536870912000 },
    network: { status: 'AVAILABLE', sampledAt, ageMs, rxBytesPerSecond: 1048576, txBytesPerSecond: 262144 },
  };
}
function absent(status = 'UNAVAILABLE', sampledAt = at, mode = 'DEMO') {
  return {
    schemaVersion: 1, sampledAt, mode,
    swap: { status, sampledAt: null, ageMs: null, usedBytes: null, totalBytes: null },
    disk: { status, sampledAt: null, ageMs: null, usedBytes: null, availableBytes: null, totalBytes: null },
    network: { status, sampledAt: null, ageMs: null, rxBytesPerSecond: null, txBytesPerSecond: null },
  };
}
const cpu = (sampledAt = at) => ({
  schemaVersion: 1, sampledAt, mode: 'DEMO',
  cpu: { status: 'AVAILABLE', sampledAt, ageMs: 0, utilizationPercent: 24.5 },
  memory: { status: 'AVAILABLE', sampledAt, ageMs: 0, usedBytes: 4294967296, totalBytes: 8589934592 },
});
async function events(page: Page, ...frames: Frame[]) {
  await page.route('**/api/v1/events', route => route.fulfill({
    status: 200, headers: { 'content-type': 'text/event-stream; charset=utf-8', 'cache-control': 'no-store' },
    body: frames.map(frame => `event: ${frame.event}\ndata: ${JSON.stringify(frame.data)}\n\n`).join(''),
  }));
}
async function observe(page: Page) {
  const errors: string[] = []; const external: string[] = []; const snapshotFetches: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  page.on('request', request => {
    const url = new URL(request.url());
    if (url.origin !== origin) external.push(url.origin);
    if (url.pathname === '/api/v1/system') snapshotFetches.push(url.pathname);
  });
  await page.addInitScript(() => {
    const diagnostics = { instances: 0, events: [] as string[] };
    Object.assign(window, { __systemDiagnostics: diagnostics });
    const Native = window.EventSource;
    window.EventSource = new Proxy(Native, { construct(target, args) {
      const source = Reflect.construct(target, args) as EventSource; diagnostics.instances++;
      for (const name of ['snapshot', 'gpus', 'models', 'containers', 'system']) source.addEventListener(name, () => {
        if (!diagnostics.events.includes(name)) diagnostics.events.push(name);
      });
      return source;
    } });
  });
  return async () => {
    expect(errors).toEqual([]); expect(external).toEqual([]); expect(snapshotFetches).toEqual([]);
    expect(await page.evaluate(() => (window as Window & { __systemDiagnostics: { instances: number } }).__systemDiagnostics.instances)).toBe(1);
  };
}

test('combined demo streams five sources on one local EventSource with responsive read-only system detail', async ({ page, request }) => {
  const clean = await observe(page);
  await expect.poll(async () => {
    const response = await request.get('/api/v1/system'); expect(response.status()).toBe(200);
    const body = await response.json() as ReturnType<typeof sample>;
    return body.mode === 'DEMO' && body.swap.status === 'AVAILABLE' && body.disk.status === 'AVAILABLE' && body.network.status === 'AVAILABLE';
  }).toBe(true);
  await page.goto('/'); const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section).toBeVisible();
  for (const label of ['Swap used and total GiB', 'Root filesystem used available and total GiB', 'Selected uplinks RX and TX byte-rates']) {
    await expect(section.getByLabel(label)).not.toHaveText('—');
  }
  for (const name of ['Swap', 'Root filesystem', 'Selected uplinks']) await expect(section.getByRole('heading', { name, exact: true })).toBeVisible();
  for (const name of ['CPU utilization', 'GPU', 'llama-swap', 'Selected containers']) await expect(page.getByRole('region', { name, exact: true })).toBeVisible();
  await expect.poll(() => page.evaluate(() => (window as Window & { __systemDiagnostics: { events: string[] } }).__systemDiagnostics.events.length)).toBe(5);
  await expect(section.locator('a, button, input, select, textarea, img')).toHaveCount(0);
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }
  await clean();
});

test('no configured swap and idle traffic are genuine fresh zero observations', async ({ page }) => {
  const clean = await observe(page); const first = sample();
  first.swap.usedBytes = 0; first.swap.totalBytes = 0;
  first.network.rxBytesPerSecond = 0; first.network.txBytesPerSecond = 0;
  await events(page, { event: 'system', data: first }); await page.goto('/');
  const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByText(/No swap configured/i)).toBeVisible();
  await expect(section.getByLabel('Selected uplinks RX and TX byte-rates')).toHaveText('RX 0 B/s · TX 0 B/s');
  await expect(section.getByText(/Unavailable|Unsupported|Stale/i)).toHaveCount(0); await clean();
});

test('reserved root space is valid and available capacity is displayed separately', async ({ page }) => {
  const clean = await observe(page);
  await events(page, { event: 'system', data: sample() }); await page.goto('/');
  const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('200.0 / 250.0 / 500.0 GiB');
  await expect(section.getByText(/Available/).first()).toBeVisible(); await clean();
});

test('sensor failures remain independent without zero substitution', async ({ page }) => {
  const clean = await observe(page); const failed = absent();
  await events(page, { event: 'system', data: { ...sample(), swap: failed.swap, network: { ...failed.network, status: 'UNSUPPORTED' } } });
  await page.goto('/'); const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByText(/Unavailable/i).first()).toBeVisible();
  await expect(section.getByText(/Unsupported/i).first()).toBeVisible();
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('200.0 / 250.0 / 500.0 GiB');
  await expect(section.getByText(/No swap configured/i)).toHaveCount(0); await clean();
});

test('network can be stale while swap and root capacity remain current', async ({ page }) => {
  const clean = await observe(page); const first = sample(); first.network.ageMs = 6001;
  await events(page, { event: 'system', data: first }); await page.goto('/');
  const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByText(/Stale/i).first()).toBeVisible();
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('200.0 / 250.0 / 500.0 GiB');
  await expect(section.getByLabel('Selected uplinks RX and TX byte-rates')).toHaveText('—'); await clean();
});

test('retained timestamp replay and unrelated CPU frames cannot renew system detail', async ({ page }) => {
  const clean = await observe(page);
  await events(page,
    { event: 'system', data: sample(5000) },
    { event: 'system', data: absent('UNAVAILABLE', '2026-01-01T00:00:01Z') },
    { event: 'system', data: sample(0) },
    { event: 'snapshot', data: cpu('2026-01-01T00:00:02Z') },
  );
  await page.goto('/'); const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('200.0 / 250.0 / 500.0 GiB');
  await expect(section.getByText(/Stale/i).first()).toBeVisible({ timeout: 4000 });
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('—'); await clean();
});

test('shortened sensor expiries persist through a lower-age replay', async ({ page }) => {
  const clean = await observe(page);
  await events(page, { event: 'system', data: sample(0) }, { event: 'system', data: sample(5500) }, { event: 'system', data: sample(0) });
  await page.goto('/'); const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByText(/Stale/i).first()).toBeVisible({ timeout: 4000 });
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('—'); await clean();
});

test('disabled system frames remain valid independently of CPU mode', async ({ page }) => {
  const clean = await observe(page);
  await events(page, { event: 'snapshot', data: cpu() }, { event: 'system', data: absent('DISABLED', at, 'DISABLED') });
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'System detail', exact: true }).getByText(/Not enabled|disabled|opt.in/i).first()).toBeVisible();
  await clean();
});

test('invalid numeric bounds and extra metadata cannot replace valid readings', async ({ page }) => {
  const clean = await observe(page); const invalid = sample(); invalid.swap.totalBytes = 0;
  invalid.disk.usedBytes = 107374182400;
  const extra = sample(); extra.disk.usedBytes = 53687091200;
  await events(page,
    { event: 'system', data: sample() },
    { event: 'system', data: invalid },
    { event: 'system', data: { ...extra, hostname: 'SYNTHETIC_PRIVATE_SENTINEL' } },
  );
  await page.goto('/'); const section = page.getByRole('region', { name: 'System detail', exact: true });
  await expect(section.getByLabel('Root filesystem used available and total GiB')).toHaveText('200.0 / 250.0 / 500.0 GiB');
  await expect(section.getByText(/SYNTHETIC_PRIVATE_SENTINEL|No swap configured/)).toHaveCount(0); await clean();
});
