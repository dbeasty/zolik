import { expect, test, type Page } from '@playwright/test';

import { grabPoint, handCards } from '../helpers/drag';
import { API_BASE, asViewer, type Viewer } from '../helpers/env';
import { selectOnly } from '../helpers/hand';

/**
 * A closed group on a phone is a column of card indices (`CardIndex`), and
 * everything the board draws over a group has to follow it there.
 *
 * `drop-spot.spec.ts` pins the drop gap at desktop width, where a group is
 * overlapped cards. This is the same gesture at 375px, where the group is
 * indices: the gap must open *inside the index column, at an index's size*,
 * the indices after it must step aside by one index, and the card must land
 * where the gap said. A gap sized for a whole card over a column of 14px
 * indices would cover half the group; one placed by the overlapped-card step
 * would open in the wrong place.
 *
 * And the ring that marks a card somebody else added (`table-changes.spec.ts`)
 * must sit on the index that card became.
 *
 * Seeded through the debug-state hatch for the reason `clean-run.spec.ts`
 * gives. Tall rather than phone-tall so the hand and the group are on screen
 * together — a drag is two points on one screen. The *width* is what puts the
 * board in its narrow layout, and it is a plain size, not a device preset
 * (which inflates innerWidth).
 */

type Ctx = import('@playwright/test').APIRequestContext;

test.use({ viewport: { width: 375, height: 1600 } });

const CONTINENTAL = {
  Profile: 'continental',
  DealSize: 13,
  MinSetSize: 3,
  MinRunSize: 4,
  InitialMeldMinimum: 0,
  DiscardDrawMinRound: 0,
  DiscardPickupMode: 'any_from_pile',
  FixedDealCount: 0,
  StaticContract: { Sets: 0, Runs: 0, RequireCleanRun: false },
  MatchEndMode: 'at_score',
  TargetScore: 200,
};

