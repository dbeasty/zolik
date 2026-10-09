import { expect, test, type Browser, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * Somebody's connection drops at a table of people: the table waits, says who
 * for and for how long, and then a bot plays their seat until they are back.
 *
 * Played with two real browser contexts rather than seeded, because what is
 * under test is presence — a socket going away and coming back — and nothing
 * but a page closing does that the way a person's phone does.
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

/**
 * A started two-person Prší table. `wantFriendOnTurn` deals again until the
 * friend (not the host) is the one the table waits on first, for the tests
 * where the host has to be the one left at the table.
 */
async function prsiTable(request: Ctx, options: Record<string, number>, wantFriendOnTurn = false) {
  for (let attempt = 0; attempt < 8; attempt++) {
    const host = await guest(request, 'standin-host');
    const friend = await guest(request, 'standin-friend');
    const auth = { Authorization: `Bearer ${host.accessToken}` };
    const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'prsi', options } });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId, joinCode } = await created.json();
    const joined = await request.post(`${API_BASE}/matches/${joinCode}/join`, {
      headers: { Authorization: `Bearer ${friend.accessToken}` },
      data: {},
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();
    await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });

    const seen = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(host))).json();
    const onTurn: string = seen.view.seats.find((s: { active: boolean }) => s.active).playerId;
    if (wantFriendOnTurn && onTurn !== friend.userId) continue;
    const names = Object.fromEntries(seen.players.map((p: { id: string; name: string }) => [p.id, p.name]));
    const gone = onTurn === host.userId ? host : friend;
    const stays = gone === host ? friend : host;
    return { matchId, host, friend, gone, stays, goneName: names[gone.userId] as string };
  }
  throw new Error('the friend was never dealt the first turn');
}

async function openAs(browser: Browser, who: Guest, matchId: string): Promise<Page> {
  const context = await browser.newContext({ reducedMotion: 'reduce', viewport: { width: 1280, height: 1000 } });
  const page = await context.newPage();
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: who.accessToken,
    refreshToken: who.refreshToken,
    userId: who.userId,
    username: 'standin',
    isGuest: true,
  });
  await seedIntroSeen(page);
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId(`seat-${who.userId}`)).toBeVisible({ timeout: 30_000 });
  return page;
}

test.describe('a player whose connection drops', () => {
  test.setTimeout(150_000);

  test('the table waits, a bot plays their seat, and they get it back', async ({ browser, request }) => {
    const t = await prsiTable(request, { standInAfter: 30 });
    const stays = await openAs(browser, t.stays, t.matchId);
    const gone = await openAs(browser, t.gone, t.matchId);

    // They close the tab while the table is waiting on them.
    await gone.context().close();

    // The person still here is told who the table waits for, and for how long.
    const waiting = stays.getByTestId('table-banner-waiting');
    await expect(waiting).toBeVisible({ timeout: 15_000 });
    await expect(waiting).toContainText(t.goneName);
    await expect(waiting).toContainText(/A bot plays for them in 0:[0-3]\d/);
    await expect(stays.getByTestId(`away-badge-${t.gone.userId}`)).toBeVisible();

    // Then a bot plays their seat — the seat is still theirs, marked, not a
    // new bot sitting down.
    await expect(stays.getByTestId(`standin-badge-${t.gone.userId}`)).toBeVisible({ timeout: 45_000 });
    await expect(stays.getByTestId('table-banner-standin')).toContainText(t.goneName);
    await expect(stays.getByTestId(`seat-${t.gone.userId}`)).toContainText(t.goneName);
    // And it plays: the turn comes back round to the person who stayed.
    await expect(stays.getByTestId(`seat-active-${t.stays.userId}`)).toBeVisible({ timeout: 20_000 });

    // They come back: told what happened, and the seat is theirs again.
    const back = await openAs(browser, t.gone, t.matchId);
    await expect(back.getByTestId('table-banner-back')).toBeVisible({ timeout: 15_000 });
    await expect(stays.getByTestId(`standin-badge-${t.gone.userId}`)).toBeHidden({ timeout: 15_000 });

    const after = await (await request.get(`${API_BASE}/matches/${t.matchId}`, asViewer(t.stays))).json();
    expect(after.players.find((p: { id: string }) => p.id === t.gone.userId).standIn).toBeUndefined();
    expect(after.stoodIn).toContain(t.gone.userId);

    await stays.context().close();
    await back.context().close();
  });

  test('a table that chose to wait waits, until the host lets a bot play — or takes it out again', async ({
    browser,
    request,
  }) => {
    const t = await prsiTable(request, { standInAfter: 0 }, true);
    const host = await openAs(browser, t.host, t.matchId);
    const friend = await openAs(browser, t.friend, t.matchId);
    await friend.context().close();

    // No countdown: this table waits for them.
    const waiting = host.getByTestId('table-banner-waiting');
    await expect(waiting).toBeVisible({ timeout: 15_000 });
    await expect(waiting).toContainText(t.goneName);
    await expect(waiting).not.toContainText('A bot plays');

    // The host decides not to.
    await host.getByTestId('table-banner-let-bot').click();
    const badge = host.getByTestId(`standin-badge-${t.friend.userId}`);
    await expect(badge).toBeVisible({ timeout: 15_000 });
    await expect(host.getByTestId(`seat-active-${t.host.userId}`)).toBeVisible({ timeout: 20_000 });

    // And changes their mind: the bot comes out and the seat is waited for.
    await host.getByTestId(`standin-open-${t.friend.userId}`).click();
    await expect(host.getByTestId('standin-sheet')).toBeVisible();
    await host.getByTestId('standin-take-out').click();
    await expect(badge).toBeHidden({ timeout: 15_000 });

    await host.context().close();
  });
});
