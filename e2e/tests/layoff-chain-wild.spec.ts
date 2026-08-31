import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { cardByCode, selectOnly } from '../helpers/hand';

/**
 * A card that only extends a run *together with another* must pick the
 * cheapest partner, not whichever one happens to be dealt first.
 *
 * Reported from a real game: a run of 2-9 on the table, and a hand holding
 * the ten and the jack that reach it only as a pair — plus a joker, dealt
 * *before* either of them, that could also bridge the same gap. The chain
 * that computes what a card "needs" walks the hand in dealt order and grabs
 * whichever bridging card it meets first, so the joker got claimed before the
 * ten was ever tried, and the jack was offered as needing the joker's
 * company. Dragging the natural pair — ten and jack, no joker spent — was
 * refused with "that card needs the ones next to it", and the only way
 * forward the UI offered was giving up a wild card the move never needed.
 *
 * `ValidateLayOff` was never the problem — it appends every submitted card
 * and revalidates the meld, so ten-and-jack was always legal. The offer that
 * lists what a card "needs" was reporting a true fact about the chain it
 * happened to build, not the cheapest one, which is what a player watching
 * their own hand expects "needs" to mean. See
 * `TestLayOffPlacements_PrefersTheNaturalCardOverAWildBridge` in
 * `server/internal/rules/layoff_chain_test.go` for the same fact pinned at
 * the offer layer; this pins the gesture end to end and checks the *server's*
 * copy of the meld, so a client that only moved the cards in its own head
 * would fail here.
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

async function tableWithBot(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `wildchain-${Math.random().toString(36).slice(2, 10)}` },
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
 * The reported board: the bot holding a run of 2-9 in clubs, and the viewer
 * down, on turn, holding the ten and the jack that reach it only as a pair —
 * a joker dealt *before* them, so a chain that walks the hand in order meets
 * it first — plus spare cards, because a lay-off may not empty a hand on a
 * deal that is not the last.
 */
async function seedRunAndWildBeforeGapCards(
  request: Ctx,
  matchId: string,
  host: any,
  auth: Record<string, string>,
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
      Hands: { [me]: ['JOKER1', 'TC', 'JC', 'KD', 'KS'], [bot]: ['2D', '3S', '4D', '9D'] },
      Melds: { [bot]: [['2C', '3C', '4C', '5C', '6C', '7C', '8C', '9C']] },
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
  await page.setViewportSize({ width: 1280, height: 1200 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'wildchain',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The bot's run, as the server has it. */
async function meldOnServer(request: Ctx, matchId: string, userId: string) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  for (const z of b.view?.zones ?? []) {
    for (const g of z.groups ?? []) if (g.id === 'meld_1') return g.cards.join(',');
  }
  return '(gone)';
}

/** The viewer's own hand, as the server has it. */
async function handOnServer(request: Ctx, matchId: string, userId: string) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  for (const z of b.view?.zones ?? []) {
    if (z.ownerId === userId && z.kind === 'hand') {
      return (z.cards ?? []).map((c: any) => c.card).join(',');
    }
  }
  return '(none)';
}

test.describe('a lay-off whose natural bridge is shadowed by a joker', () => {
  test('the ten and the jack go onto a run of 2-9 in one drag, joker untouched', async ({
    page,
    request,
  }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRunAndWildBeforeGapCards(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    // Both cards picked, then one gesture — the pair the engine has always
    // accepted, with no joker in the selection at all.
    await selectOnly(page, ['TC', 'JC']);
    await dragLocatorTo(page, cardByCode(page, 'JC'), page.getByTestId('group-meld_1'));

    // The server is the witness: the run grew by exactly the two natural
    // cards, in order.
    await expect
      .poll(() => meldOnServer(request, matchId, host.userId), { timeout: 10_000 })
      .toBe('2C,3C,4C,5C,6C,7C,8C,9C,TC,JC');

    // And the joker that used to get spent bridging this gap is still in
    // hand — the move never needed it.
    await expect
      .poll(() => handOnServer(request, matchId, host.userId), { timeout: 5_000 })
      .toContain('JOKER1');
  });

  test('the jack on its own is refused, and the board does not move', async ({
    page,
    request,
  }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRunAndWildBeforeGapCards(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    // The other half of the same fact: the jack alone leaves a gap at the
    // ten, so this must stay refused even though the pair is legal — and,
    // crucially, must not be silently "fixed" by reaching for the joker.
    await selectOnly(page, ['JC']);
    await dragLocatorTo(page, cardByCode(page, 'JC'), page.getByTestId('group-meld_1'));

    await expect
      .poll(() => meldOnServer(request, matchId, host.userId), { timeout: 5_000 })
      .toBe('2C,3C,4C,5C,6C,7C,8C,9C');
  });
});
