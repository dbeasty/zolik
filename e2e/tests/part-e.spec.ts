import { expect, test, type Browser, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * A seat changing hands, and a table taking somebody new.
 *
 * - The host hands an away player's seat to somebody else: they open a link,
 *   give their name and play the seat; its first player's own sign-in is
 *   told it is no longer theirs.
 * - A cash poker table takes a player who arrives after the deal.
 */

type Ctx = Parameters<Parameters<typeof test>[1]>[0]['request'];
type Guest = { userId: string; accessToken: string; refreshToken: string };

async function guest(request: Ctx, name: string): Promise<Guest> {
  return (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `${name}-${Math.random().toString(36).slice(2, 8)}` },
    })
  ).json();
}

async function openAs(browser: Browser, who: Guest | null, path: string): Promise<Page> {
  const context = await browser.newContext({ reducedMotion: 'reduce', viewport: { width: 1280, height: 1000 } });
  const page = await context.newPage();
  if (who) {
    await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
      accessToken: who.accessToken,
      refreshToken: who.refreshToken,
      userId: who.userId,
      username: 'parte',
      isGuest: true,
    });
  }
  await seedIntroSeen(page);
  await page.goto(path);
  return page;
}

test.describe('a seat changing hands', () => {
  test.setTimeout(120_000);

  test('the host hands an away player’s seat to somebody new, who plays it under their own name', async ({
    browser,
    request,
  }) => {
    // A Prší table that waits for its players, with the friend on turn.
    let t: { matchId: string; host: Guest; friend: Guest; friendName: string } | null = null;
    for (let attempt = 0; attempt < 8 && !t; attempt++) {
      const host = await guest(request, 'parte-host');
      const friend = await guest(request, 'parte-friend');
      const auth = { Authorization: `Bearer ${host.accessToken}` };
      const { matchId, joinCode } = await (
        await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'prsi', options: { standInAfter: 0 } } })
      ).json();
      await request.post(`${API_BASE}/matches/${joinCode}/join`, {
        headers: { Authorization: `Bearer ${friend.accessToken}` },
        data: {},
      });
      await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
      const seen = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(host))).json();
      const onTurn = seen.view.seats.find((s: { active: boolean }) => s.active).playerId;
      if (onTurn !== friend.userId) continue;
      t = { matchId, host, friend, friendName: seen.players.find((p: { id: string }) => p.id === friend.userId).name };
    }
    expect(t, 'the friend was never dealt the first turn').toBeTruthy();
    const { matchId, host, friend, friendName } = t!;

    const hostPage = await openAs(browser, host, `/match/${matchId}`);
    await expect(hostPage.getByTestId(`seat-${host.userId}`)).toBeVisible({ timeout: 30_000 });
    // The friend was here, and has gone.
    const gone = await openAs(browser, friend, `/match/${matchId}`);
    await expect(gone.getByTestId(`seat-${friend.userId}`)).toBeVisible({ timeout: 30_000 });
    await gone.context().close();
    // A bot first, so the seat has the host's sheet.
    await expect(hostPage.getByTestId('table-banner-waiting')).toBeVisible({ timeout: 15_000 });
    await hostPage.getByTestId('table-banner-let-bot').click();
    await hostPage.getByTestId(`standin-open-${friend.userId}`).click();
    await hostPage.getByTestId('standin-hand-over').click();
    const link = (await hostPage.getByTestId('handover-url').innerText()).trim();
    const path = new URL(link, 'http://x').pathname;
    expect(path).toMatch(/^\/seat\//);
    await hostPage.getByTestId('standin-backdrop').click({ position: { x: 5, y: 5 } });

    // Somebody new opens it, says who they are, and takes the seat.
    const newcomer = await openAs(browser, null, path);
    await expect(newcomer.getByTestId('seat-you-are')).toContainText(friendName, { timeout: 30_000 });
    await newcomer.getByTestId('seat-name').fill('Zed');
    await newcomer.getByTestId('seat-claim').click();
    await expect(newcomer).toHaveURL(new RegExp(`/match/${matchId}`), { timeout: 30_000 });
    await expect(newcomer.getByTestId(`seat-${friend.userId}`)).toContainText('Zed', { timeout: 30_000 });

    // Everybody sees whose seat it was.
    await expect(hostPage.getByTestId(`seat-${friend.userId}`)).toContainText('Zed', { timeout: 15_000 });
    await expect(hostPage.getByTestId(`former-${friend.userId}`)).toContainText(friendName);

    // The friend comes back on their own sign-in, and is told it is gone.
    const back = await openAs(browser, friend, `/match/${matchId}`);
    await expect(back.getByText('Your seat was handed to someone else while you were away')).toBeVisible({
      timeout: 30_000,
    });

    for (const p of [hostPage, newcomer, back]) await p.context().close();
  });

  test('a cash poker table takes a player who arrives after the deal', async ({ browser, request }) => {
    const host = await guest(request, 'parte-cash-host');
    const friend = await guest(request, 'parte-cash-friend');
    const late = await guest(request, 'parte-late');
    const auth = { Authorization: `Bearer ${host.accessToken}` };
    const { matchId, joinCode } = await (
      await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'holdem', options: { format: 1 } } })
    ).json();
    await request.post(`${API_BASE}/matches/${joinCode}/join`, {
      headers: { Authorization: `Bearer ${friend.accessToken}` },
      data: {},
    });
    await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });

    // Late arrives with the code, the way anybody joins a table.
    const joined = await request.post(`${API_BASE}/matches/${joinCode}/join`, {
      headers: { Authorization: `Bearer ${late.accessToken}` },
      data: {},
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();

    const page = await openAs(browser, late, `/match/${matchId}`);
    await expect(page.getByTestId(`seat-${late.userId}`)).toBeVisible({ timeout: 30_000 });
    // Nobody has acted, so the hand in play is still the first: the new seat
    // sits it out and says so.
    await expect(page.getByTestId(`seat-${late.userId}`)).toContainText('JOINS NEXT DEAL');
    const seen = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(late))).json();
    const mine = seen.view.seats.find((s: { playerId: string }) => s.playerId === late.userId);
    expect(mine.labelKeys).toContain('seat.joining');
    await page.context().close();
  });
});
