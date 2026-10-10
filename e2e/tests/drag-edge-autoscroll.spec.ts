import { expect, test, type Page } from '@playwright/test';

import { carryPointOver, grabPoint, handCards, release } from '../helpers/drag';
import { API_BASE, asViewer, type Viewer } from '../helpers/env';
import { cardByCode, selectOnly } from '../helpers/hand';

/**
 * Carrying a card toward a meld the window has scrolled away from.
 *
 * The hand sits above the melds, so a lay-off onto a meld below the fold means
 * reaching it with the button held down, when nothing scrolls. While a drag is
 * in flight, a pointer held in the bottom band of the window scrolls the board,
 * and the drop rects follow. This is the whole gesture, end to end, with the
 * server's copy of the meld as the witness; the feature shipped without one,
 * and the layoff specs that assumed a meld in view broke under it unnoticed.
 */

const SHORT = 900;

type Ctx = import('@playwright/test').APIRequestContext;

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
    data: { guestName: `chain-${Math.random().toString(36).slice(2, 10)}` },
  });
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', options: {} },
  });
  const { matchId } = await created.json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host, auth };
}

/**
 * The reported board: the bot holding a run of 7-8-9-10 in clubs, and the
 * viewer down, on turn, holding the 5 and the 6 of clubs that reach it only
 * as a pair — plus spare cards, because a lay-off may not empty a hand on a
 * deal that is not the last.
 */
async function seedRunAndGapCards(request: Ctx, matchId: string, host: any, auth: Record<string, string>) {
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
      Hands: { [me]: ['5C', '6C', 'KD', 'KS'], [bot]: ['2D', '3S', '4D', '9D'] },
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
  return { me, bot };
}

async function openMatch(page: Page, host: any, matchId: string) {
  // Both the hand and the meld have to be on screen at once: a drag is two
  // points on one screen, and scrolling to the target would take the source
  // out from under the pointer.
  await page.setViewportSize({ width: 1280, height: SHORT });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'chain',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The bot's run, as the server has it. */
async function meldOnServer(request: Ctx, matchId: string, viewer: Viewer) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(viewer))).json();
  for (const z of b.view?.zones ?? []) {
    for (const g of z.groups ?? []) if (g.id === 'meld_1') return g.cards.join(',');
  }
  return '(gone)';
}

test.describe('a drag near the window edge scrolls the board', () => {
  test('a pair carried to the bottom edge reaches a meld that started below the fold', async ({ page, request }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRunAndGapCards(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    const meld = page.getByTestId('group-meld_1');
    const before = await meld.boundingBox();
    expect(before, 'the meld should be on the page').toBeTruthy();
    expect(
      before!.y + before!.height / 2,
      'the premise: the meld starts below the fold, out of a pointer\'s reach',
    ).toBeGreaterThan(SHORT);

    await selectOnly(page, ['5C', '6C']);
    const grab = await grabPoint(cardByCode(page, '6C'));
    // Down into the bottom band, and hold there: only a timer can scroll for a
    // pointer that has stopped moving.
    await carryPointOver(page, grab, { x: grab.x, y: SHORT - 20 });
    await expect
      .poll(async () => (await meld.boundingBox())!.y, { timeout: 10_000, message: 'the board should scroll while the card is held at the edge' })
      .toBeLessThan(SHORT / 2);

    // Now the meld is under a pointer a person could put there.
    // Pull the pointer out of the band first, or the board keeps scrolling
    // under it, and aim at the part of the meld that is on screen.
    const at = (await meld.boundingBox())!;
    await page.mouse.move(at.x + at.width / 2, at.y + 60, { steps: 10 });
    await page.waitForTimeout(200);
    await release(page);

    await expect
      .poll(() => meldOnServer(request, matchId, host), { timeout: 10_000 })
      .toBe('5C,6C,7C,8C,9C,TC');
  });

  test('a drag that starts in the bottom band does not carry the board away', async ({ page, request }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRunAndGapCards(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    // A card dragged sideways along the hand, and held, must leave the page where it was.
    const scrollOf = () => page.evaluate(() => Math.max(0, ...[...document.querySelectorAll('*')].map((e) => e.scrollTop)));
    const start = await scrollOf();
    const grab = await grabPoint(cardByCode(page, 'KD'));
    await carryPointOver(page, grab, { x: grab.x + 120, y: grab.y });
    await page.waitForTimeout(800);
    expect(await scrollOf()).toBe(start);
    await release(page);
  });
});
