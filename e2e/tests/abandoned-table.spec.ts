import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * What a table looks like after the sweeper has set it aside, and the way back.
 *
 * Reported from a real link: a match left sitting for a while, opened again
 * hours later, and the screen said "Connecting…" for ever. Two separate dead
 * ends were behind that, and this pins the one a signed-in player hits.
 *
 * Before this, `abandoned` fell through every branch that named a status. The
 * dot stayed green, the explainer said the match was "in progress — everything
 * is connected and moving normally", the end-of-match banner was gated on
 * `completed` so nothing appeared, and every control was refused by an engine
 * that knew perfectly well the game was over. A board that says it is fine
 * while refusing every move is indistinguishable from a bug, which is exactly
 * what it was reported as — the same failure the shell's own comments record
 * being fixed once already for *finished* matches.
 *
 * The position is seeded through the dev-only debug-state hatch rather than
 * played into: waiting out AbandonWindow in a browser test would make this the
 * slowest spec in the suite for no extra confidence.
 */

type Ctx = Parameters<Parameters<typeof test>[1]>[0]['request'];

const CLASSIC = {
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
};

/** A started table whose only other seat is a bot — the case resume is for. */
async function tableWithBot(request: Ctx) {
  const host = await (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `aband-${Math.random().toString(36).slice(2, 10)}` },
    })
  ).json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const { matchId } = await (
    await request.post(`${API_BASE}/matches`, {
      headers: auth,
      data: { moduleId: 'zolik', variation: 'zolik_classic' },
    })
  ).json();
  const { playerId: bot } = await (
    await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth, data: {} })
  ).json();
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host, auth, bot };
}

/** Seeds a mid-deal position and marks the table the way the reaper does. */
async function seedAbandoned(request: Ctx, matchId: string, me: string, bot: string, auth: Record<string, string>) {
  const state = {
    rules: {
      Rules: CLASSIC,
      Status: 'active',
      Phase: 'meld',
      GameNumber: 1,
      Round: 3,
      CurrentTurn: me,
      TurnOrder: [me, bot],
      Hands: { [me]: ['KD', 'KS', '7C', '7D', '5S'], [bot]: ['2C', '3S', '4D', '9D'] },
      Melds: { [me]: [['AC', 'AD', 'AH']], [bot]: [['5C', '5D', '5H']] },
      MeldMeta: {
        [me]: [{ MeldID: 'meld_1', Type: 'set', OwnerID: me }],
        [bot]: [{ MeldID: 'meld_2', Type: 'set', OwnerID: bot }],
      },
      RoundReqMet: { [me]: true, [bot]: true },
      MeldsLaidThisTurn: 0,
      DrawPile: ['2C', '3C', '4C'],
      DiscardPile: ['QS'],
      DeckSeed: 42,
      GameScores: { [me]: [], [bot]: [] },
      TotalScores: { [me]: 0, [bot]: 0 },
    },
  };
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: { state, status: 'abandoned' },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
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
      username: 'aband',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
}

test.describe('a table the sweeper set aside', () => {
  test('says so, and offers the way back', async ({ page, request }) => {
    const { matchId, host, auth, bot } = await tableWithBot(request);
    await seedAbandoned(request, matchId, host.userId, bot, auth);
    await openMatch(page, host, matchId);

    // The banner exists at all. It did not before: only `completed` drew one,
    // so an abandoned table showed the dead position and nothing else.
    await expect(page.getByTestId('match-over')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId('match-over-title')).toHaveText('Table set aside');
    // And says the thing a player actually needs to know, which is that the
    // game is still there.
    await expect(page.getByTestId('match-over-outcome')).toContainText(
      'exactly where you left them',
    );

    // The status the header reports is the real one, not "in progress".
    await expect(page.getByTestId('match-status')).toHaveText('abandoned');

    // Resuming carries this game on rather than starting a new one, so it is
    // offered above the rematch.
    await expect(page.getByTestId('match-over-resume')).toBeVisible();
    await page.getByTestId('match-over-resume').click();

    // The banner goes on its own: no navigation, because the socket is still
    // in the table's room and the revived board arrives as an ordinary state
    // message.
    await expect(page.getByTestId('match-over')).toBeHidden({ timeout: 30_000 });
    await expect(page.getByTestId('match-status')).toHaveText('active');

    // And the server agrees, which is what makes the board above real rather
    // than a client that merely stopped drawing the banner.
    const after = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
    expect(after.status).toBe('active');
  });

  // The other half of the report: a link opened without a session used to sit
  // on "Connecting…" for ever, because the socket URL needs a token and the
  // hook is handed null without one — an input it neither opens nor fails on.
  test('a link followed while signed out goes to sign-in, not a spinner', async ({ page, request }) => {
    const { matchId } = await tableWithBot(request);

    await page.setViewportSize({ width: 1280, height: 1200 });
    await page.goto(`/match/${matchId}`);

    await expect(page.getByTestId('match-connecting')).toBeHidden({ timeout: 30_000 });
    await expect(page).toHaveURL(/\/auth\/guest/, { timeout: 30_000 });
  });
});
