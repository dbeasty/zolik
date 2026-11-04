import { expect, test, type Page } from '@playwright/test';

import { carryPointOver, grabPoint, handCards, release, touchCarry } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { cardByCode } from '../helpers/hand';

/**
 * The floating hand: the hand and its controls docked above the scrolling
 * board, so a player can look over a long row of melds and still drag a card
 * onto the one they are looking at (plans/floating-hand.md).
 *
 * CanastaX, seeded through the debug-state hatch with two humans and no bots,
 * and the other side holding enough melds that their sequence — the one the
 * 6♠ buys the joker out of — is well below the hand:
 *
 *   ours    K K K K 2♣        9♣ 9♦ 9♠
 *   theirs  eight sets of three, then 5♠ JOKER2 7♠ 8♠
 *   hand    6♠ 9♥ Q♣ 2♦ 2♥ A♣ 10♣ 4♠
 *
 * Every check is against what the user would see — one hand, one action bar,
 * the dock on screen — or against the server's own copy of the table.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx, name: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName: name } });
  expect(res.ok()).toBeTruthy();
  return res.json();
}

const FILLER_RANKS = ['4', '5', '6', '7', '8', 'T', 'J', 'A'];

async function longTable(request: Ctx) {
  const host = await guest(request, `fh-${Math.random().toString(36).slice(2, 8)}`);
  const other = await guest(request, `fo-${Math.random().toString(36).slice(2, 8)}`);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'canastax', options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  expect(
    (await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${other.accessToken}` },
    })).ok(),
  ).toBeTruthy();
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

  const p1 = host.userId as string;
  const p2 = other.userId as string;
  const state: any = {
    status: 'active',
    variation: 'canastax',
    players: [p1, p2],
    turnOrder: [p1, p2],
    current: p1,
    phase: 'meld',
    teams: [
      {
        id: 0, players: [p1], score: 0, hasMelded: true, meldSeq: 1,
        melds: [
          { id: 't0-K', teamId: 0, kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', '2C'] },
          { id: 't0-9', teamId: 0, kind: 'set', rank: '9', cards: ['9C', '9D', '9S'] },
        ],
      },
      {
        id: 1, players: [p2], score: 0, hasMelded: true, meldSeq: 1,
        melds: [
          ...FILLER_RANKS.map((r) => ({ id: `t1-${r}`, teamId: 1, kind: 'set', rank: r, cards: [`${r}H`, `${r}D`, `${r}S`] })),
          { id: 't1-seq1', teamId: 1, kind: 'run', suit: 'S', low: 1, cards: ['5S', 'JOKER2', '7S', '8S'] },
        ],
      },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(40).fill('4C'),
    discardPile: ['4D', '8C'],
    hands: {
      [p1]: ['6S', '9H', 'QC', '2D', '2H', 'AC', 'TC', '4S'],
      [p2]: ['8H', '9H', 'TH', 'JD', 'QD'],
    },
    handSize: 15,
    targetScore: 10000,
    canastasToGoOut: 2,
    dealNumber: 1,
    dealer: 1,
    meldsAtTurnStart: true,
    pause: true,
    seed: 7,
  };
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, { headers: auth, data: { state } });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  return { matchId, host };
}

/** A phone-ish height, so the melds are well below the hand. */
async function openMatch(page: Page, host: any, matchId: string, docked = false, width = 900) {
  await page.setViewportSize({ width, height: 800 });
  await page.addInitScript(
    ({ s, docked }) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
      window.localStorage.setItem('zolik_seen_intro', '1');
      if (docked) window.localStorage.setItem('zolik_floating_hand', JSON.stringify({ canasta: true }));
    },
    {
      s: {
        accessToken: host.accessToken,
        refreshToken: host.refreshToken,
        userId: host.userId,
        username: host.guestName,
        isGuest: true,
      },
      docked,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await handCards(page);
}

async function onServer(request: Ctx, matchId: string, host: any) {
  const body = await (
    await request.get(`${API_BASE}/matches/${matchId}`, { headers: { Authorization: `Bearer ${host.accessToken}` } })
  ).json();
  const melds: Record<string, string> = {};
  let hand: string[] = [];
  for (const z of body.view?.zones ?? []) {
    for (const g of z.groups ?? []) melds[g.id] = g.cards.join(' ');
    if (z.kind === 'hand' && z.ownerId === host.userId) hand = (z.cards ?? []).map((c: any) => c.card);
  }
  return { melds, hand };
}

/** Scrolls the board — not the window — until this is in view below the dock. */
async function scrollBoardTo(page: Page, testId: string) {
  await page.getByTestId(testId).scrollIntoViewIfNeeded();
  await page.waitForTimeout(150);
}

/** Drags a hand card by its code to a point, and lets go. */
async function dragCardTo(page: Page, code: string, to: { x: number; y: number }) {
  const from = await grabPoint(cardByCode(page, code));
  await carryPointOver(page, from, to);
  await release(page);
}

test('the float button docks the hand and the controls, one copy of each, and remembers it for the game', async ({
  page,
  request,
}) => {
  test.setTimeout(90_000);
  const { matchId, host } = await longTable(request);
  await openMatch(page, host, matchId);
  const handZone = `zone-hand:${host.userId}`;

  await expect(page.getByTestId('hand-dock')).toHaveCount(0);
  await page.getByTestId(`hand-float-hand:${host.userId}`).click();

  const dock = page.getByTestId('hand-dock');
  await expect(dock).toBeVisible();
  // One hand and one action bar, both inside the dock — not a second copy
  // left in the page under it.
  await expect(page.getByTestId(handZone)).toHaveCount(1);
  await expect(page.getByTestId('controls-panel')).toHaveCount(1);
  await expect(dock.getByTestId(handZone)).toHaveCount(1);
  await expect(dock.getByTestId('controls-panel')).toHaveCount(1);
  // Above the board, not over it.
  const dockBox = (await dock.boundingBox())!;
  const boardBox = (await page.getByTestId('match-screen').boundingBox())!;
  expect(dockBox.y + dockBox.height).toBeLessThanOrEqual(boardBox.y + 1);

  // Scrolling the melds leaves the dock where it is.
  await scrollBoardTo(page, 'group-t1-seq1');
  expect((await dock.boundingBox())!.y).toBeCloseTo(dockBox.y, 0);

  // Remembered for this game across a reload.
  await page.reload();
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId('hand-dock')).toBeVisible();

  // And pressed again, the hand goes back into the page.
  await page.getByTestId(`hand-float-hand:${host.userId}`).click();
  await expect(page.getByTestId('hand-dock')).toHaveCount(0);
  await expect(page.getByTestId(handZone)).toHaveCount(1);
  await expect(page.getByTestId('controls-panel')).toHaveCount(1);
});

