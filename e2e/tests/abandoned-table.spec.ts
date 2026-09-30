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

/** Signs somebody in, without a table. */
async function guest(request: Ctx, name: string) {
  return (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `${name}-${Math.random().toString(36).slice(2, 8)}` },
    })
  ).json();
}

/** A started table with two people at it — the case that had no way back. */
async function tableWithAFriend(request: Ctx) {
  const host = await guest(request, 'aband-host');
  const friend = await guest(request, 'aband-friend');
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const { matchId, joinCode } = await (
    await request.post(`${API_BASE}/matches`, {
      headers: auth,
      data: { moduleId: 'zolik', variation: 'zolik_classic' },
    })
  ).json();
  const joined = await request.post(`${API_BASE}/matches/${joinCode}/join`, {
    headers: { Authorization: `Bearer ${friend.accessToken}` },
    data: {},
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  // The name the board will print, read off the table rather than guessed
  // from the sign-in response — they are the same today, and a spec that
  // asserts on a screen should take its expectation from the same place the
  // screen does.
  const seated = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const friendName = seated.players.find((p: { id: string }) => p.id === friend.userId).name;
  return { matchId, host, friend, friendName, auth };
}

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

/**
 * Seeds a mid-deal position and marks the table the way the reaper does.
 *
 * `other` is the opposite seat, bot or human: the position is the same either
 * way, and which it is is exactly what the resume rule used to turn on.
 */
async function seedAbandoned(request: Ctx, matchId: string, me: string, other: string, auth: Record<string, string>) {
  const state = {
    rules: {
      Rules: CLASSIC,
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

  // The bug this closes, reported from production and reproduced against it.
  //
  // Two friends were playing. One of them dropped off for two and a half
  // minutes — a tunnel, a locked phone, a closed lid — and the sweeper
  // resolved the table. From then on the game could not be brought back by
  // anybody: not by the player who had dropped, and not by the one who had
  // never left. The position was entirely intact, both of them were looking
  // at it, and the banner told them so — "the cards are exactly where you
  // left them" — above a single button reading "Back to games".
  //
  // What decides it now is presence rather than whether the other seats are
  // bots: everybody the game concerns is here, so there is nobody left for a
  // resume to surprise. Both browsers are real here for exactly that reason —
  // the rule is about two people being at the table at the same time, and one
  // context cannot be two people.
  test('two players who are both back can pick their game up again', async ({
    browser,
    request,
  }) => {
    const { matchId, host, friend, friendName, auth } = await tableWithAFriend(request);
    await seedAbandoned(request, matchId, host.userId, friend.userId, auth);

    const hostCtx = await browser.newContext();
    const friendCtx = await browser.newContext();
    try {
      const hostPage = await hostCtx.newPage();
      const friendPage = await friendCtx.newPage();

      // Only the host is looking at it to begin with. Their friend is still
      // away, so there is nothing to press — and, crucially, the screen says
      // who it is waiting for rather than going quiet.
      await openMatch(hostPage, host, matchId);
      await expect(hostPage.getByTestId('match-over')).toBeVisible({ timeout: 30_000 });
      await expect(hostPage.getByTestId('match-over-resume')).toHaveCount(0);
      await expect(hostPage.getByTestId('match-over-waiting')).toContainText(friendName, {
        timeout: 15_000,
      });

      // The friend opens the same link. Nobody presses anything yet: the
      // offer has to arrive on its own, because the thing that changed is
      // who is in the room and not what either of them did.
      await openMatch(friendPage, friend, matchId);
      await expect(friendPage.getByTestId('match-over')).toBeVisible({ timeout: 30_000 });
      await expect(hostPage.getByTestId('match-over-resume')).toBeVisible({ timeout: 30_000 });
      await expect(hostPage.getByTestId('match-over-waiting')).toHaveCount(0);

      // Either of them may press it; the one who stayed has as much claim on
      // the game as the one who dropped.
      await friendPage.getByTestId('match-over-resume').click();

      // Both boards come back, and the friend's does so without a navigation
      // — the socket is already in the table's room.
      await expect(friendPage.getByTestId('match-over')).toBeHidden({ timeout: 30_000 });
      await expect(friendPage.getByTestId('match-status')).toHaveText('active');
      await expect(hostPage.getByTestId('match-status')).toHaveText('active', { timeout: 30_000 });

      // And the server agrees, which is what makes either board real.
      const after = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
      expect(after.status).toBe('active');
    } finally {
      await hostCtx.close();
      await friendCtx.close();
    }
  });

  // The other half of the same rule, and the one it was originally written
  // for: a player alone at a table their opponent has long since left may not
  // decide on their own that the game is live again.
  test('one player alone cannot revive a game the other has left', async ({ browser, request }) => {
    const { matchId, host, friend, friendName, auth } = await tableWithAFriend(request);
    await seedAbandoned(request, matchId, host.userId, friend.userId, auth);

    const ctx = await browser.newContext();
    try {
      const page = await ctx.newPage();
      await openMatch(page, host, matchId);
      await expect(page.getByTestId('match-over')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId('match-over-resume')).toHaveCount(0);
      // Not a dead end: it says who, so the player knows a link is what is
      // missing rather than that the game is gone.
      await expect(page.getByTestId('match-over-waiting')).toContainText(friendName);
      // Nothing was quietly revived by the looking.
      const after = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
      expect(after.status).toBe('abandoned');
    } finally {
      await ctx.close();
    }
  });

  // A table that is gone rather than merely finished — which retention makes
  // the normal end state of every old link somebody saved. The socket used to
  // be sent nothing at all in this case, leaving the screen on "Waiting for
  // the table…" for ever: connected, so not even a spinner, and no way out.
  test('a link to a table that no longer exists says so, and offers a way out', async ({
    page,
    request,
  }) => {
    const { host } = await tableWithBot(request);
    // A well-formed id that was never a match — the shape a link to a
    // long-since-retired table has.
    const gone = '6aa0e923f50dcd02734b9099';

    await openMatch(page, host, gone);

    await expect(page.getByTestId('match-gone')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId('match-gone')).toHaveText('That table no longer exists');
    // Never the spinner: the two are mutually exclusive, and showing both
    // would be the old hang with an error printed above it.
    await expect(page.getByTestId('match-connecting')).toBeHidden();

    await page.getByTestId('match-gone-leave').click();
    await expect(page).toHaveURL(/\/lobby\/games/, { timeout: 30_000 });
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
