import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * When a deal begins, the game is in front of the player.
 *
 * The other half of `round-end-in-view.spec.ts`, and the same kind of bug
 * read the other way round. That one is about a player parked at the bottom
 * of the board when the table stops; this one is about a player dropped at
 * the *top* of it when the table starts. Starting a game navigates straight
 * to this screen, and the screen opens on the module's name, the header
 * facts and the seat strip — furniture. On the laptop window these are
 * measured at, the controls sat some five hundred pixels below the fold, and
 * every player who started a game here scrolled for them.
 *
 * So these tests open a table that has just been dealt, and assert — without
 * scrolling anything themselves — that the game is in the window. Like the
 * ending's, they measure against the *viewport* rather than asking
 * `toBeVisible()`, which is equally true of something a screen and a half
 * below the fold.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `deal-start-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Seats the human, fills the rest with bots, starts.
 *
 * Hold'em and four seats, for the reasons `round-end-in-view.spec.ts` gives:
 * a hand of it is a few presses, and a full table is the shape where the
 * board is taller than the window. A two-seat board barely is, and a test run
 * on one would be measuring a screen where nothing was ever out of view.
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
    username: host.username ?? 'deal-start',
    isGuest: true,
  });
}

/** The viewer's own hand, and a pile the whole table draws from, by the server's own names. */
async function pieces(request: Ctx, matchId: string, userId: string) {
  const state = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  const zones = state.view?.zones ?? [];
  const hand = zones.find((z: any) => z.kind === 'hand' && z.ownerId === userId);
  const pile = zones.find((z: any) => !z.ownerId && (z.kind === 'stack' || z.kind === 'pile'));
  expect(hand, 'the player should have been dealt a hand').toBeTruthy();
  expect(pile, "the table should have a pile of its own").toBeTruthy();
  return { hand: `zone-${hand.id}`, pile: `zone-${pile.id}` };
}

/** How far down the board is, and how much of it the window cannot hold. */
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

/** Whether the element is inside the window, rather than merely in the document. */
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
      // The promise for a board too tall to show whole: every part of the play
      // is at least *reachable by the eye*, rather than one end of it being
      // dropped off the window altogether.
      partlyInView: r.bottom > 0 && r.top < window.innerHeight,
    };
  }, testId);
}

/** Waits until the board has settled wherever the screen means to leave it. */
async function settled(page: Page) {
  let last = -1;
  for (let i = 0; i < 20; i++) {
    const { offset } = await board(page);
    if (offset === last) return offset;
    last = offset;
    await page.waitForTimeout(250);
  }
  return last;
}

test.describe('a dealt table brings the game to the player', () => {
  test('the play is in the window when a game opens, without scrolling to it', async ({
    page,
    request,
  }) => {
    test.setTimeout(120_000);
    // A laptop window rather than the phone the ending's tests use, because
    // the two problems live at different shapes. A phone is tall: the board
    // very nearly fits it, and what falls off the bottom is the melds nobody
    // has laid yet. A laptop is wide and *short* — the same board laid out in
    // two hundred fewer pixels of height — and there the piles, the hand and
    // the controls do not fit together at all. That is the window the request
    // for this came with.
    await page.setViewportSize({ width: 1000, height: 600 });

    const { matchId, host } = await table(request, { handLimit: 5 });
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    const { hand, pile } = await pieces(request, matchId, host.userId);
    await expect(page.getByTestId(hand)).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId('controls-panel')).toBeVisible({ timeout: 30_000 });

    // Nothing below this line scrolls the page. The player arrived here from
    // the lobby, which is to say at the top of the board.
    const offset = await settled(page);

    expect(
      (await board(page)).overflow,
      'a board the window can hold has nothing to say about this',
    ).toBeGreaterThan(150);
    expect(
      offset,
      'the screen should have gone to the game rather than leaving the player on the furniture',
    ).toBeGreaterThan(0);

    // Their own cards, whole: this is the thing a player came to look at, and
    // on this board it is the middle of the play.
    const cards = await viewportBox(page, hand);
    expect(
      cards!.whollyInView,
      `the hand was at ${cards!.top}..${cards!.bottom} in a ${cards!.windowHeight}-tall window`,
    ).toBeTruthy();

    // And both ends of the play with it — the pile the game is dealt from
    // above, the controls that spend the hand below. Before the screen learned
    // to do this, the controls sat about 500px below the fold.
    const pileBox = await viewportBox(page, pile);
    expect(
      pileBox!.partlyInView,
      `the table's pile was at ${pileBox!.top}..${pileBox!.bottom}`,
    ).toBeTruthy();
    const controls = await viewportBox(page, 'controls-panel');
    expect(
      controls!.partlyInView,
      `the controls were at ${controls!.top}..${controls!.bottom}`,
    ).toBeTruthy();
  });

  test('the next round brings the play back after the settlement', async ({ page, request }) => {
    test.setTimeout(180_000);
    await page.setViewportSize({ width: 1000, height: 600 });

    const { matchId, host } = await table(request, { handLimit: 5 });
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    const { hand } = await pieces(request, matchId, host.userId);
    await expect(page.getByTestId(hand)).toBeVisible({ timeout: 30_000 });

    // Play the hand out — whatever is offered, except the way on itself, which
    // is the moment this test starts from.
    for (let i = 0; i < 200; i++) {
      const state = await (
        await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`)
      ).json();
      if (state.rounds?.paused) break;
      const ids = await page
        .locator('[data-testid^="offer-"]:not([aria-disabled="true"])')
        .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? '').filter(Boolean));
      const pick = ids.find((id) => id !== 'offer-show' && !id.includes('continue'));
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

    // The table has stopped, and `useEndingScroll` has taken the player up to
    // the settlement — which is where this test's subject begins.
    await expect(page.getByTestId('offer-continue')).toBeVisible({ timeout: 30_000 });
    const atTheSettlement = await settled(page);
    expect(
      (await viewportBox(page, 'round-results'))?.partlyInView,
      'the settlement should be what the player is looking at when the round ends',
    ).toBeTruthy();

    await page.getByTestId('offer-continue').click();
    await expect
      .poll(
        async () =>
          (await (await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`)).json())
            .rounds?.paused ?? false,
        { timeout: 30_000 },
      )
      .toBe(false);

    // Again, nothing here scrolls the page.
    const dealt = await settled(page);
    expect(
      dealt,
      'a fresh deal should take the player back down to it, not leave them on the settlement',
    ).toBeGreaterThan(atTheSettlement);
    const cards = await viewportBox(page, hand);
    expect(
      cards!.partlyInView,
      `the new hand was at ${cards!.top}..${cards!.bottom} in a ${cards!.windowHeight}-tall window`,
    ).toBeTruthy();
    expect(
      (await viewportBox(page, 'controls-panel'))?.partlyInView,
      'and the controls that play it with them',
    ).toBeTruthy();
  });
});
