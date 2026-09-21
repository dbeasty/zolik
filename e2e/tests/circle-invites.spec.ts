import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { loginAsFreshGuest, type GuestIdentity } from '../helpers/login';

/**
 * Game circle invites — telling the people you play with that you opened a
 * table, wherever they are in the app.
 *
 * Two independent browser contexts again, because the whole feature is the gap
 * between them: one person opens a table, and a different person, on a
 * different screen, is shown a banner they can join from. Everything is driven
 * through the screens a person uses — the friend link, the circle screen, the
 * banner — and the only API calls read a friend code the recipient would have
 * shared by hand.
 */

const suffix = () => Math.random().toString(36).slice(2, 8);

async function friendCode(request: APIRequestContext, who: GuestIdentity): Promise<string> {
  const res = await request.get(`${API_BASE}/notify/me`, {
    headers: { Authorization: `Bearer ${who.accessToken}` },
  });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).friendCode;
}

/** The host follows the friend's link and confirms, as they would from a chat. */
async function addByFriendLink(page: Page, code: string, friendName: string) {
  await page.goto(`/add/${code}`);
  await expect(page.getByText(friendName, { exact: true })).toBeVisible({ timeout: 20_000 });
  await page.getByTestId('add-friend-confirm').click();
}

async function openFriendsTable(page: Page) {
  await page.goto('/lobby/games');
  await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 20_000 });
  await page.getByTestId('play-friends-prsi').click();
  await expect(page.getByTestId('table-screen')).toBeVisible({ timeout: 20_000 });
}

test.describe('circle invites', () => {
  test('a table opened by someone in your circle reaches you on any screen, and Join seats you', async ({
    browser,
    request,
  }) => {
    const hostCtx = await browser.newContext();
    const friendCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const friendPage = await friendCtx.newPage();

    try {
      const host = await loginAsFreshGuest(hostPage, request, `e2e-circlehost-${suffix()}`);
      const friend = await loginAsFreshGuest(friendPage, request, `e2e-circlefriend-${suffix()}`);

      await addByFriendLink(hostPage, await friendCode(request, friend), friend.username);

      // The friend's circle screen now lists the host — both directions, since
      // following a link is consent from both sides.
      await friendPage.goto('/circle');
      await expect(friendPage.getByTestId('circle-screen')).toBeVisible({ timeout: 20_000 });
      await expect(friendPage.getByText(host.username, { exact: false }).first()).toBeVisible({
        timeout: 15_000,
      });

      // The friend is somewhere that has nothing to do with playing.
      await friendPage.goto('/settings');
      await expect(friendPage.getByText(/notifications/i).first()).toBeVisible({ timeout: 20_000 });

      await openFriendsTable(hostPage);
      await expect(hostPage.getByTestId('notify-circle-card')).toContainText('1', { timeout: 15_000 });

      // What the friend sees: a banner naming the host, without having gone
      // looking for anything.
      const banner = friendPage.getByTestId('invite-banner');
      await expect(banner).toBeVisible({ timeout: 15_000 });
      await expect(banner).toContainText(host.username);

      await friendPage.getByTestId('invite-banner-join').click();
      await expect(friendPage.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });

      // And the seat is real, seen from the host's side.
      await expect(hostPage.getByTestId(`seated-${friend.userId}`)).toBeVisible({ timeout: 15_000 });
    } finally {
      await hostCtx.close();
      await friendCtx.close();
    }
  });

  test('muting someone stops their tables reaching you', async ({ browser, request }) => {
    const hostCtx = await browser.newContext();
    const friendCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const friendPage = await friendCtx.newPage();

    try {
      const host = await loginAsFreshGuest(hostPage, request, `e2e-mutehost-${suffix()}`);
      const friend = await loginAsFreshGuest(friendPage, request, `e2e-mutefriend-${suffix()}`);
      await addByFriendLink(hostPage, await friendCode(request, friend), friend.username);

      // The friend mutes the host from their circle screen.
      await friendPage.goto('/circle');
      const mute = friendPage.getByTestId(`circle-notifier-mute-guest:${host.userId}`);
      await expect(mute).toBeVisible({ timeout: 20_000 });
      const muted = friendPage.waitForResponse(
        (r) => r.url().includes('/mute') && r.request().method() === 'POST',
      );
      await mute.click();
      expect((await muted).status()).toBe(204);
      // Reload so what follows is read back from the server, not the tap.
      await friendPage.reload();
      await expect(friendPage.getByTestId('circle-screen')).toBeVisible({ timeout: 20_000 });

      await openFriendsTable(hostPage);
      await expect(hostPage.getByTestId('notify-circle-card')).toBeVisible({ timeout: 15_000 });

      // Nothing arrives. A fixed wait is the honest shape of "never": the
      // unmuted test above sees its banner well inside this window.
      await friendPage.waitForTimeout(5_000);
      await expect(friendPage.getByTestId('invite-banner')).toHaveCount(0);
    } finally {
      await hostCtx.close();
      await friendCtx.close();
    }
  });
});
