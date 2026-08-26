import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * Žolík Classic's house rule: you are not "down" — and so may not lay off on
 * anyone's meld — until a joker-free run is on the table. Sets alone never
 * satisfy it, however many you lay.
 *
 * Reported from a real game: three sets down, a 5♠ in hand, a meld of fives
 * sitting on the table, and every lay-off silently refused. The engine was
 * right; the screen said "Lay your own initial meld first" to a player
 * looking at three melds they had just laid, which reads as a broken feature
 * rather than an unmet rule.
 *
 * So what is pinned here is what the player *reads*, not just what the engine
 * decides — and that the rule can be switched off by a table that does not
 * want it, which is the other half of the report.
 *
 * The position is seeded through the dev-only debug-state hatch rather than
 * played out: reaching "three sets and no run" by playing takes a deal's
 * worth of luck, and a test that only runs when the cards fall right is a
 * test that mostly does not run.
 */

type Ctx = import('@playwright/test').APIRequestContext;

const CLASSIC = (requireCleanRun: boolean) => ({
  Profile: 'zolik_classic',
  DealSize: 13,
  MinSetSize: 3,
  MinRunSize: 3,
  InitialMeldMinimum: 0,
  DiscardDrawMinRound: 0,
  DiscardPickupMode: 'any_from_pile',
  JokerDiscardRestricted: true,
  FixedDealCount: 0,
  StaticContract: { Sets: 0, Runs: 0, RequireCleanRun: requireCleanRun },
  MatchEndMode: 'at_score',
  TargetScore: 200,
});

async function tableWithBot(request: Ctx, options: Record<string, number>) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `clean-${Math.random().toString(36).slice(2, 10)}` },
  });
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', variation: 'zolik_classic', options },
  });
  const { matchId } = await created.json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host, auth };
}

/**
 * The reported board: three sets of the viewer's own, no run anywhere, a 5♠
 * still in hand, and the bot holding a meld of fives to aim it at.
 *
 * `RoundReqMet` is seeded to whatever the engine itself would have written
 * when that third set went down under this setting — false while the clean
 * run is required, true once it is not (see the Go tests in
 * rules/zolik_classic_test.go, which drive the real ValidateMeldAction to
 * establish both).
 */
async function seedReportedBoard(
  request: Ctx,
  matchId: string,
  host: any,
  auth: Record<string, string>,
  requireCleanRun: boolean,
  meldsLaidThisTurn: number,
) {
  const live = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const me = host.userId as string;
  const bot = live.players.find((p: any) => p.id !== me).id as string;

  const state = {
    rules: {
      Rules: CLASSIC(requireCleanRun),
      Status: 'active',
      Phase: 'meld',
      GameNumber: 1,
      Round: 3,
      CurrentTurn: me,
      TurnOrder: [me, bot],
      Hands: { [me]: ['KD', 'KS', '7C', '7D', '5S'], [bot]: ['2C', '3S', '4D', '9D'] },
      Melds: {
        [me]: [['AC', 'AD', 'AH'], ['4C', '4H', '4S'], ['9C', '9H', '9S']],
        [bot]: [['5C', '5D', '5H']],
      },
      MeldMeta: {
        [me]: [
          { MeldID: 'meld_1', Type: 'set', OwnerID: me },
          { MeldID: 'meld_3', Type: 'set', OwnerID: me },
          { MeldID: 'meld_5', Type: 'set', OwnerID: me },
        ],
        [bot]: [{ MeldID: 'meld_2', Type: 'set', OwnerID: bot }],
      },
      RoundReqMet: { [me]: !requireCleanRun, [bot]: true },
      MeldsLaidThisTurn: meldsLaidThisTurn,
      DrawPile: ['2C', '3C', '4C'],
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
  await page.setViewportSize({ width: 1280, height: 1200 });
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: 'clean',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The reason text the shell prints under a disabled control. */
async function reasonUnder(page: Page, label: string) {
  const control = page.locator('[data-testid^="offer-"]', { hasText: label }).first();
  await expect(control).toBeVisible();
  return (await control.locator('xpath=..').innerText()).replace(/\s+/g, ' ');
}

test.describe("Žolík Classic's clean-run rule", () => {
  test('a refused lay-off says a joker-free run is what is missing', async ({ page, request }) => {
    const { matchId, host, auth } = await tableWithBot(request, {});
    await seedReportedBoard(request, matchId, host, auth, true, 1);
    await openMatch(page, host, matchId);
    await handCards(page);

    // The whole point of the report: not "no", but *why*. "Lay your own
    // initial meld first" was the old answer, to a player looking at three
    // melds of their own.
    expect(await reasonUnder(page, 'Lay off')).toContain('joker-free run');

    // And the turn cannot simply be walked away from with an unfinished
    // lay-down on the table — that is how the dead position was reached.
    expect(await reasonUnder(page, 'Discard')).toContain('undo');
  });

  test('with the rule turned off, the same card lays off onto the meld', async ({
    page,
    request,
  }) => {
    const { matchId, host, auth } = await tableWithBot(request, { requireCleanRun: 0 });
    await seedReportedBoard(request, matchId, host, auth, false, 0);
    await openMatch(page, host, matchId);
    await handCards(page);

    // The exact move from the report: the 5♠ (last card in the seeded hand)
    // onto the opponent's meld of fives.
    await dragLocatorTo(page, page.locator('[data-testid^="card-hand:"]').nth(4), page.getByTestId('group-meld_2'));

    // The server is the witness — a card that only moved on screen proves
    // nothing about the rule.
    await expect
      .poll(
        async () => {
          const b = await (await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`)).json();
          for (const z of b.view?.zones ?? []) {
            for (const g of z.groups ?? []) if (g.id === 'meld_2') return g.cards.join(',');
          }
          return '(gone)';
        },
        { timeout: 10_000 },
      )
      .toBe('5C,5D,5H,5S');
  });
});
