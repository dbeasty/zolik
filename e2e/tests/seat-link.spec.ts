import { expect, test, type Browser, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * A seat link: one person back to their own seat, on whatever device they
 * have now.
 *
 * The case it exists for: a table swept up while Bob was away cannot be picked
 * back up until everybody is back — and Bob, on a new phone or a cleared
 * browser, is a stranger to his own table. Following the join link makes him a
 * new guest, who is not the seat everybody is waiting for.
 *
 * The table is seeded as abandoned through the dev-only debug-state hatch
 * rather than waited into, as abandoned-table.spec does.
 */

type Ctx = Parameters<Parameters<typeof test>[1]>[0]['request'];

async function guest(request: Ctx, name: string) {
  return (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `${name}-${Math.random().toString(36).slice(2, 6)}` },
    })
  ).json();
}

/** Ann and Bob's Prší table, set aside while Bob was away, and his link. */
async function anAbandonedTableAndBobsLink(request: Ctx) {
  const ann = await guest(request, 'Ann');
  const bob = await guest(request, 'Bob');
  const asAnn = { Authorization: `Bearer ${ann.accessToken}` };
  const { matchId } = await (
    await request.post(`${API_BASE}/matches`, { headers: asAnn, data: { moduleId: 'prsi' } })
  ).json();
  expect(
    (await request.post(`${API_BASE}/matches/${matchId}/join`, { headers: { Authorization: `Bearer ${bob.accessToken}` }, data: {} })).ok(),
  ).toBeTruthy();
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: asAnn })).ok()).toBeTruthy();
  const order = [ann.userId, bob.userId];
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: asAnn,
    data: {
      status: 'abandoned',
      state: {
        status: 'active',
        players: order,
        turnOrder: order,
        current: ann.userId,
        drawPile: ['7C', '8C', '9C', '10C'],
        discardPile: ['9S'],
        hands: { [ann.userId]: ['9D', 'KS'], [bob.userId]: ['KH', '7H'] },
        seed: 1,
        reshuffles: 0,
      },
    },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();

  const minted = await request.post(`${API_BASE}/matches/${matchId}/seats/${bob.userId}/link`, { headers: asAnn });
  expect(minted.ok(), await minted.text()).toBeTruthy();
  const { path } = await minted.json();
  return { matchId, ann, bob, path: path as string };
}

async function signedInAs(browser: Browser, who: any): Promise<Page> {
  const context = await browser.newContext({ viewport: { width: 1280, height: 1200 } });
  const page = await context.newPage();
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: who.accessToken,
    refreshToken: who.refreshToken,
    userId: who.userId,
    username: 'seat-link',
    isGuest: true,
  });
  return page;
}

/** A device that has never played: no session, nothing stored. */
async function aNewPhone(browser: Browser): Promise<Page> {
  const context = await browser.newContext({ viewport: { width: 1280, height: 1200 } });
  return context.newPage();
}

test.describe('a seat link', () => {
  test('brings Bob back to his own seat on a new phone, and the table can be picked up', async ({ browser, request }) => {
    test.setTimeout(120_000);
    const { matchId, ann, bob, path } = await anAbandonedTableAndBobsLink(request);

    const annPage = await signedInAs(browser, ann);
    await annPage.goto(`/match/${matchId}`);
    await expect(annPage.getByTestId('match-over-waiting')).toContainText('Bob');
    await expect(annPage.getByTestId('match-over-resume')).toHaveCount(0);

    // Before anything is taken, the link says whose seat it is.
    const phone = await aNewPhone(browser);
    await phone.goto(path);
    await expect(phone.getByTestId('seat-you-are')).toContainText("You're Bob");
    await expect(phone.getByTestId(`seat-other-${ann.userId}`)).toContainText('at the table');

    await phone.getByTestId('seat-claim').click();
    await expect(phone).toHaveURL(new RegExp(`/match/${matchId}$`));
    // His own hand, not a spectator's.
    await expect(phone.getByTestId(`card-hand:${bob.userId}-0`)).toBeVisible({ timeout: 30_000 });
    // The phone took the seat, not Bob's identity: no session was made for it.
    const stored = await phone.evaluate(() => Object.keys(window.localStorage));
    expect(stored).toContain(`zolik_seat_${matchId}`);
    expect(stored).not.toContain('zolik_session');

    // Everybody is back, so either of them may pick it up.
    await expect(annPage.getByTestId('match-over-resume')).toBeVisible({ timeout: 10_000 });
    await phone.getByTestId('match-over-resume').click();
    await expect(phone.getByTestId('match-over')).toBeHidden({ timeout: 10_000 });
    await expect(annPage.getByTestId('match-over')).toBeHidden({ timeout: 10_000 });
  });

  test('"Not Bob?" takes nothing', async ({ browser, request }) => {
    const { path, matchId } = await anAbandonedTableAndBobsLink(request);
    const phone = await aNewPhone(browser);
    await phone.goto(path);
    await expect(phone.getByTestId('seat-you-are')).toBeVisible();
    await phone.getByTestId('seat-not-me').click();
    await expect(phone).not.toHaveURL(/\/seat\//);
    const stored = await phone.evaluate(() => Object.keys(window.localStorage));
    expect(stored).not.toContain(`zolik_seat_${matchId}`);
  });

  test('a replaced link says so', async ({ browser, request }) => {
    const { path, matchId, ann, bob } = await anAbandonedTableAndBobsLink(request);
    // Ann sent it to the wrong person, and makes another.
    expect(
      (await request.post(`${API_BASE}/matches/${matchId}/seats/${bob.userId}/link`, {
        headers: { Authorization: `Bearer ${ann.accessToken}` },
      })).ok(),
    ).toBeTruthy();
    const phone = await aNewPhone(browser);
    await phone.goto(path);
    await expect(phone.getByTestId('seat-error')).toContainText('no longer opens a seat');
    await expect(phone.getByTestId('seat-claim')).toHaveCount(0);
  });
});
