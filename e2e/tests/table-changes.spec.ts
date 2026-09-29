import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { selectOnly } from '../helpers/hand';

/**
 * What changed on the table while you were not looking, and what to do now.
 *
 * Players kept missing a card another player had added to a meld: a stacked
 * meld shows only a new corner, and nothing else on the board said anything
 * had happened. After the bot's turn, the meld it changed must be marked, its
 * owner's panel must say so on its header, and the viewer's turn must open
 * with a line saying what they can do.
 *
 * Seeded through the debug-state hatch, for the reason clean-run.spec.ts
 * gives: the bot holds a card that extends its own run, so its turn changes
 * the table for certain rather than by luck of the deal.
 */

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

async function seededTable(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `marks-${Math.random().toString(36).slice(2, 10)}` },
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

  const live = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const me = host.userId as string;
  const bot = live.players.find((p: any) => p.id !== me).id as string;

  // The viewer has drawn and only has to discard. The bot holds the jack that
  // extends its run, and spare cards so that laying it off does not empty its
  // hand on a deal that is not the last.
  const state = {
    rules: {
      Rules: CONTINENTAL,
      Status: 'active',
      Phase: 'meld',
      GameNumber: 1,
      Round: 1,
      CurrentTurn: me,
      TurnOrder: [me, bot],
      Hands: { [me]: ['2H', '5S', '9D', 'KS', 'QD'], [bot]: ['JC', '2D', '4S', '8H'] },
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
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: { state },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  return { matchId, host, me, bot };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.setViewportSize({ width: 1280, height: 1200 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'marks',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function meldOnServer(request: Ctx, matchId: string, userId: string) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  for (const z of b.view?.zones ?? []) {
    for (const g of z.groups ?? []) if (g.id === 'meld_1') return g.cards as string[];
  }
  return [];
}

test("a meld somebody else changed is marked, and the viewer's turn says what to do", async ({
  page,
  request,
}) => {
  test.setTimeout(60_000);
  const { matchId, host } = await seededTable(request);
  await openMatch(page, host, matchId);
  await handCards(page);

  // Nothing has changed since the viewer sat down, so nothing is marked.
  await expect(page.locator('[data-testid^="group-mark-"]')).toHaveCount(0);
  await expect(page.getByTestId('turn-step')).toContainText('Your turn');

  await selectOnly(page, ['2H']);
  await page.getByTestId('offer-discard').click();

  // The bot's turn: it extends its run. The viewer sees it marked when their
  // turn comes round.
  await expect
    .poll(async () => (await meldOnServer(request, matchId, host.userId)).length, { timeout: 30_000 })
    .toBeGreaterThan(4);
  await expect(page.getByTestId('group-mark-meld_1')).toBeVisible({ timeout: 15_000 });
  await expect(page.locator('[data-testid^="card-mark-meld_1-"]').first()).toBeVisible();
  await expect(page.locator('[data-testid^="zone-marks-"]').first()).toBeVisible();

  // And it stays up for the viewer's own turn, which is when it is needed.
  await expect(page.getByTestId('turn-step')).toContainText('Your turn', { timeout: 15_000 });
  await expect(page.getByTestId('group-mark-meld_1')).toBeVisible();
});
