import { defineConfig } from '@playwright/test';

/**
 * A separate config from the e2e suite's, for a run with the opposite shape.
 *
 * The suite is many short tests, parallel, each with a thirty-second timeout
 * and a retry. This is one run that lasts as long as it was asked to, and every
 * one of those settings would be wrong for it: a timeout would kill it, a retry
 * would start the whole thing again, and a second worker would mean two fleets
 * competing for the same server with two separate reports.
 *
 * It is a Playwright config only for the runner's TypeScript and its browser
 * management. There are no fixtures in use and no `page` handed to the test —
 * the fleet launches and owns its own browser, because how many contexts exist
 * and when each one is created is the subject of the run rather than a detail
 * of it.
 */
export default defineConfig({
  testDir: '.',
  // The run ends when the fleet's deadline passes, not when a clock in the
  // runner says so. `ZOLIK_SOAK_FOR` is the only thing that decides how long
  // this takes.
  timeout: 0,
  globalTimeout: 0,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  expect: { timeout: 10_000 },
});
