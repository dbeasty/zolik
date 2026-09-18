import { expect, test, type Page } from '@playwright/test';

import { grabPoint, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * Where a card is about to land, shown before it lands.
 *
 * The hand's half of this lives in `hand-order.spec.ts` ("the hand comes apart
 * to show where the card will land"). This file is the other half: a meld,
 * which is drawn as a stack running top to bottom rather than a fan running
 * left to right, and which belongs to a module rather than to the shell.
 *
 * That last part is the interesting one. The shell may not know what "front"
 * means — a run's ends are a rule of Žolíky and nothing here is allowed a
 * rule of Žolíky. So the module says where each of its positions *is*, as an
 * index among the group's own cards (`Placement.slots`), and the board opens a
 * gap there without ever learning the word. A run that takes a card at one end
 * only is the case that forces it: it offers a single position, and "the first
 * of one" names no end at all.
 *
 * The position is seeded through the dev-only debug-state hatch rather than
 * played to, for the reason `clean-run.spec.ts` gives — waiting for the right
 * cards to fall is a test that mostly does not run.
 */

type Ctx = import('@playwright/test').APIRequestContext;

test.use({ viewport: { width: 1280, height: 1900 } });

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
    data: { guestName: `spot-${Math.random().toString(36).slice(2, 10)}` },
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
 * The bot holding a run of 7-8-9-10 in clubs, and the viewer on turn holding
 * the 6 that only fits in front of it and the jack that only fits after —
 * one card for each end, so the two ends can be told apart by which card is
 * being carried rather than by where the pointer happens to be.
 */
async function seedRun(request: Ctx, matchId: string, host: any, auth: Record<string, string>) {
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
      Hands: {
        [me]: ['6C', 'JC', 'KD', 'KS', '2S', '3D', '4H', '9S'],
        [bot]: ['2D', '3S', '4D', '9D'],
      },
      Melds: { [bot]: [['7C', '8C', '9C', 'TC']] },
      MeldMeta: { [bot]: [{ MeldID: 'meld_1', Type: 'run', OwnerID: bot }] },
      RoundReqMet: { [me]: true, [bot]: true },
      DrawPile: ['2H', '3H', '4H'],
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
  return { me };
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
      username: 'spot',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The bot's run, as the server has it — the only witness to where a card landed. */
async function runOnServer(request: Ctx, matchId: string, userId: string) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  for (const z of b.view?.zones ?? []) {
    for (const g of z.groups ?? []) if (g.id === 'meld_1') return g.cards.join(',');
  }
  return '(gone)';
}

/** Where the meld's gap and its cards are drawn, right now. */
async function meldShape(page: Page) {
  return page.evaluate(() => {
    const group = document.querySelector('[data-testid="group-meld_1"]');
    if (!group) return null;
    const hole = group.querySelector('[data-testid^="group-slice-"]');
    const box = group.getBoundingClientRect();
    return {
      // The group's own measured box. Every drop on the board is hit-tested
      // against this, so it must be the same with the gap open as without.
      box: `${Math.round(box.x)},${Math.round(box.y)},${Math.round(box.width)},${Math.round(box.height)}`,
      hole: hole
        ? {
            y: Math.round(hole.getBoundingClientRect().y),
            height: Math.round(hole.getBoundingClientRect().height),
          }
        : null,
      // Every card in the stack, top edge first, in the order they are drawn.
      cards: Array.from(group.querySelectorAll('[aria-label]'))
        .map((c) => ({
          card: c.getAttribute('aria-label') ?? '',
          y: Math.round(c.getBoundingClientRect().y),
          height: Math.round(c.getBoundingClientRect().height),
        }))
        .filter((c) => /^[0-9TJQKA]/.test(c.card)),
    };
  });
}

/** Picks up `code` and holds it over the meld, without letting go. */
async function carryOverMeld(page: Page, code: string) {
  const hand = await handCards(page);
  const index = hand.indexOf(code);
  expect(index, `${code} is not in hand: ${hand}`).toBeGreaterThanOrEqual(0);

  const meld = page.getByTestId('group-meld_1');
  await expect(meld).toBeVisible({ timeout: 15_000 });
  const target = await meld.boundingBox();
  const from = await grabPoint(page.locator('[data-testid^="card-hand:"]').nth(index));
  if (!target) throw new Error('the meld has no box');

  const to = { x: target.x + target.width / 2, y: target.y + target.height * 0.3 };
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.waitForTimeout(200);
  await page.mouse.move(from.x + 80, from.y - 60, { steps: 10 });
  await page.waitForTimeout(150);
  await page.mouse.move(to.x, to.y, { steps: 20 });
  await page.waitForTimeout(350);
  return to;
}

test.describe('the place a card is about to go', () => {
  test('a meld opens a gap at the end the card would join', async ({ page, request }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRun(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    const resting = await meldShape(page);
    expect(resting, 'the run is not on the table').toBeTruthy();
    expect(resting!.hole, 'a gap is open with nothing being carried').toBeNull();
    expect(resting!.cards.map((c) => c.card)).toEqual(['7C', '8C', '9C', 'TC']);
    const closed = resting!.cards[0];

    await carryOverMeld(page, '6C');
    try {
      const open = await meldShape(page);
      expect(open!.hole, 'no gap opened in the meld').toBeTruthy();

      // A gap a card could go in, judged against the cards it sits among —
      // the whole complaint was a drop spot too small to see.
      expect(open!.hole!.height).toBeGreaterThan(closed.height * 0.9);

      // At the *front*: above the 7, which is where a 6 goes. The board is
      // told this as a number by the module; it has no idea what a run is.
      expect(
        open!.hole!.y,
        'the gap is not above the first card of the run',
      ).toBeLessThan(open!.cards[0].y);

      // And the run has stepped down out of the way, by a card, so the gap is
      // a space rather than something drawn over the 7.
      expect(open!.cards[0].y - closed.y).toBeGreaterThan(closed.height * 0.9);

      // The one thing that must not move. Drops are hit-tested against rects
      // read once at the start of the drag, so a target that changed size
      // mid-drag would send every card after it somewhere else.
      expect(open!.box, 'the meld changed size while the gap was open').toBe(resting!.box);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(400);
    }

    // The gap told the truth: the 6 is now the front of the run.
    await expect
      .poll(() => runOnServer(request, matchId, host.userId), { timeout: 10_000 })
      .toBe('6C,7C,8C,9C,TC');
  });

  test('the other end of the same run opens a gap after the last card', async ({
    page,
    request,
  }) => {
    // The same meld and the same gesture, with the card swapped. Nothing about
    // the pointer changes — it is held over the top of the run both times —
    // so the only thing that can move the gap to the far end is the module
    // having said where the jack goes.
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRun(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    const resting = await meldShape(page);
    expect(resting, 'the run is not on the table').toBeTruthy();

    await carryOverMeld(page, 'JC');
    try {
      const open = await meldShape(page);
      expect(open!.hole, 'no gap opened in the meld').toBeTruthy();

      const last = open!.cards[open!.cards.length - 1];
      expect(open!.cards.map((c) => c.card)).toEqual(['7C', '8C', '9C', 'TC']);
      expect(open!.hole!.y, 'the gap is not below the last card of the run').toBeGreaterThan(last.y);

      // And no card has moved: a gap past the end of the stack has nothing
      // after it to step aside. The run is exactly where it was resting.
      expect(open!.cards.map((c) => c.y)).toEqual(resting!.cards.map((c) => c.y));
      expect(open!.box, 'the meld changed size while the gap was open').toBe(resting!.box);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(400);
    }

    await expect
      .poll(() => runOnServer(request, matchId, host.userId), { timeout: 10_000 })
      .toBe('7C,8C,9C,TC,JC');
  });
});
