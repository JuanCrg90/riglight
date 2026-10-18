import { devices, defineConfig } from '@playwright/test';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = fileURLToPath(new URL('.', import.meta.url));
const repositoryRoot = resolve(webRoot, '..');
const binary = process.env.RIGLIGHT_BINARY
  ? resolve(process.env.RIGLIGHT_BINARY)
  : resolve(repositoryRoot, 'bin', 'riglight');
const baseURL = 'http://127.0.0.1:19090';
const gpuBaseURL = 'http://127.0.0.1:19091';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  reporter: 'list',
  use: { baseURL, trace: 'off', screenshot: 'off', video: 'off' },
  projects: [
    { name: 'chromium', testIgnore: '**/gpu.spec.ts', use: { ...devices['Desktop Chrome'] } },
    { name: 'webkit', testIgnore: '**/gpu.spec.ts', use: { ...devices['Desktop Safari'] } },
    { name: 'chromium-gpu', testMatch: '**/gpu.spec.ts', use: { ...devices['Desktop Chrome'], baseURL: gpuBaseURL } },
    { name: 'webkit-gpu', testMatch: '**/gpu.spec.ts', use: { ...devices['Desktop Safari'], baseURL: gpuBaseURL } },
  ],
  webServer: [
    {
      command: `"${binary}" --demo --listen 127.0.0.1:19090`,
      cwd: repositoryRoot,
      url: baseURL,
      timeout: 30_000,
      reuseExistingServer: false,
    },
    {
      command: `"${binary}" --demo --gpu --listen 127.0.0.1:19091`,
      cwd: repositoryRoot,
      url: gpuBaseURL,
      timeout: 30_000,
      reuseExistingServer: false,
    },
  ],
});
