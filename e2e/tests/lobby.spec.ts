import { expect, test, type Page } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';

/**
 * The waiting room: a host picking up a real, independently-connected player
 * without a join code.
 *
 * This is the one flow in the whole suite that genuinely needs two separate
 * people — two browser contexts with two different sessions, exactly the
 * "two computers" shape the feature exists for. Everything else in this
 * repo's e2e suite is one person driving one game; this is the first spec
 * where a second, independent browser is not a nice-to-have but the actual
 * subject under test — a mock second client would only prove the host's
 * screen talks to itself.
 *
 * Becoming available is now a toggle on the main menu rather than a
 * dedicated screen — "Find players" opens the connection in place. See
 * WaitingStatusCard in app/index.tsx.
 */

async function becomeAvailable(page: Page) {
  await page.goto('/');
  await page.getByText('Find players', { exact: true }).click();
  await expect(page.getByTestId('waiting-status-open')).toBeVisible({ timeout: 15_000 });
}

/**
 * Opens a table as the host: pick a game, then "Open a table" rather than
 * "Play against bots".
 *
 * Choosing the game is now a step, because there is more than one. The screen
 * this replaces created a Žolíky game and nothing else, so a host had nothing
 * to choose — which is exactly the assumption the picker exists to remove.
 * Prší is used here only because *some* module has to be; nothing below cares
 * which, and the waiting room does not know a game exists.
 */
async function openATable(page: Page) {
  await page.goto('/lobby/games');
  await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 20_000 });
  await page.getByTestId('play-friends-prsi').click();
  await expect(page.getByTestId('table-screen')).toBeVisible({ timeout: 20_000 });
  await expect(page.getByTestId('waiting-players-panel')).toBeVisible({ timeout: 15_000 });
}

test.describe('the waiting room', () => {
  test('a waiting player is visible to the host, and inviting them seats and notifies them', async ({
    browser,
    request,
  }) => {
    const hostCtx = await browser.newContext();
    const waiterCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const waiterPage = await waiterCtx.newPage();

    try {
      const host = await loginAsFreshGuest(hostPage, request, `e2e-host-${Math.random().toString(36).slice(2, 8)}`);
      const waiter = await loginAsFreshGuest(
        waiterPage,
        request,
        `e2e-waiter-${Math.random().toString(36).slice(2, 8)}`,
      );

      // The waiting player becomes available first, independently of
      // anything the host does — that toggle *is* the whole action.
      await becomeAvailable(waiterPage);

      // The host opens a table, exactly as any host would: pick a game, then
      // "Open a table" rather than "Play against bots".
      await openATable(hostPage);

      // The waiting player shows up on the host's screen without either side
      // having exchanged a join code — the waiting-list poll (2s) is the
      // only thing standing between "they connected" and "the host sees
      // them", so this is the assertion that actually proves presence
      // propagated end to end through the server.
      const waiterRow = hostPage.getByTestId(`waiting-player-${waiter.userId}`);
      await expect(waiterRow).toBeVisible({ timeout: 10_000 });
      await expect(waiterRow).toContainText(waiter.username);

      await hostPage.getByTestId(`invite-${waiter.userId}`).click();

      // The invited player's own connection is what carries the
      // notification — nothing on their screen was clicked or polled by the
      // test to make this happen.
      await expect(waiterPage).toHaveURL(/\/lobby\/join/, { timeout: 10_000 });
      await expect(waiterPage.getByTestId('lobby-joined')).toBeVisible();
      await expect(waiterPage.getByTestId(`lobby-player-${waiter.userId}`)).toBeVisible();
      await expect(waiterPage.getByTestId(`lobby-player-${host.userId}`)).toBeVisible();

      // The host's own player list picks up the new seat on its next poll,
      // and the waiting panel drops their row — checked by id rather than
      // asserting the panel is entirely empty, since the pool is shared
      // global state and another spec's waiter may legitimately still be
      // in it under parallel workers.
      await expect(hostPage.getByText(waiter.username)).toBeVisible({ timeout: 10_000 });
      await expect(hostPage.getByTestId(`waiting-player-${waiter.userId}`)).toBeHidden({
        timeout: 10_000,
      });
    } finally {
      await hostCtx.close();
      await waiterCtx.close();
    }
  });

  // The server-side refusal to invite someone who is no longer actually
  // waiting is covered directly at the Go layer
  // (TestInviteIsRefusedWhenTheTargetHasStoppedWaiting in
  // server/internal/match/invite_test.go). What this spec checks is the half
  // that only a real browser can prove: that leaving genuinely disappears
  // the player from a host's live view within one poll, rather than leaving
  // a stale, inviteable-looking row behind — covering both ways a person
  // actually leaves: closing the tab, and tapping "Stop".
  test('a player who disconnects disappears from the host\'s view', async ({ browser, request }) => {
    const hostCtx = await browser.newContext();
    const waiterCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const waiterPage = await waiterCtx.newPage();

    try {
      await loginAsFreshGuest(hostPage, request, `e2e-host2-${Math.random().toString(36).slice(2, 8)}`);
      const waiter = await loginAsFreshGuest(
        waiterPage,
        request,
        `e2e-leaver-${Math.random().toString(36).slice(2, 8)}`,
      );

      await becomeAvailable(waiterPage);

      await openATable(hostPage);
      await expect(
        hostPage.getByTestId(`waiting-player-${waiter.userId}`),
      ).toBeVisible({ timeout: 10_000 });

      // Closing the tab is the real-world equivalent of navigating away
      // without an explicit action, and is exactly what tears down the
      // WebSocket the waiting room is keyed on.
      await waiterCtx.close();

      // The host's snapshot is stale until its next poll; the server itself
      // must be the one that actually refuses this, which is what the
      // "still there" assertion below checks for — a client-only guard
      // would just let the stale row keep showing an inviteable player
      // forever.
      await expect(hostPage.getByTestId(`waiting-player-${waiter.userId}`)).toBeHidden({
        timeout: 10_000,
      });
    } finally {
      await hostCtx.close();
    }
  });

  test('tapping "Stop" removes a player from the host\'s view without closing anything', async ({
    browser,
    request,
  }) => {
    const hostCtx = await browser.newContext();
    const waiterCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const waiterPage = await waiterCtx.newPage();

    try {
      await loginAsFreshGuest(hostPage, request, `e2e-host3-${Math.random().toString(36).slice(2, 8)}`);
      const waiter = await loginAsFreshGuest(
        waiterPage,
        request,
        `e2e-stopper-${Math.random().toString(36).slice(2, 8)}`,
      );

      await becomeAvailable(waiterPage);

      await openATable(hostPage);
      await expect(
        hostPage.getByTestId(`waiting-player-${waiter.userId}`),
      ).toBeVisible({ timeout: 10_000 });

      // The explicit, in-place way to stop being available — no navigation,
      // no closed tab. Checked via the idle-state button reappearing on the
      // waiter's own screen (not an absolute "no one waiting" claim, which
      // the shared, global pool can't guarantee under parallel workers) and
      // via their row disappearing from the host's panel.
      await waiterPage.getByText('Stop', { exact: true }).click();
      await expect(waiterPage.getByText('Find players', { exact: true })).toBeVisible({
        timeout: 15_000,
      });

      await expect(hostPage.getByTestId(`waiting-player-${waiter.userId}`)).toBeHidden({
        timeout: 10_000,
      });
    } finally {
      await hostCtx.close();
      await waiterCtx.close();
    }
  });
});
