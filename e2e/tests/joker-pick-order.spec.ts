import { expect, test, type Page } from '@playwright/test';

import { API_BASE, asViewer, type Viewer } from '../helpers/env';
import { handCards } from '../helpers/drag';
import { selectOnly } from '../helpers/hand';

/**
 * Where a joker goes in a run is the player's to say, by the order they pick
 * the cards. 5♠ 6♠ 7♠ and a joker make 4-7 or 5-8 alike; picked after the
 * seven the joker is the eight, picked before the five it is the four.
 *
 * Before, the client sent the cards in the order they sat in the fan and the
 * server put the joker wherever it was worth more — always the eight here, so
 * there was no way to lay the four at all. The server's copy of the meld is
 * the witness: the cards are stored in their laid order.
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
    data: { guestName: `jokerorder-${Math.random().toString(36).slice(2, 10)}` },
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

/** The viewer on turn in the meld phase, holding `hand` in that fan order. */
async function seedHand(request: Ctx, matchId: string, host: any, auth: Record<string, string>, hand: string[]) {
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
      Hands: { [me]: hand, [bot]: ['2D', '3S', '4D', '9D'] },
      Melds: {},
      MeldMeta: {},
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
      username: 'jokerorder',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The viewer's melds, as the server has them. */
async function myMeldsOnServer(request: Ctx, matchId: string, viewer: Viewer & { userId: string }) {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(viewer))).json();
  const out: string[] = [];
  for (const z of b.view?.zones ?? []) {
    if (z.ownerId !== viewer.userId || z.kind === 'hand') continue;
    for (const g of z.groups ?? []) out.push(g.cards.join(','));
  }
  return out.join(' | ') || '(none)';
}

test.describe('the order cards are picked in places a run’s joker', () => {
  for (const tc of [
    {
      name: 'picked last, the joker is the eight — though it sits first in the fan',
      hand: ['JOKER1', '5S', '6S', '7S', 'KD', 'KS'],
      pick: ['5S', '6S', '7S', 'JOKER1'],
      want: '5S,6S,7S,JOKER1',
    },
    {
      name: 'picked first, the joker is the four — though the eight is worth more',
      hand: ['5S', '6S', '7S', 'JOKER1', 'KD', 'KS'],
      pick: ['JOKER1', '5S', '6S', '7S'],
      want: 'JOKER1,5S,6S,7S',
    },
  ]) {
    test(tc.name, async ({ page, request }) => {
      const { matchId, host, auth } = await tableWithBot(request);
      await seedHand(request, matchId, host, auth, tc.hand);
      await openMatch(page, host, matchId);
      await handCards(page);

      await selectOnly(page, tc.pick);
      const meld = page.getByTestId('offer-lay_meld');
      await expect(meld).toBeEnabled();
      await meld.click();

      await expect
        .poll(() => myMeldsOnServer(request, matchId, host), { timeout: 10_000 })
        .toBe(tc.want);
    });
  }
});
