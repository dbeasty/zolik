import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * When the table stops, the way on is in front of the player.
 *
 * The end of a round and the end of a match both leave exactly one thing to do
 * — go on to the next round, or play the same table again — and both draw it at
 * the top of a board whose reader is at the bottom of it. The layout puts a
 * hand and the controls that spend it together under the thumb, deliberately,
 * so that is where a player has been for the whole of the turn that just
 * ended. Measured on a 390-wide phone parked there, the settlement lands
 * several hundred pixels above the fold. `ResultsFlash` says the round is over;
 * it does not say where the button is, and two seconds later it is gone and the
 * screen looks like the one that was already there.
 *
 * These tests park on the hand the way a player does, let the round end, and
 * then assert — without scrolling — that the control is *inside the viewport*.
 * That is the assertion the old ones were missing: `toBeVisible()` is true of
 * anything rendered and not hidden, including something a long way off the top
 * of the page, which is how both announcements came to be shipped with green
 * tests and reported as missing.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `round-end-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Seats the human, fills the rest with bots, starts.
 *
 * Hold'em, because a hand of it is a few presses where a rummy deal is a
 * hundred — and the screen under test names no game, so the game that reaches
 * the moment soonest is the right one to reach it with.
 *
 * Four seats rather than the two a bug report needs, because two of them make a
 * board barely taller than a phone: the hand ends up near the top whatever the
 * player does, and a test parked on it would be measuring a screen where
 * nothing was ever out of view. A full table is the shape this goes wrong on.
 */
async function table(request: Ctx, options: Record<string, number>, bots = 3) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'holdem', variation: 'timed', options },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (let i = 0; i < bots; i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, host };
}

async function signIn(page: Page, host: any) {
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'round-end',
    isGuest: true,
  });
}

/**
 * Which zone on this board is the player's own hand — the thing the layout
 * deliberately puts under their thumb, and therefore the thing they are looking
 * at. Asked of the server rather than guessed from the DOM, because the screen
 * under test names no game and neither should this.
 */
async function handZone(request: Ctx, matchId: string, userId: string): Promise<string> {
  const state = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  const mine = (state.view?.zones ?? []).find(
    (z: any) => z.kind === 'hand' && z.ownerId === userId,
  );
  expect(mine, 'the player should have a hand to look at').toBeTruthy();
  return mine.id;
}

/**
 * Puts the player where a player is: their own hand at the top of the window,
 * the controls that spend it just below, and everything the board did before
 * this turn scrolled off above.
 *
 * `scrollIntoView` rather than Playwright's `scrollIntoViewIfNeeded`, which
 * does nothing when the element is already on screen somewhere — including the
 * bottom of a window whose top still shows the settlement, which is the one
 * arrangement this test must not accidentally set up.
 */
async function parkOnTheHand(page: Page, zoneId: string) {
  await page.evaluate((id) => {
    document.querySelector(`[data-testid="zone-${id}"]`)?.scrollIntoView({ block: 'start' });
  }, zoneId);
}

/**
 * How far down the board is, and how much of it the window cannot hold.
 *
 * The scroller is found by walking up from the board rather than named
 * outright: react-native-web builds a ScrollView out of nested elements and
 * puts the testID on the content, so the one that actually scrolls is an
 * ancestor whose depth is the web renderer's business rather than this suite's.
 */
async function board(page: Page) {
  return page.evaluate(() => {
    let el = document.querySelector('[data-testid="match-screen"]') as HTMLElement | null;
    while (el && el.scrollHeight <= el.clientHeight + 1) el = el.parentElement;
    if (!el) return { offset: 0, overflow: 0 };
    return {
      offset: Math.round(el.scrollTop),
      overflow: Math.round(el.scrollHeight - el.clientHeight),
    };
  });
}

/**
 * Whether the element is inside the window, rather than merely in the document.
 *
 * The whole point of the suite: `toBeVisible()` cannot tell those apart, and
 * the difference is every reported instance of this bug.
 */
async function viewportBox(page: Page, testId: string) {
  return page.evaluate((id) => {
    const el = document.querySelector(`[data-testid="${id}"]`) as HTMLElement | null;
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return {
      top: Math.round(r.top),
      bottom: Math.round(r.bottom),
      windowHeight: window.innerHeight,
      whollyInView: r.top >= 0 && r.bottom <= window.innerHeight,
    };
  }, testId);
}

/** Presses whatever the table offers, stopping the moment it stops offering play. */
async function playUntil(
  page: Page,
  request: Ctx,
  matchId: string,
  userId: string,
  zoneId: string,
  done: (s: any) => boolean,
) {
  // The board arrives over the socket a moment after the screen does, and a
  // hand that is not on screen yet cannot be looked at: parking on it before it
  // is there records a board with nowhere to scroll, and the measurement this
  // suite exists for is where the player was.
  await expect(page.getByTestId(`zone-${zoneId}`)).toBeVisible({ timeout: 30_000 });

  // Where the board was left the last time the player looked at their hand.
  // Read here rather than after the table stops, because by then the screen may
  // already have done the thing under test.
  let parked = 0;
  for (let i = 0; i < 120; i++) {
    const state = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
    if (done(state)) return { state, parked };
    await parkOnTheHand(page, zoneId);
    parked = (await board(page)).offset;
    const ids = await page
      .locator('[data-testid^="offer-"]:not([aria-disabled="true"])')
      .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? '').filter(Boolean));
    // Never press the intermission's own control here — that is the moment
    // under test, and agreeing to go on would skip it.
    const pick = ids.find((id) => !id.includes('continue'));
    if (!pick) {
      await page.waitForTimeout(400);
      continue;
    }
    try {
      await page.getByTestId(pick).click({ timeout: 4000 });
    } catch {
      // The board moved under us; the server state read above decides.
    }
    await page.waitForTimeout(250);
  }
  return { state: null, parked };
}

test.describe('a stopped table brings the way on to the player', () => {
  test('the control to start the next round is in view without scrolling to it', async ({
    page,
    request,
  }) => {
    test.setTimeout(180_000);
    // A phone, because a phone is where the board is taller than the window and
    // a settlement at the top of it is a settlement nobody receives.
    await page.setViewportSize({ width: 390, height: 844 });

    const { matchId, host } = await table(request, { handLimit: 5, pauseBetweenRounds: 1 });
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    const hand = await handZone(request, matchId, host.userId);
    const { state, parked } = await playUntil(
      page,
      request,
      matchId,
      host.userId,
      hand,
      (s) => !!s.rounds?.paused,
    );
    expect(
      state,
      'a table with the pause switched on should have stopped between rounds',
    ).toBeTruthy();

    // Nothing below this line scrolls the page. The player was last seen on
    // their own hand, well down a board the window cannot hold.
    await expect(page.getByTestId('offer-continue')).toBeVisible({ timeout: 30_000 });

    const goOn = await viewportBox(page, 'offer-continue');
    expect(goOn, 'a stopped table should offer a way on at all').toBeTruthy();
    expect(
      goOn!.whollyInView,
      `the way on was at ${goOn!.top}..${goOn!.bottom} in a ${goOn!.windowHeight}-tall window — ` +
        'a player looking at their own hand never sees it',
    ).toBeTruthy();

    // In view is not the same as noticed. The one thing left to do wears a ring
    // and is filled; everything the table is not waiting on is an outline. The
    // suite runs under prefers-reduced-motion, so the ring here is the still
    // version of it — which is the point: the cue survives someone who asked
    // their system for less movement.
    const ringed = await page.evaluate(() => {
      const el = document.querySelector('[data-testid="offer-continue"]') as HTMLElement;
      const ring = el.querySelector('[data-testid="attention-ring"]');
      return { ring: !!ring, fill: getComputedStyle(el).backgroundColor };
    });
    expect(ringed.ring, 'the way on should be ringed, not just present').toBeTruthy();
    expect(ringed.fill, 'the way on should be the filled control, not an outline').not.toBe(
      'rgba(0, 0, 0, 0)',
    );

    // And the settlement it belongs to came with it, rather than being cut off
    // above the fold — which is how a player comes to think they missed
    // something.
    const results = await viewportBox(page, 'round-results');
    expect(results?.top, 'the settlement should not be cut off above the window').toBeGreaterThanOrEqual(0);

    // The two facts that make the assertion above worth making, so this can
    // never quietly become a test of a board that fits on a phone. Without the
    // screen going to fetch the player, this measured -13..19: the way on sat
    // across the top edge of the window, half of it above the fold.
    expect(
      (await board(page)).overflow,
      'a board the window can hold has nothing to say about this',
    ).toBeGreaterThan(150);
    expect(parked, 'the player should have been down the board, on their hand').toBeGreaterThan(0);

    // And it works: pressing it starts the next round.
    await page.getByTestId('offer-continue').click();
    await expect
      .poll(
        async () =>
          (await (await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`)).json())
            .rounds?.paused ?? false,
        { timeout: 30_000 },
      )
      .toBe(false);
  });

  test('the offer to play again is in view when the match ends', async ({ page, request }) => {
    test.setTimeout(180_000);
    await page.setViewportSize({ width: 390, height: 844 });

    // No pause between rounds: the table runs its hands straight through and
    // stops for good, which is the only stop under test here.
    const { matchId, host } = await table(request, { handLimit: 5 });
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    const hand = await handZone(request, matchId, host.userId);
    const { state, parked } = await playUntil(
      page,
      request,
      matchId,
      host.userId,
      hand,
      (s) => s.status === 'completed',
    );
    expect(state, 'the shortest table this game offers should have ended').toBeTruthy();

    await expect(page.getByTestId('match-over')).toBeVisible({ timeout: 30_000 });

    const banner = await viewportBox(page, 'match-over');
    expect(
      banner!.whollyInView,
      `the banner was at ${banner!.top}..${banner!.bottom} in a ${banner!.windowHeight}-tall window — ` +
        'which is exactly how a finished match came to be reported as a hung one',
    ).toBeTruthy();

    // Every other seat was a bot, so the same table is one press away — and the
    // press is the point of arriving here, so it is ringed like the way on
    // between rounds.
    expect(
      await page
        .locator('[data-testid="match-over-again"] [data-testid="attention-ring"]')
        .count(),
      'the offer to play again should be ringed',
    ).toBe(1);
    const again = await viewportBox(page, 'match-over-again');
    expect(
      again?.whollyInView,
      'the offer to play the same table again should be in view',
    ).toBeTruthy();

    // As above. Without the screen going to fetch the player, this measured
    // -511..-395: the banner was half a screen above the top of the window.
    expect(
      (await board(page)).overflow,
      'a board the window can hold has nothing to say about this',
    ).toBeGreaterThan(150);
    expect(parked, 'the player should have been down the board, on their hand').toBeGreaterThan(0);
  });
});
