import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * A game's finished tables, reached from the game itself.
 *
 * The main menu row resumes one table, so before this the older finished games
 * of a game were only in the account menu's list, mixed in with every other
 * game. Both the menu row and the game's page now lead to that list narrowed
 * to the game, on the Finished tab.
 *
 * Finished tables are seeded through the debug-state hatch: playing a whole
 * match out in a browser would make this the slowest spec in the suite.
 */

type Req = Parameters<Parameters<typeof test>[1]>[0]['request'];

async function startedTable(request: Req, auth: Record<string, string>, moduleId: string) {
  const { matchId } = await (
    await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId } })
  ).json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth, data: {} });
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return matchId as string;
}

/** The seeded position: any valid zolik board, since only the status matters. */
function board(me: string, other: string) {
  return {
    rules: {
      Rules: {
        Profile: 'zolik_classic',
        DealSize: 13,
        MinSetSize: 3,
        MinRunSize: 3,
        InitialMeldMinimum: 0,
        DiscardDrawMinRound: 0,
        DiscardPickupMode: 'any_from_pile',
        JokerDiscardRestricted: true,
        FixedDealCount: 0,
        StaticContract: { Sets: 0, Runs: 0, RequireCleanRun: false },
        MatchEndMode: 'at_score',
        TargetScore: 200,
      },
      Status: 'active',
      Phase: 'meld',
      GameNumber: 1,
      Round: 3,
      CurrentTurn: me,
      TurnOrder: [me, other],
      Hands: { [me]: ['KD', 'KS', '7C', '7D', '5S'], [other]: ['2C', '3S', '4D', '9D'] },
      Melds: { [me]: [['AC', 'AD', 'AH']], [other]: [['5C', '5D', '5H']] },
      MeldMeta: {
        [me]: [{ MeldID: 'meld_1', Type: 'set', OwnerID: me }],
        [other]: [{ MeldID: 'meld_2', Type: 'set', OwnerID: other }],
      },
      RoundReqMet: { [me]: true, [other]: true },
      MeldsLaidThisTurn: 0,
      DrawPile: ['2C', '3C', '4C'],
      DiscardPile: ['QS'],
      DeckSeed: 42,
      GameScores: { [me]: [], [other]: [] },
      TotalScores: { [me]: 0, [other]: 0 },
    },
  };
}

/** Marks a started zolik table finished. */
async function finish(request: Req, auth: Record<string, string>, matchId: string, me: string) {
  const seated = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const other = seated.players.find((p: { id: string }) => p.id !== me).id;
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: { state: board(me, other), status: 'completed' },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

test.describe('previous games of one game', () => {
  test('the menu row and the game page both open that game’s finished list', async ({
    page,
    request,
  }) => {
    const host = await (
      await request.post(`${API_BASE}/auth/guest`, {
        data: { guestName: `prev-${Math.random().toString(36).slice(2, 8)}` },
      })
    ).json();
    const auth = { Authorization: `Bearer ${host.accessToken}` };

    const done1 = await startedTable(request, auth, 'zolik');
    const done2 = await startedTable(request, auth, 'zolik');
    // Another game's finished table is not seeded (its board is its own), so
    // the filter is checked against a table of it that is still running.
    const other = await startedTable(request, auth, 'prsi');
    const running = await startedTable(request, auth, 'zolik');
    await finish(request, auth, done1, host.userId);
    await finish(request, auth, done2, host.userId);

    await page.addInitScript(
      (s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)),
      {
        accessToken: host.accessToken,
        refreshToken: host.refreshToken,
        userId: host.userId,
        username: host.guestName ?? 'prev',
        isGuest: true,
      },
    );
    await seedIntroSeen(page);

    const expectFinishedZolik = async () => {
      await expect(page.getByTestId('mine-tab-finished')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId(`mine-row-${done1}`)).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId(`mine-row-${done2}`)).toBeVisible();
      // Narrowed to the game, and to the finished tab.
      await expect(page.getByTestId(`mine-row-${other}`)).toHaveCount(0);
      await expect(page.getByTestId(`mine-row-${running}`)).toHaveCount(0);
      // Each finished game can be stepped through.
      await expect(page.getByTestId(`mine-replay-${done1}`)).toBeVisible();
    };

    // From the main menu row.
    await page.goto('/');
    await page.getByTestId('picker-zolik-previous').click();
    await expectFinishedZolik();

    // From the game's own page.
    await page.goto('/lobby/games?moduleId=zolik');
    await page.getByTestId('previous-games-zolik').click();
    await expectFinishedZolik();

    // The account menu's list is unchanged: every game, in progress.
    await page.goto('/lobby/mine');
    await expect(page.getByTestId(`mine-row-${running}`)).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId(`mine-row-${done1}`)).toHaveCount(0);
  });
});
