import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, BOARD_FITS_HEIGHT } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * CanastaX lets a side rearrange its own melds, which includes joining two of
 * them: a second group of kings onto the first, or a run of 9-10-J onto 5-8.
 * Neither can be done a card at a time — a group left with two kings is no
 * meld — so the move was never offered and the cards could not be picked up.
 * Dragging one king now carries the other two with it.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBot(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `combine-${Math.random().toString(36).slice(2, 10)}` },
  });
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'canastax' },
  });
  const { matchId } = await created.json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  const live = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const bot = live.players.find((p: any) => p.id !== host.userId).id as string;
  return { matchId, host, auth, me: host.userId as string, bot };
}

/** Two groups of kings and two heart runs, the viewer to meld. */
async function seedTwoKingGroups(request: Ctx, matchId: string, auth: Record<string, string>, me: string, bot: string) {
  const meld = (id: string, kind: string, cards: string[], extra: Record<string, string>) => ({
    id, teamId: id.startsWith('t0') ? 0 : 1, kind, rank: '', ...extra, cards,
  });
  const state = {
    status: 'active',
    variation: 'canastax',
    players: [me, bot],
    turnOrder: [me, bot],
    current: me,
    phase: 'meld',
    meldsAtTurnStart: true,
    handSize: 15,
    targetScore: 10000,
    canastasToGoOut: 2,
    teams: [
      {
        id: 0, players: [me], score: 0, hasMelded: true,
        melds: [
          meld('t0-K', 'set', ['KH', 'KD', 'KS', 'KC'], { rank: 'K' }),
          meld('t0-K-2', 'set', ['KH', 'KD', 'KS'], { rank: 'K' }),
          meld('t0-seq1', 'run', ['5H', '6H', '7H', '8H'], { suit: 'H' }),
          meld('t0-seq2', 'run', ['9H', 'TH', 'JH'], { suit: 'H' }),
        ],
      },
      { id: 1, players: [bot], score: 0, hasMelded: true, melds: [meld('t1-9', 'set', ['9C', '9D', '9S'], { rank: '9' })] },
    ],
    teamOf: { [me]: 0, [bot]: 1 },
    drawPile: Array.from({ length: 30 }, () => '8S'),
    discardPile: ['3C'],
    hands: { [me]: ['4S', 'QS', 'JD', '6C', '7D'], [bot]: ['8C', '8D', 'TC', 'QD'] },
  };
  // No `rules`: the module resolves them from the variation.
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, { headers: auth, data: { state } });
  expect(res.ok(), await res.text()).toBeTruthy();
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.setViewportSize({ width: 1280, height: BOARD_FITS_HEIGHT });
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, { accessToken: host.accessToken, refreshToken: host.refreshToken, userId: host.userId, username: 'combine', isGuest: true });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

test.describe('CanastaX combines two melds', () => {
  test('dragging one king of a second group joins all three to the first', async ({ page, request }) => {
    const { matchId, host, auth, me, bot } = await tableWithBot(request);
    await seedTwoKingGroups(request, matchId, auth, me, bot);
    await openMatch(page, host, matchId);

    await page.getByTestId('group-toggle-t0-K-2').click();
    const king = page.getByTestId('pick-t0-K-2-0');
    await expect(king).toBeVisible();
    await dragLocatorTo(page, king, page.getByTestId('group-t0-K'));

    // What the player sees: the second group gone, seven kings in the first.
    await expect(page.getByTestId('group-t0-K-2')).toHaveCount(0);
    await page.getByTestId('group-toggle-t0-K').click();
    await expect(page.locator('[data-testid^="card-t0-K-"]')).toHaveCount(7);
  });

  test('dragging the end of one run joins it whole to the other', async ({ page, request }) => {
    const { matchId, host, auth, me, bot } = await tableWithBot(request);
    await seedTwoKingGroups(request, matchId, auth, me, bot);
    await openMatch(page, host, matchId);

    await page.getByTestId('group-toggle-t0-seq2').click();
    const jack = page.getByTestId('pick-t0-seq2-2');
    await expect(jack).toBeVisible();
    await dragLocatorTo(page, jack, page.getByTestId('group-t0-seq1'));

    await expect(page.getByTestId('group-t0-seq2')).toHaveCount(0);
    await page.getByTestId('group-toggle-t0-seq1').click();
    await expect(page.locator('[data-testid^="card-t0-seq1-"]')).toHaveCount(7);
  });
});
