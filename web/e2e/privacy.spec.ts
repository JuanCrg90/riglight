import { expect, test } from '@playwright/test';

test('production demo responses keep browser security headers and API responses private', async ({ request }) => {
  const page = await request.get('/');
  expect(page.ok()).toBeTruthy();
  const pageHeaders = page.headers();
  expect(pageHeaders['content-security-policy']).toContain("frame-ancestors 'none'");
  expect(pageHeaders['x-content-type-options']).toBe('nosniff');
  expect(pageHeaders['referrer-policy']).toBe('no-referrer');

  const api = await request.get('/api/v1/snapshot');
  expect(api.ok()).toBeTruthy();
  expect(api.headers()['cache-control']).toContain('no-store');
});

test('production API rejects hostile, null, and development origins plus foreign hosts', async ({ request }) => {
  for (const origin of ['https://attacker.invalid', 'null', 'http://127.0.0.1:5173']) {
    const response = await request.get('/api/v1/snapshot', { headers: { Origin: origin } });
    expect(response.ok(), `production accepted Origin: ${origin}`).toBe(false);
  }

  const foreignHost = await request.get('/api/v1/snapshot', { headers: { Host: 'attacker.invalid' } });
  expect(foreignHost.ok()).toBe(false);
});
