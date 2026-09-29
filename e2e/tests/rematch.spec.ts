import { expect, test, type Browser, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Playing a finished table again when there are other people at it.
 *
 * "Play again" used to be offered only when every other seat was a bot, so two
 * friends who had just finished a game had no way to go again except to open a
 * new table and send the link round. Now the first to ask opens the rematch,
 * and everybody else still looking at the result is told whose it is and
 * offered their seat back — held for them, where they sat.
 *
 * The table is seeded one card from the end through the dev-only debug-state
 * hatch, and the last card played in the browser, so the match finishes by the
 * real path rather than by a status written over it.
 */

type Ctx = Parameters<Parameters<typeof test>[1]>[0]['request'];

async function guest(request: Ctx, name: string) {
  return (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `${name}-${Math.random().toString(36).slice(2, 6)}` },
    })
  ).json();
}

/** Ann, Bob and a bot at a Prší table, with Ann to play her last card. */
async function aTableOneCardFromTheEnd(request: Ctx) {
  const ann = await guest(request, 'Ann');
  const bob = await guest(request, 'Bob');
  const asAnn = { Authorization: `Bearer ${ann.accessToken}` };
  const { matchId } = await (
    await request.post(`${API_BASE}/matches`, { headers: asAnn, data: { moduleId: 'prsi' } })
  ).json();
  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${bob.accessToken}` },
    data: {},
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  expect((await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: asAnn, data: {} })).ok()).toBeTruthy();
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: asAnn })).ok()).toBeTruthy();

  const table = await (await request.get(`${API_BASE}/matches/${matchId}?as=${ann.userId}`)).json();
  const order: string[] = table.players.map((p: any) => p.id);
  const bot = order.find((id) => id !== ann.userId && id !== bob.userId)!;
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: asAnn,
    data: {
      state: {
        status: 'active',
        players: order,
        turnOrder: order,
        current: ann.userId,
        drawPile: ['7C', '8C', '9C', '10C'],
        discardPile: ['9S'],
        hands: { [ann.userId]: ['9D'], [bob.userId]: ['KS', '7H'], [bot]: ['8H', '10H'] },
        seed: 1,
        reshuffles: 0,
      },
    },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  return { matchId, ann, bob, order };
}

/** A browser of that player's own, so two people never share a session. */
async function openAs(browser: Browser, who: any, path: string): Promise<Page> {
  const context = await browser.newContext({ viewport: { width: 1280, height: 1200 } });
  const page = await context.newPage();
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: who.accessToken,
    refreshToken: who.refreshToken,
    userId: who.userId,
    username: 'rematch',
    isGuest: true,
  });
  await page.goto(path);
  return page;
}

async function annFinishesTheGame(browser: Browser, request: Ctx) {
  const t = await aTableOneCardFromTheEnd(request);
  const bobPage = await openAs(browser, t.bob, `/match/${t.matchId}`);
  const annPage = await openAs(browser, t.ann, `/match/${t.matchId}`);
  await expect(annPage.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await expect(bobPage.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

  await annPage.getByTestId(`card-hand:${t.ann.userId}-0`).click();
  await annPage.getByTestId('offer-play_card').click();
  await expect(annPage.getByTestId('match-over')).toBeVisible();
  await expect(bobPage.getByTestId('match-over')).toBeVisible();
  return { ...t, annPage, bobPage };
}

test.describe('a rematch with people at the table', () => {
  test('the other player is offered their seat back, and takes it', async ({ browser, request }) => {
    test.setTimeout(120_000);
    const { annPage, bobPage, bob, order } = await annFinishesTheGame(browser, request);

    // Offered to both, not only to a table of bots.
    await expect(bobPage.getByTestId('match-over-again')).toBeVisible();
    await annPage.getByTestId('match-over-again').click();

    // Ann waits at her own table, and can see who it is holding a seat for.
    await expect(annPage).toHaveURL(/\/lobby\/table\?matchId=/);
    await expect(annPage.getByTestId(`held-${bob.userId}`)).toBeVisible();

    // Bob hears about it where he is, by name, without a link being sent —
    // on the table's own banner, and only there: the invite that reaches
    // people who left would say the same thing a second time.
    await expect(bobPage.getByTestId('match-over-rematch-offer')).toHaveText(/wants a rematch/);
    await expect(bobPage.getByTestId('invite-banner')).toHaveCount(0);
    await bobPage.getByTestId('match-over-join-rematch').click();
    await expect(bobPage).toHaveURL(/\/lobby\/join\?matchId=/);

    // Everybody back where they sat, the held seat now filled.
    const nextId = new URL(annPage.url()).searchParams.get('matchId')!;
    const next = await (await request.get(`${API_BASE}/matches/${nextId}`)).json();
    expect(next.players.map((p: any) => p.id)).toEqual(order);
    expect(next.reserved ?? []).toEqual([]);
    await expect(annPage.getByTestId(`held-${bob.userId}`)).toBeHidden();
  });

  test('saying no thanks gives the seat back', async ({ browser, request }) => {
    test.setTimeout(120_000);
    const { annPage, bobPage, bob } = await annFinishesTheGame(browser, request);

    await annPage.getByTestId('match-over-again').click();
    await expect(annPage.getByTestId(`held-${bob.userId}`)).toBeVisible();

    await bobPage.getByTestId('match-over-decline-rematch').click();
    await expect(bobPage.getByTestId('match-over-rematch-offer')).toBeHidden();
    // The host is no longer shown somebody who is not coming.
    await expect(annPage.getByTestId(`held-${bob.userId}`)).toBeHidden({ timeout: 10_000 });
  });

  test('somebody who already left is invited wherever they are', async ({ browser, request }) => {
    test.setTimeout(120_000);
    const { annPage, bobPage, order } = await annFinishesTheGame(browser, request);

    // Bob has gone back to the games list before Ann asks.
    await bobPage.goto('/lobby/games');
    await expect(bobPage.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });
    await annPage.getByTestId('match-over-again').click();
    await expect(annPage).toHaveURL(/\/lobby\/table\?matchId=/);

    const banner = bobPage.getByTestId('invite-banner');
    await expect(banner).toBeVisible({ timeout: 15_000 });
    await expect(bobPage.getByTestId('invite-banner-text')).toContainText('wants a rematch');
    await expect(bobPage.getByTestId('invite-banner-text')).toContainText('Ann');
    await bobPage.getByTestId('invite-banner-join').click();
    await expect(bobPage).toHaveURL(/\/lobby\/join\?matchId=/);

    const nextId = new URL(annPage.url()).searchParams.get('matchId')!;
    const next = await (await request.get(`${API_BASE}/matches/${nextId}`)).json();
    expect(next.players.map((p: any) => p.id)).toEqual(order);
  });

  test('the host can stop waiting and seat a bot where they sat', async ({ browser, request }) => {
    test.setTimeout(120_000);
    const { annPage, bob, order } = await annFinishesTheGame(browser, request);

    await annPage.getByTestId('match-over-again').click();
    await expect(annPage.getByTestId(`held-${bob.userId}`)).toBeVisible();
    // Dealing now would go on without him, and the button says so.
    await expect(annPage.getByTestId('table-start')).toContainText('Start without');

    await annPage.getByTestId(`held-fill-${bob.userId}`).click();
    await expect(annPage.getByTestId(`held-${bob.userId}`)).toBeHidden({ timeout: 10_000 });
    await expect(annPage.getByTestId('table-start')).toHaveText('Start');

    const nextId = new URL(annPage.url()).searchParams.get('matchId')!;
    const next = await (await request.get(`${API_BASE}/matches/${nextId}`)).json();
    const seats = next.players.map((p: any) => p.id);
    expect(seats).toHaveLength(order.length);
    expect(seats[order.indexOf(bob.userId)]).toMatch(/^bot:/);
  });
});
