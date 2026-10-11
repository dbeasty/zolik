import { expect, test } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';

/**
 * Each game's page is that game's waiting room: who is waiting to play it,
 * live, without the page itself joining the pool — that stays an explicit
 * "Make me available to play" tap (see `WaitingCard`). The main menu carries
 * the count on each game's row.
 *
 * Prší throughout, because *some* game has to be named; the split between
 * games is its own test at the bottom.
 */
const GAME = '/lobby/games?moduleId=prsi';

test.describe("a game's page shows its waiting room", () => {
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
      await homePage.goto(GAME);
      const card = homePage.getByTestId('home-waiting-status');
      await expect(card).not.toContainText(waiter.username, { timeout: 10_000 });

      // The status card itself must never have joined the pool — only an
      // actual availability tap does that.
      await homePage.reload();
      await expect(card).not.toContainText(waiter.username, { timeout: 10_000 });

      // The other player becomes available for real.
      await waiterPage.goto(GAME);
      await waiterPage.getByText('Make me available to play', { exact: true }).click();
      await expect(waiterPage.getByTestId('waiting-status-open')).toBeVisible({
        timeout: 15_000,
      });

      // The game page's own poll (every 5s) picks it up without any
      // navigation or user action on that page.
      await expect(card).toContainText(waiter.username, { timeout: 10_000 });
    } finally {
      await homeCtx.close();
      await waiterCtx.close();
    }
  });

  /**
   * Making yourself available used to blank the roster out and replace it
   * with a bare count — "2 other players are also waiting" — which is the
   * one moment a person most wants to know *who*, since these are the people
   * they are about to be seated with. This asserts the names survive the
   * toggle, on the screen of somebody who has taken it.
   */
  test('someone who is waiting still sees who else is waiting, by name', async ({
    browser,
    request,
  }) => {
    const oneCtx = await browser.newContext();
    const twoCtx = await browser.newContext();
    const onePage = await oneCtx.newPage();
    const twoPage = await twoCtx.newPage();

    try {
      const one = await loginAsFreshGuest(
        onePage,
        request,
        `e2e-pair1-${Math.random().toString(36).slice(2, 8)}`,
      );
      const two = await loginAsFreshGuest(
        twoPage,
        request,
        `e2e-pair2-${Math.random().toString(36).slice(2, 8)}`,
      );

      for (const page of [onePage, twoPage]) {
        await page.goto(GAME);
        await page.getByText('Make me available to play', { exact: true }).click();
        await expect(page.getByTestId('waiting-status-open')).toBeVisible({ timeout: 15_000 });
      }

      // Each sees the other by name — and never a row for themselves, which
      // is what makes "N other players" an honest count rather than one that
      // silently includes the reader.
      await expect(onePage.getByTestId(`home-waiting-player-${two.userId}`)).toContainText(
        two.username,
        { timeout: 15_000 },
      );
      await expect(onePage.getByTestId(`home-waiting-player-${one.userId}`)).toBeHidden();

      await expect(twoPage.getByTestId(`home-waiting-player-${one.userId}`)).toContainText(
        one.username,
        { timeout: 15_000 },
      );
      await expect(twoPage.getByTestId(`home-waiting-player-${two.userId}`)).toBeHidden();
    } finally {
      await oneCtx.close();
      await twoCtx.close();
    }
  });

  /**
   * The pool is split by game: somebody waiting for Canasta is not offered to
   * a Prší table, and the main menu counts them under Canasta only.
   */
  test('a player waiting for one game is listed and counted under that game only', async ({
    browser,
    request,
  }) => {
    const watchCtx = await browser.newContext();
    const waiterCtx = await browser.newContext();
    const watchPage = await watchCtx.newPage();
    const waiterPage = await waiterCtx.newPage();

    try {
      await loginAsFreshGuest(watchPage, request, `e2e-watch-${Math.random().toString(36).slice(2, 8)}`);
      const waiter = await loginAsFreshGuest(
        waiterPage,
        request,
        `e2e-canasta-${Math.random().toString(36).slice(2, 8)}`,
      );

      // In through the main menu, as a player would, so there is a menu to
      // go back to below.
      await waiterPage.goto('/');
      await waiterPage.getByTestId('game-canasta').click();
      await waiterPage.getByText('Make me available to play', { exact: true }).click();
      await expect(waiterPage.getByTestId('waiting-status-open')).toBeVisible({ timeout: 15_000 });

      // Canasta's page lists them; Prší's never does.
      await watchPage.goto('/lobby/games?moduleId=canasta');
      await expect(watchPage.getByTestId(`home-waiting-player-${waiter.userId}`)).toBeVisible({
        timeout: 15_000,
      });
      await watchPage.goto(GAME);
      await expect(watchPage.getByTestId('home-waiting-status')).toBeVisible({ timeout: 15_000 });
      await expect(watchPage.getByTestId(`home-waiting-player-${waiter.userId}`)).toHaveCount(0);

      // The main menu counts them on Canasta's row.
      await watchPage.goto('/');
      await expect(watchPage.getByTestId('picker-canasta-waiting')).toBeVisible({ timeout: 15_000 });

      // Leaving the page does not stop them waiting: the menu says so, and
      // Stop there takes them out.
      // The header's Back arrow (nav-arrows.spec.ts owns what it means); it used to
      // be labelled with the screen it returned to.
      await waiterPage.getByRole('button', { name: 'Back', exact: true }).click();
      const strip = waiterPage.getByTestId('availability-strip');
      await expect(strip).toContainText('Canasta', { timeout: 10_000 });
      await waiterPage.getByTestId('availability-strip-stop').click();
      await expect(strip).toHaveCount(0);
      await watchPage.reload();
      await expect(watchPage.getByTestId('games-list')).toBeVisible({ timeout: 15_000 });
      await expect(watchPage.getByTestId('picker-canasta-waiting')).toHaveCount(0, { timeout: 15_000 });
    } finally {
      await watchCtx.close();
      await waiterCtx.close();
    }
  });
});
