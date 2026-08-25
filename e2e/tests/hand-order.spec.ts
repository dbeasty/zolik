import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * Arranging the cards in your own hand.
 *
 * A player rearranging their hand is the oldest habit in card games and the
 * one thing the generic shell could not do: it drew the hand in whatever order
 * the module happened to keep it, and redrew it from scratch every time anyone
 * at the table moved.
 *
 * The interesting claim is not that a card can be dragged — it is that doing so
 * is *not a move*. Arrangement is a view preference, so it never reaches the
 * server, no module knows the feature exists, and every game gets it anyway.
 * The third test is the one that actually pins that down.
 *
 * Žolíky is used here only because it deals a large hand. Nothing below knows
 * a rule of it, and the same drags work in any of the four.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats: number) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `hand-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    expect((await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth })).ok())
      .toBeTruthy();
  }
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok())
    .toBeTruthy();

  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'hand',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The viewer's hand as the *server* holds it — the order nobody rearranged. */
async function serverHand(request: Ctx, matchId: string, userId: string): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const hand = (body.view?.zones ?? []).find(
    (z: any) => z.kind === 'hand' && z.ownerId === userId,
  );
  return (hand?.cards ?? []).map((c: any) => c.card);
}

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);

test.describe('arranging your hand', () => {
  test('a card dragged along the hand lands where it was dropped, and takes nothing with it', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    expect(before.length).toBeGreaterThan(4);

    await dragLocatorTo(page, card(page, 0), card(page, 3));

    const after = await handCards(page);

    // The card that was first is now fourth, and everything it passed has
    // shifted down one. Asserting the whole row rather than just the moved
    // card is deliberate: the failure worth catching is a reorder that also
    // disturbs its neighbours.
    const expected = [before[1], before[2], before[3], before[0], ...before.slice(4)];
    expect(after).toEqual(expected);

    // Nothing gained, nothing lost, nothing duplicated. A slot model that
    // mints a new identity where it should have reused one shows up here as
    // a card appearing twice.
    expect([...after].sort()).toEqual([...before].sort());
  });

  test('the arrangement survives the state pushes that follow every move at the table', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    await dragLocatorTo(page, card(page, 5), card(page, 0));

    const arranged = await handCards(page);
    expect(arranged[0]).toBe(before[5]);

    // This is the whole reason arrangement had to be reconciled rather than
    // recomputed. The server re-pushes the entire board after every move by
    // anyone, in its own order; before this existed, each bot turn silently
    // reshuffled the player's hand back underneath them. Bots move on their
    // own, so waiting is all it takes to get several pushes.
    await page.waitForTimeout(4000);

    const later = await handCards(page);
    expect(later[0]).toBe(before[5]);
    // Compared as a subsequence, because a turn may legitimately have taken a
    // card out of the hand — what must hold is that the surviving cards are
    // still in the order they were put in, not that the hand is identical.
    expect(later.filter((c) => arranged.includes(c))).toEqual(
      arranged.filter((c) => later.includes(c)),
    );
  });

  test('rearranging is not a move: the server never hears about it', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const serverBefore = await serverHand(request, matchId, host.userId);
    const shownBefore = await handCards(page);
    expect(serverBefore.length).toBe(shownBefore.length);

    await dragLocatorTo(page, card(page, 0), card(page, 2));

    const shownAfter = await handCards(page);
    expect(shownAfter).not.toEqual(shownBefore);

    // The point of the whole design: the drag changed what the player sees and
    // nothing else. If arrangement had been sent as an action — or worse, if
    // the shell had rebuilt a submission from screen positions — the server's
    // own copy would have moved too.
    const serverAfter = await serverHand(request, matchId, host.userId);
    expect(serverAfter).toEqual(serverBefore);
  });
});
