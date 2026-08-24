import { expect, test } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';

/**
 * "The main page would be the waiting room and would give us status of the
 * players available" — the main menu shows a live count without requiring a
 * tap into a separate screen, and without itself joining the pool (that
 * stays an explicit "Find players" action — see WaitingStatusCard's own doc
 * comment for why the two are kept apart).
 */
test.describe('the main menu shows waiting-room status', () => {
  test('reports "no one waiting" until someone actually joins, then reflects them live', async ({
    browser,
    request,
  }) => {
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

      await homePage.goto('/');
      const card = homePage.getByTestId('home-waiting-status');
      await expect(card).toContainText('No one is waiting to play right now', { timeout: 10_000 });

      // The status card itself must never have joined the pool — only an
      // actual "Find players" tap does that.
      await homePage.reload();
      await expect(card).toContainText('No one is waiting to play right now', { timeout: 10_000 });

      // Someone else opens the waiting room for real.
      await waiterPage.goto('/lobby/waiting-room');
      await expect(waiterPage.getByTestId('waiting-room-status-open')).toBeVisible({
        timeout: 15_000,
      });

      // The home page's own poll (every 5s) picks it up without any
      // navigation or user action on that page.
      await expect(card).toContainText('1 player waiting to play', { timeout: 10_000 });
      await expect(card).toContainText(waiter.username);
    } finally {
      await homeCtx.close();
      await waiterCtx.close();
    }
  });
});