async function tableWithBot(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `idx-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host, auth };
}

/**
 * The bot's run of 7-10 in clubs. `viewerHand` and `botHand` decide what
 * happens next: the drag tests give the viewer the 6 (front only) and the jack
 * (back only); the ring test gives the bot the jack so its own turn adds it.
 */
async function seed(
  request: Ctx,
  matchId: string,
  host: any,
  auth: Record<string, string>,
  viewerHand: string[],
  botHand: string[],
) {
  const live = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const me = host.userId as string;
  const bot = live.players.find((p: any) => p.id !== me).id as string;

  const state = {
    rules: {
      Rules: CONTINENTAL,
      Status: 'active',
      Phase: 'meld',
      GameNumber: 1,
      Round: 1,
      CurrentTurn: me,
      TurnOrder: [me, bot],
      Hands: { [me]: viewerHand, [bot]: botHand },
      Melds: { [bot]: [['7C', '8C', '9C', 'TC']] },
      MeldMeta: { [bot]: [{ MeldID: 'meld_1', Type: 'run', OwnerID: bot }] },
      RoundReqMet: { [me]: true, [bot]: true },
      DrawPile: ['3H', '4H', '6D', '7S', '3D', '5H'],
      DiscardPile: ['QS'],
      DeckSeed: 42,
      GameScores: { [me]: [], [bot]: [] },
      TotalScores: { [me]: 0, [bot]: 0 },
    },
    players: live.players.map((p: any) => ({ id: p.id, name: p.name, isAI: !!p.isAI })),
  };

  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: { state },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'idx',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  // The narrow layout is what this file is about; a wide one would pass the
  // drag assertions against overlapped cards and prove nothing.
  expect(await page.evaluate(() => window.innerWidth)).toBe(375);
}

async function runOnServer(request: Ctx, matchId: string, viewer: Viewer) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(viewer))).json();
  for (const z of b.view?.zones ?? []) {
    for (const g of z.groups ?? []) if (g.id === 'meld_1') return g.cards.join(',');
  }
  return '(gone)';
}

type Box = { x: number; y: number; width: number; height: number };

/**
 * What the group looks like right now: its box, its gap, and each card — by
 * the card code on its wrapper, with the box of the index drawn inside it.
 * `index` is null for a card drawn as a card rather than an index.
 */
async function groupShape(page: Page) {
  return page.evaluate(() => {
    const round = (r: DOMRect) => ({
      x: Math.round(r.x),
      y: Math.round(r.y),
      width: Math.round(r.width),
      height: Math.round(r.height),
    });
    const group = document.querySelector('[data-testid="group-meld_1"]');
    if (!group) return null;
    const hole = group.querySelector('[data-testid^="group-slice-"]');
    return {
      box: round(group.getBoundingClientRect()),
      hole: hole ? round(hole.getBoundingClientRect()) : null,
      cards: Array.from(group.querySelectorAll('[data-card]'))
        .filter((c) => /^[0-9TJQKA]/.test(c.getAttribute('data-card') ?? ''))
        .map((c) => {
          const index = c.querySelector('[data-testid^="index-meld_1-"]');
          return {
            card: c.getAttribute('data-card') ?? '',
            y: Math.round(c.getBoundingClientRect().y),
            index: index ? round(index.getBoundingClientRect()) : null,
          };
        }),
    };
  });
}

/** Picks up `code` and holds it over the group, without letting go. */
async function carryOverGroup(page: Page, code: string) {
  const hand = await handCards(page);
  const at = hand.indexOf(code);
  expect(at, `${code} is not in hand: ${hand}`).toBeGreaterThanOrEqual(0);

  const group = page.getByTestId('group-meld_1');
  await expect(group).toBeVisible({ timeout: 15_000 });
  const target = await group.boundingBox();
  const from = await grabPoint(page.locator('[data-testid^="card-hand:"]').nth(at));
  if (!target) throw new Error('the group has no box');

  const to = { x: target.x + target.width / 2, y: target.y + target.height * 0.3 };
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.waitForTimeout(200);
  await page.mouse.move(from.x + 40, from.y + 60, { steps: 10 });
  await page.waitForTimeout(150);
  await page.mouse.move(to.x, to.y, { steps: 20 });
  await page.waitForTimeout(350);
}

function near(actual: number, expected: number, what: string, slack = 1) {
  expect(Math.abs(actual - expected), `${what}: ${actual} vs ${expected}`).toBeLessThanOrEqual(slack);
}

test.describe('a group drawn as card indices on a narrow screen', () => {
  test('a drop gap opens inside the index column, an index tall, and the card lands there', async ({
    page,
    request,
  }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seed(request, matchId, host, auth, ['6C', 'JC', 'KD', 'KS', '2S', '3D', '4H', '9S'], [
      '2D',
      '3S',
      '4D',
      '9D',
    ]);
    await openMatch(page, host, matchId);
    await handCards(page);

    const resting = (await groupShape(page))!;
    expect(resting, 'the run is not on the table').toBeTruthy();
    expect(resting.hole, 'a gap is open with nothing being carried').toBeNull();
    // Closed on a phone: every card is an index, in order.
    expect(resting.cards.map((c) => c.card)).toEqual(['7C', '8C', '9C', 'TC']);
    expect(resting.cards.every((c) => c.index), 'the closed group is not drawn as indices').toBe(true);
    const index = resting.cards[0].index as Box;
    const step = resting.cards[1].y - resting.cards[0].y;
    // An index, not a card: well short of the ~48px compact card it replaces.
    expect(index.height).toBeLessThan(24);
    expect(step).toBeLessThan(index.height + 6);

    await carryOverGroup(page, '6C');
    try {
      const open = (await groupShape(page))!;
      expect(open.hole, 'no gap opened in the group').toBeTruthy();
      const hole = open.hole!;

      // Still indices while the card is in flight — the gap opens in the
      // column the player is looking at, not in a group that swapped back to
      // cards under them.
      expect(open.cards.every((c) => c.index)).toBe(true);

      // An index's size: the gap is the space one index takes, not a card's.
      near(hole.height, index.height, 'gap height vs an index');

      // At the front, where the 6 goes — the top of the column, which is
      // where the first index sat at rest — and inside the group's box.
      near(hole.y, resting.cards[0].y, 'gap top vs where the first index was');
      expect(hole.y).toBeGreaterThanOrEqual(open.box.y);
      expect(hole.y + hole.height).toBeLessThanOrEqual(open.box.y + open.box.height);
      expect(hole.x).toBeGreaterThanOrEqual(open.box.x);
      expect(hole.x + hole.width).toBeLessThanOrEqual(open.box.x + open.box.width);

      // Every index stepped down by exactly one index step, so the gap is a
      // space and not something drawn over the 7.
      open.cards.forEach((c, i) => near(c.y - resting.cards[i].y, step, `${c.card} stepped aside`));

      // The group's own box must not move: drops are hit-tested against it.
      expect(open.box).toEqual(resting.box);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(400);
    }

    // The gap told the truth — on the server, and on screen as indices in
    // that order.
    await expect
      .poll(() => runOnServer(request, matchId, host), { timeout: 10_000 })
      .toBe('6C,7C,8C,9C,TC');
    await expect
      .poll(async () => (await groupShape(page))!.cards.map((c) => c.card).join(','))
      .toBe('6C,7C,8C,9C,TC');
    const landed = (await groupShape(page))!;
    expect(landed.hole).toBeNull();
    expect(landed.cards.every((c) => c.index), 'the grown group is not drawn as indices').toBe(true);
    // Stacked evenly, one step apart, top to bottom.
    for (let i = 1; i < landed.cards.length; i++) {
      near(landed.cards[i].y - landed.cards[i - 1].y, step, `${landed.cards[i].card} spacing`);
    }
  });

  test('a card for the far end opens the gap after the last index, and nothing moves', async ({
    page,
    request,
  }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seed(request, matchId, host, auth, ['6C', 'JC', 'KD', 'KS', '2S', '3D', '4H', '9S'], [
      '2D',
      '3S',
      '4D',
      '9D',
    ]);
    await openMatch(page, host, matchId);
    await handCards(page);

    const resting = (await groupShape(page))!;
    expect(resting.cards.every((c) => c.index)).toBe(true);
    const index = resting.cards[0].index as Box;
    const step = resting.cards[1].y - resting.cards[0].y;

    await carryOverGroup(page, 'JC');
    try {
      const open = (await groupShape(page))!;
      expect(open.hole, 'no gap opened in the group').toBeTruthy();
      const hole = open.hole!;
      const last = open.cards[open.cards.length - 1];

      near(hole.height, index.height, 'gap height vs an index');
      // One index step after the last index — where a fifth index will sit.
      near(hole.y, last.y + step, 'gap top vs one step after the last index');
      expect(open.cards.map((c) => c.y)).toEqual(resting.cards.map((c) => c.y));
      expect(open.box).toEqual(resting.box);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(400);
    }

    await expect
      .poll(() => runOnServer(request, matchId, host), { timeout: 10_000 })
      .toBe('7C,8C,9C,TC,JC');
    await expect
      .poll(async () => (await groupShape(page))!.cards.map((c) => c.card).join(','))
      .toBe('7C,8C,9C,TC,JC');
  });

  test("a card somebody else added is ringed on the index it became", async ({ page, request }) => {
    test.setTimeout(60_000);
    const { matchId, host, auth } = await tableWithBot(request);
    // The viewer only has to discard; the bot holds the jack that extends its
    // own run, so its turn changes this group for certain.
    await seed(request, matchId, host, auth, ['2H', '5S', '9D', 'KS', 'QD'], ['JC', '2D', '4S', '8H']);
    await openMatch(page, host, matchId);
    await handCards(page);
    await expect(page.locator('[data-testid^="card-mark-"]')).toHaveCount(0);

    await selectOnly(page, ['2H']);
    await page.getByTestId('offer-discard').click();

    await expect
      .poll(() => runOnServer(request, matchId, host), { timeout: 30_000 })
      .toBe('7C,8C,9C,TC,JC');

    // The jack is the fifth card, and the ring is on the fifth index.
    const ring = page.getByTestId('card-mark-meld_1-4');
    await expect(ring).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('[data-testid^="card-mark-meld_1-"]')).toHaveCount(1);

    const shape = (await groupShape(page))!;
    expect(shape.cards.map((c) => c.card)).toEqual(['7C', '8C', '9C', 'TC', 'JC']);
    expect(shape.cards.every((c) => c.index), 'the group is not drawn as indices').toBe(true);
    const jack = shape.cards[4].index as Box;
    const r = (await ring.boundingBox())!;
    // Hugging the index — a ring sized for a card would reach over its
    // neighbours and mark the wrong ones.
    near(r.x, jack.x - 1, 'ring left', 2);
    near(r.y, jack.y - 1, 'ring top', 2);
    near(r.width, jack.width + 2, 'ring width', 2);
    near(r.height, jack.height + 2, 'ring height', 2);
  });
});
