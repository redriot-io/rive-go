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

  // webServer is intentionally absent from this config.
  // CI: the workflow starts `node server.js` before running playwright.
  // Local dev: run `npm run serve` in a separate terminal, then `npm test`.

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
