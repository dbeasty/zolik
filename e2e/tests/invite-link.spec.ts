import { expect, test, type Page } from '@playwright/test';

import { WEB_BASE } from '../helpers/env';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * Sharing a table by URL — the Zoom-shaped half of opening a table.
 *
 * The waiting room (lobby.spec.ts) fills a table by having the host pick
 * somebody who is already in the app and already available. This is the other
 * direction, and the one that reaches people who are not here yet: the host
 * copies a link, sends it however they send things, and whoever opens it ends
 * up at the table. Nothing is typed and nothing is read out.
 *
 * Like lobby.spec.ts, this genuinely needs two independent browser contexts.
 * A link is only a link if it works somewhere other than where it was made,
 * and the interesting failures all live in the gap — a URL built on the API's
 * port instead of the client's, or an invite dropped by the sign-in the
 * recipient has to do first. A single-context test would pass through both.
 */

async function openATable(page: Page) {
  await page.goto('/lobby/games');
  await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 20_000 });
  await page.getByTestId('play-friends-prsi').click();
  await expect(page.getByTestId('table-screen')).toBeVisible({ timeout: 20_000 });
}

/** The link exactly as a host would copy it off their screen. */
async function inviteLink(page: Page): Promise<string> {
  const url = page.getByTestId('invite-url');
  await expect(url).toBeVisible({ timeout: 15_000 });
  return ((await url.textContent()) ?? '').trim();
}

test.describe('inviting by link', () => {
  test('a host shares a link, and a stranger who opens it lands at the table', async ({
    browser,
    request,
  }) => {
    const hostCtx = await browser.newContext();
    // Deliberately no session seeded here: an invite almost always arrives on
    // a device that has never played, and that is the path worth proving.
    const guestCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const guestPage = await guestCtx.newPage();

    try {
      const host = await loginAsFreshGuest(
        hostPage,
        request,
        `e2e-linkhost-${Math.random().toString(36).slice(2, 8)}`,
      );

      await openATable(hostPage);
      const link = await inviteLink(hostPage);

      // The single most likely way for this feature to break silently: in
      // development the client is served on :8114 while the server's
      // configured public base names :8090, where no client answers. A link
      // to the API's port looks perfectly well-formed on the host's screen
      // and is a blank page for everybody else.
      expect(link).toContain('/join/');
      expect(link.startsWith(WEB_BASE)).toBeTruthy();

      // The code is still offered alongside it — the link is an addition, not
      // a replacement, and a host on the phone still needs six characters.
      await expect(hostPage.getByTestId('table-join-code')).toBeVisible();

      // Now the part that is the whole feature: somebody who has never opened
      // this app before follows the URL out of a chat.
      await guestPage.goto(link);

      // They are asked the one thing Zoom asks — what to call yourself — and
      // nothing else. No code to type, no account.
      await expect(guestPage.getByPlaceholder('Display name')).toBeVisible({ timeout: 20_000 });
      const guestName = `e2e-invitee-${Math.random().toString(36).slice(2, 8)}`;
      await guestPage.getByPlaceholder('Display name').fill(guestName);
      await guestPage.getByText('Continue', { exact: true }).click();

      // And they are at the host's table, not the game picker the guest
      // screen normally lands on — the invite survived the sign-in detour.
      await expect(guestPage.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });

      // Proven from the host's side too, which is the only view that can
      // distinguish "the invitee's screen says it worked" from "a seat was
      // actually taken at this table".
      await expect(hostPage.getByText(guestName, { exact: false })).toBeVisible({
        timeout: 15_000,
      });

      // And the host can now deal, which is the point of having filled the
      // table at all.
      await hostPage.getByTestId('table-start').click();
      await expect(hostPage).toHaveURL(/\/match\//, { timeout: 20_000 });
      // The invitee is carried into the same match by their own polling,
      // without touching anything.
      await expect(guestPage).toHaveURL(/\/match\//, { timeout: 20_000 });

      expect(host.userId).toBeTruthy();
    } finally {
      await hostCtx.close();
      await guestCtx.close();
    }
  });

  test('a player who is already signed in is seated straight away', async ({ browser, request }) => {
    // The other arrival: the link reaches somebody with the app already set
    // up. They should not be asked anything at all.
    const hostCtx = await browser.newContext();
    const friendCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const friendPage = await friendCtx.newPage();

    try {
      await loginAsFreshGuest(
        hostPage,
        request,
        `e2e-linkhost2-${Math.random().toString(36).slice(2, 8)}`,
      );
      const friend = await loginAsFreshGuest(
        friendPage,
        request,
        `e2e-linkfriend-${Math.random().toString(36).slice(2, 8)}`,
      );

      await openATable(hostPage);
      const link = await inviteLink(hostPage);

      await friendPage.goto(link);
      await expect(friendPage.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });
      await expect(hostPage.getByTestId(`seated-${friend.userId}`)).toBeVisible({
        timeout: 15_000,
      });

      // Following the same link twice is a no-op rather than a second seat:
      // people re-open links, and the server's join is idempotent.
      await friendPage.goto(link);
      await expect(friendPage.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });
      await expect(hostPage.getByTestId('table-screen')).toBeVisible();
      await expect(hostPage.getByText(/^Players \(2\)$/)).toBeVisible({ timeout: 15_000 });
    } finally {
      await hostCtx.close();
      await friendCtx.close();
    }
  });

  test('a link to a table that no longer takes players says so', async ({ browser, request }) => {
    // Links sit in chats. By the time somebody taps one the table may have
    // been dealt, and "that could not be joined" with a way onward is the
    // difference between a dead end and a small disappointment.
    const hostCtx = await browser.newContext();
    const lateCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const latePage = await lateCtx.newPage();

    try {
      await loginAsFreshGuest(
        hostPage,
        request,
        `e2e-linkhost3-${Math.random().toString(36).slice(2, 8)}`,
      );
      await loginAsFreshGuest(
        latePage,
        request,
        `e2e-latecomer-${Math.random().toString(36).slice(2, 8)}`,
      );

      await openATable(hostPage);
      const link = await inviteLink(hostPage);

      // The host fills the table with a bot and deals before the link is
      // followed.
      await hostPage.getByTestId('table-add-bot').click();
      await hostPage.getByTestId('table-start').click();
      await expect(hostPage).toHaveURL(/\/match\//, { timeout: 20_000 });

      await latePage.goto(link);
      await expect(latePage.getByTestId('invite-error')).toBeVisible({ timeout: 20_000 });
      // Worded, not a status code — and with somewhere to go next.
      await expect(latePage.getByTestId('invite-error')).not.toBeEmpty();
      await expect(latePage.getByTestId('invite-error-join')).toBeVisible();
    } finally {
      await hostCtx.close();
      await lateCtx.close();
    }
  });
});