test('a natural dragged from the docked hand swaps for the wild in a meld scrolled far below', async ({
  page,
  request,
}) => {
  test.setTimeout(90_000);
  const { matchId, host } = await longTable(request);
  await openMatch(page, host, matchId, true);
  await expect(page.getByTestId('hand-dock')).toBeVisible();

  await scrollBoardTo(page, 'group-t1-seq1');
  const b = (await page.getByTestId('group-t1-seq1').boundingBox())!;
  await dragCardTo(page, '6S', { x: b.x + b.width / 2, y: b.y + b.height / 2 });

  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t1-seq1']).toBe('5S 6S 7S 8S');
  const after = await onServer(request, matchId, host);
  expect(after.hand).toContain('JOKER2');
  expect(after.hand).not.toContain('6S');
});

test('a card dragged from the docked hand adds to a meld of your own', async ({ page, request }) => {
  test.setTimeout(90_000);
  const { matchId, host } = await longTable(request);
  await openMatch(page, host, matchId, true);

  await scrollBoardTo(page, 'group-t0-9');
  const b = (await page.getByTestId('group-t0-9').boundingBox())!;
  await dragCardTo(page, '9H', { x: b.x + b.width / 2, y: b.y + b.height / 2 });

  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-9']).toBe('9C 9D 9S 9H');
});

test('a meld scrolled up under the dock does not take a card let go over the dock', async ({ page, request }) => {
  test.setTimeout(90_000);
  const { matchId, host } = await longTable(request);
  await openMatch(page, host, matchId, true);

  // Scroll the board until our sets have gone up past the top of it, under
  // the dock — still somewhere in window coordinates, but out of sight.
  const screen = page.getByTestId('match-screen');
  await screen.evaluate((el) => {
    const scroller = (el.scrollHeight > el.clientHeight ? el : el.firstElementChild) as HTMLElement;
    scroller.scrollTop = scroller.scrollHeight;
  });
  await page.waitForTimeout(200);
  const boardTop = (await screen.boundingBox())!.y;
  const hidden = (await page.getByTestId('group-t0-9').boundingBox())!;
  expect(hidden.y + hidden.height).toBeLessThan(boardTop);

  await dragCardTo(page, '9H', { x: hidden.x + hidden.width / 2, y: hidden.y + hidden.height / 2 });
  await page.waitForTimeout(800);
  expect((await onServer(request, matchId, host)).melds['t0-9']).toBe('9C 9D 9S');
});

test.describe('by touch', () => {
  // A finger, not a mouse: a phone takes a vertical swipe for a scroll unless
  // the card was picked up first, and the mouse specs above cannot see that.
  // A plain phone-sized width rather than a device preset, whose inflated
  // innerWidth lays the board out wider than a phone does.
  test.use({ hasTouch: true });

  test('a card held and carried out of the docked hand swaps for the wild below', async ({ page, request }) => {
    test.setTimeout(90_000);
    const { matchId, host } = await longTable(request);
    await openMatch(page, host, matchId, true, 412);
    await expect(page.getByTestId('hand-dock')).toBeVisible();

    await scrollBoardTo(page, 'group-t1-seq1');
    const b = (await page.getByTestId('group-t1-seq1').boundingBox())!;
    const from = await grabPoint(cardByCode(page, '6S'));
    const lift = await touchCarry(page, from, { x: b.x + b.width / 2, y: b.y + b.height / 2 });
    await lift();

    await expect.poll(async () => (await onServer(request, matchId, host)).melds['t1-seq1']).toBe('5S 6S 7S 8S');
    expect((await onServer(request, matchId, host)).hand).toContain('JOKER2');
  });
});
