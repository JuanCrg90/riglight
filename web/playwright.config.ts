import { devices, defineConfig } from '@playwright/test';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = fileURLToPath(new URL('.', import.meta.url));
const repositoryRoot = resolve(webRoot, '..');
const binary = process.env.RIGLIGHT_BINARY
  ? resolve(process.env.RIGLIGHT_BINARY)
  : resolve(repositoryRoot, 'bin', 'riglight');
const baseURL = 'http://127.0.0.1:19090';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  reporter: 'list',
  use: { baseURL, trace: 'retain-on-failure' },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: {
    command: `"${binary}" --demo --listen 127.0.0.1:19090`,
    cwd: repositoryRoot,
    url: baseURL,
    timeout: 30_000,
    reuseExistingServer: false,
  },
});
