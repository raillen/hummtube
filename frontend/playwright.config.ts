import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const runtimeDirectory = mkdtempSync(join(tmpdir(), 'nanosuite-e2e-'));
const xdgEnvironment = `XDG_DATA_HOME=${runtimeDirectory}/data XDG_CONFIG_HOME=${runtimeDirectory}/config XDG_CACHE_HOME=${runtimeDirectory}/cache XDG_STATE_HOME=${runtimeDirectory}/state`;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  timeout: 45000,
  retries: process.env.CI ? 2 : 1,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:8999',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'nanotube',
      use: { ...devices['Desktop Chrome'] },
      testIgnore: ['**/iptv.spec.ts', '**/nanomusic.spec.ts'],
    },
    {
      name: 'nanoiptv',
      use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:9000' },
      testMatch: '**/iptv.spec.ts',
    },
    {
      name: 'nanomusic',
      use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:9001' },
      testMatch: '**/nanomusic.spec.ts',
    },
  ],
  webServer: [
    {
      command: `${xdgEnvironment} ../bin/nanotube-web --server --port 8999 --no-browser`,
      url: 'http://127.0.0.1:8999/api/health',
      reuseExistingServer: !process.env.CI,
      stdout: 'pipe', stderr: 'pipe', timeout: 30000,
    },
    {
      command: `${xdgEnvironment} ../bin/nanoiptv --server --port 9000 --no-browser`,
      url: 'http://127.0.0.1:9000/api/health',
      reuseExistingServer: !process.env.CI,
      stdout: 'pipe', stderr: 'pipe', timeout: 30000,
    },
    {
      command: `${xdgEnvironment} ../bin/nanomusic --server --port 9001 --no-browser`,
      url: 'http://127.0.0.1:9001/api/health',
      reuseExistingServer: !process.env.CI,
      stdout: 'pipe', stderr: 'pipe', timeout: 30000,
    },
  ],
});
