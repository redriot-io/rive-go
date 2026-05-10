import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: ['*.spec.ts'],
  timeout: 60_000,          // per-test timeout (CDN load + render time)
  expect: { timeout: 30_000 },
  fullyParallel: false,     // sequential: avoids 6-WASM-instance "page unresponsive" problem
  workers: 1,
  retries: 0,
  reporter: process.env.CI ? [['github'], ['list']] : [['list']],

  use: {
    baseURL: 'http://127.0.0.1:5173',
    headless: true,
    bypassCSP: true,        // allow canvas.getImageData() cross-origin reads
  },

  // Start the repo-root static server before any test.
  // In CI the server is started by the workflow before playwright runs.
  webServer: {
    command: 'node server.js',
    url: 'http://127.0.0.1:5173',
    reuseExistingServer: true,   // always reuse if already up (CI starts it; local dev starts it)
    timeout: 30_000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
