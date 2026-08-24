import { expect, test } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';

/**
 * "The main page would be the waiting room and would give us status of the
 * players available" — the main menu shows a live count on its own, without
 * itself joining the pool (that stays an explicit "Find players" tap, which
 * now toggles in place rather than navigating — see WaitingStatusCard's own
 * doc comment in app/index.tsx for why the two data sources are kept apart).
 */
test.describe('the main menu shows waiting-room status', () => {
  test('a new waiter is absent until they connect, then appears live', async ({ browser, request }) => {
    const homeCtx = await browser.newContext();
    const waiterCtx = await browser.newContext();
    const homePage = await homeCtx.newPage();
    const waiterPage = await waiterCtx.newPage();

    try {
      await loginAsFreshGuest(homePage, request, `e2e-home-${Math.random().toString(36).slice(2, 8)}`);
      const waiter = await loginAsFreshGuest(
        waiterPage,
        request,
        `e2e-status-${Math.random().toString(36).slice(2, 8)}`,
      );

      // The waiting pool is a shared, global resource — other specs (and
      // other runs of this one, under parallel workers) can have their own
      // waiters in it at the same time. Asserting an absolute "no one is
      // waiting" here would be true in isolation but flaky under any
      // concurrency, so every assertion below is scoped to this one waiter
      // by name rather than to the pool's total size.
      await homePage.goto('/');
      const card = homePage.getByTestId('home-waiting-status');
      await expect(card).not.toContainText(waiter.username, { timeout: 10_000 });

      // The status card itself must never have joined the pool — only an
      // actual "Find players" tap does that.
      await homePage.reload();
      await expect(card).not.toContainText(waiter.username, { timeout: 10_000 });

      // The other player becomes available for real.
      await waiterPage.goto('/');
      await waiterPage.getByText('Find players', { exact: true }).click();
      await expect(waiterPage.getByTestId('waiting-status-open')).toBeVisible({
        timeout: 15_000,
      });

      // The home page's own poll (every 5s) picks it up without any
      // navigation or user action on that page.
      await expect(card).toContainText(waiter.username, { timeout: 10_000 });
    } finally {
      await homeCtx.close();
      await waiterCtx.close();
    }
  });
});
