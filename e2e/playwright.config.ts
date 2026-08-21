import { defineConfig, devices } from '@playwright/test';

import { WEB_BASE } from './helpers/env';

// Points at whatever server/web instances are already running (see
// e2e/README.md for how to start them) — this suite deliberately does not
// spin up its own webServer, since the seeding helpers need a specific,
// already-known API base to hit with plain HTTP before any page ever opens.
export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  // Each test seeds its own fresh game via the REST API, so tests don't
  // share mutable state and can safely run concurrently.
  workers: process.env.CI ? 2 : 4,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list']],
  use: {
    baseURL: WEB_BASE,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
