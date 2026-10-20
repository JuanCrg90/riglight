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
const modelsBaseURL = 'http://127.0.0.1:19092';
const containersBaseURL = 'http://127.0.0.1:19093';
const systemBaseURL = 'http://127.0.0.1:19094';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  reporter: 'list',
  use: { baseURL, trace: 'off', screenshot: 'off', video: 'off' },
  projects: [
    { name: 'chromium', testIgnore: ['**/gpu.spec.ts', '**/models.spec.ts', '**/containers.spec.ts', '**/system.spec.ts'], use: { ...devices['Desktop Chrome'] } },
    { name: 'webkit', testIgnore: ['**/gpu.spec.ts', '**/models.spec.ts', '**/containers.spec.ts', '**/system.spec.ts'], use: { ...devices['Desktop Safari'] } },
    { name: 'chromium-gpu', testMatch: '**/gpu.spec.ts', use: { ...devices['Desktop Chrome'], baseURL: gpuBaseURL } },
    { name: 'webkit-gpu', testMatch: '**/gpu.spec.ts', use: { ...devices['Desktop Safari'], baseURL: gpuBaseURL } },
    { name: 'chromium-models', testMatch: '**/models.spec.ts', workers: 2, use: { ...devices['Desktop Chrome'], baseURL: modelsBaseURL } },
    { name: 'webkit-models', testMatch: '**/models.spec.ts', workers: 2, use: { ...devices['Desktop Safari'], baseURL: modelsBaseURL } },
    { name: 'chromium-containers', testMatch: '**/containers.spec.ts', workers: 2, use: { ...devices['Desktop Chrome'], baseURL: containersBaseURL } },
    { name: 'webkit-containers', testMatch: '**/containers.spec.ts', workers: 2, use: { ...devices['Desktop Safari'], baseURL: containersBaseURL } },
    { name: 'chromium-system', testMatch: '**/system.spec.ts', workers: 2, use: { ...devices['Desktop Chrome'], baseURL: systemBaseURL } },
    { name: 'webkit-system', testMatch: '**/system.spec.ts', workers: 2, use: { ...devices['Desktop Safari'], baseURL: systemBaseURL } },
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
    {
      command: `"${binary}" --demo --gpu --llama-swap --listen 127.0.0.1:19092`,
      cwd: repositoryRoot,
      url: modelsBaseURL,
      timeout: 30_000,
      reuseExistingServer: false,
    },
    {
      command: `"${binary}" --demo --gpu --llama-swap --docker --listen 127.0.0.1:19093`,
      cwd: repositoryRoot,
      url: containersBaseURL,
      timeout: 30_000,
      reuseExistingServer: false,
    },
    {
      command: `"${binary}" --demo --gpu --llama-swap --docker --system-details --listen 127.0.0.1:19094`,
      cwd: repositoryRoot,
      url: systemBaseURL,
      timeout: 30_000,
      reuseExistingServer: false,
    },
  ],
});
