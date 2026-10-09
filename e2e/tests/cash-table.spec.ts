import { expect, test, type Browser, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * Getting up from a poker table.
 *
 * At a cash table the chips in front of you are yours: you may leave whenever
 * you like, and what you leave with is your result. In a tournament they are
 * what everybody is playing for, so there is no such control at all.
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

/** Two people and a bot at a poker table in the given format. */
async function pokerTable(request: Ctx, format: number) {
  const host = await guest(request, 'cash-host');
  const friend = await guest(request, 'cash-friend');
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'holdem', options: { format } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId, joinCode } = await created.json();
  const joined = await request.post(`${API_BASE}/matches/${joinCode}/join`, {
    headers: { Authorization: `Bearer ${friend.accessToken}` },
    data: {},
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth, data: {} });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host, friend };
}

async function openAs(browser: Browser, who: Guest, matchId: string): Promise<Page> {
  const context = await browser.newContext({ reducedMotion: 'reduce', viewport: { width: 1280, height: 1000 } });
  const page = await context.newPage();
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: who.accessToken,
    refreshToken: who.refreshToken,
    userId: who.userId,
    username: 'cash',
    isGuest: true,
  });
  await seedIntroSeen(page);
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId(`seat-${who.userId}`)).toBeVisible({ timeout: 30_000 });
  return page;
}

test.describe('getting up from a poker table', () => {
  test.setTimeout(90_000);

  test('at a cash table a player leaves with their chips, after saying yes', async ({ browser, request }) => {
    const t = await pokerTable(request, 1);
    const host = await openAs(browser, t.host, t.matchId);
    const friend = await openAs(browser, t.friend, t.matchId);

    const leave = friend.getByTestId('offer-leave');
    await expect(leave).toBeVisible({ timeout: 15_000 });

    // It asks first, and "stay" changes nothing.
    await leave.click();
    await expect(friend.getByTestId('leave-sheet')).toBeVisible();
    await friend.getByTestId('leave-cancel').click();
    await expect(friend.getByTestId('leave-sheet')).toBeHidden();
    await expect(friend.getByTestId('offer-leave')).toBeVisible();

    await friend.getByTestId('offer-leave').click();
    await friend.getByTestId('leave-confirm').click();

    // Everybody's board says so, and the seat keeps its chips as its result.
    await expect(host.getByTestId(`seat-${t.friend.userId}`)).toContainText('Left the table', { timeout: 15_000 });
    await expect(friend.getByTestId('offer-leave')).toBeHidden();

    const seen = await (await request.get(`${API_BASE}/matches/${t.matchId}`, asViewer(t.host))).json();
    const seat = seen.view.seats.find((s: { playerId: string }) => s.playerId === t.friend.userId);
    expect(seat.labelKeys).toContain('seat.left');
    expect(seen.status).toBe('active');

    await host.context().close();
    await friend.context().close();
  });

  test('a tournament has no way to leave with the chips', async ({ browser, request }) => {
    const t = await pokerTable(request, 0);
    const friend = await openAs(browser, t.friend, t.matchId);
    await expect(friend.getByTestId('controls-panel')).toBeVisible({ timeout: 15_000 });
    await expect(friend.getByTestId('offer-leave')).toHaveCount(0);
    await friend.context().close();
  });
});
