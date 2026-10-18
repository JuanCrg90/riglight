import { expect, test } from '@playwright/test';

const widths = [320, 768, 1024, 1440];

test('receives labeled demo metrics over SSE without browser errors or external requests', async ({ page }) => {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const externalRequests: string[] = [];
  page.on('console', (message) => { if (message.type() === 'error') consoleErrors.push(message.text()); });
  page.on('pageerror', (error) => pageErrors.push(error.message));
  page.on('request', (request) => {
    if (new URL(request.url()).origin !== 'http://127.0.0.1:19090') externalRequests.push(new URL(request.url()).origin);
  });

  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'System overview' })).toBeVisible();
  await expect(page.getByText('DEMO MODE')).toBeVisible();
  await expect(page.getByText(/GPU monitoring is disabled/)).toBeVisible();
  await expect(page.getByText('Connected', { exact: true })).toBeVisible();
  await expect(page.getByRole('img', { name: 'CPU utilization history' })).toBeVisible();
  await expect(page.getByRole('img', { name: 'Memory utilization history' })).toBeVisible();

  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 });
    const documentWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(documentWidth, `horizontal overflow at ${width}px`).toBeLessThanOrEqual(width);
  }
  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
  expect(externalRequests).toEqual([]);
});

test('native EventSource reconnects after a transient transport failure', async ({ page }) => {
  let attempts = 0;
  await page.route('**/api/v1/events', async (route) => {
    attempts += 1;
    if (attempts === 1) await route.abort('failed');
    else await route.continue();
  });

  await page.goto('/');
  await expect(page.getByText('Reconnecting', { exact: true })).toBeVisible();
  await expect(page.getByText('DEMO MODE')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Connected', { exact: true })).toBeVisible();
  expect(attempts).toBeGreaterThanOrEqual(2);
});
