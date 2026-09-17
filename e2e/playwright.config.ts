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
    // The board animates — cards deal in, pile tops flip over — and every
    // animation in the client goes still under prefers-reduced-motion (see
    // `src/hooks/useReducedMotion.ts`). Asking the browser for stillness
    // makes the suite deterministic the same way it makes the board calm for
    // a person who asked for it: elements are born already in their final
    // position, so a test's first measurement can never race an entrance.
    contextOptions: { reducedMotion: 'reduce' },
    // Slow the whole run down so a person can watch it, which is the only way
    // to check the half of a drag no assertion describes — where the card is
    // relative to the hand carrying it, frame by frame.
    //
    //   ZOLIK_E2E_SLOWMO=250 npx playwright test tests/hand-order.spec.ts --headed
    //
    // Nothing in the suite depends on it: the waits that make a drag work
    // (see helpers/drag.ts) are fixed and already generous, and this only adds
    // to them. Unset, it is exactly the run CI does.
    launchOptions: { slowMo: Number(process.env.ZOLIK_E2E_SLOWMO ?? 0) || undefined },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
