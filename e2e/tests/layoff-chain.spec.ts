import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { cardByCode, selectOnly } from '../helpers/hand';

/**
 * Two cards that only extend a run *together* go onto it in one gesture.
 *
 * Reported from a real game: a run of 7-8-9-10 on the table, the 5 and the 6
 * in hand, and dropping the pair on it refused — the player had to lay the 6
 * first and then the 5, one card per gesture, for a move the engine takes
 * whole.
 *
 * The engine was never the problem. `ValidateLayOff` appends every submitted
 * card and revalidates the meld, so the pair was always legal; it was the
 * offer that was narrower than the validator, listing only the cards that
 * extend the run *on their own*. The client trusts that list, as it is meant
 * to, and refused the move on the server's behalf.
 *
 * So this pins the gesture end to end, and it checks the *server's* copy of
 * the meld: a client that only moved two cards in its own head would satisfy
 * every visual assertion here.
 *
 * The position is seeded through the dev-only debug-state hatch rather than
 * played to, for the reason clean-run.spec.ts gives — waiting for the right
 * cards to fall is a test that mostly does not run.
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
  await page.setViewportSize({ width: 1280, height: 1200 });
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
async function meldOnServer(request: Ctx, matchId: string, userId: string) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
  for (const z of b.view?.zones ?? []) {
    for (const g of z.groups ?? []) if (g.id === 'meld_1') return g.cards.join(',');
  }
  return '(gone)';
}

test.describe('a lay-off whose cards need each other', () => {
  test('the 5 and the 6 go onto a run of 7-8-9-10 in one drag', async ({ page, request }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRunAndGapCards(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    // Both cards picked, then one gesture. Selecting explicitly rather than
    // trusting what is already picked: a card that arrives in hand lands
    // selected, so "drag the 5" otherwise means "drag the 5 and whatever
    // else happened to be on".
    await selectOnly(page, ['5C', '6C']);
    await dragLocatorTo(page, cardByCode(page, '6C'), page.getByTestId('group-meld_1'));

    // The server is the witness.
    await expect
      .poll(() => meldOnServer(request, matchId, host.userId), { timeout: 10_000 })
      .toBe('5C,6C,7C,8C,9C,TC');
  });

  test('the 5 on its own is refused, and the board does not move', async ({ page, request }) => {
    const { matchId, host, auth } = await tableWithBot(request);
    await seedRunAndGapCards(request, matchId, host, auth);
    await openMatch(page, host, matchId);
    await handCards(page);

    // The other half of the same fact. The 5 alone leaves a gap at the 6, and
    // the offer says so per card — so this must stay refused even though the
    // pair is now legal, or the fix would have simply widened the list into a
    // lie.
    await selectOnly(page, ['5C']);
    await dragLocatorTo(page, cardByCode(page, '5C'), page.getByTestId('group-meld_1'));

    await expect
      .poll(() => meldOnServer(request, matchId, host.userId), { timeout: 5_000 })
      .toBe('7C,8C,9C,TC');
  });
});
