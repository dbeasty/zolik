import { expect, test, type Page } from '@playwright/test';

import { BOARD_FITS_HEIGHT, dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { cardByCode, selectOnly } from '../helpers/hand';

/**
 * A drop the server refuses says why, where the player let go.
 *
 * CanastaX keeps Samba's wild limits: two wilds at most in a group. Each of
 * 2, 2 and a joker fits a group of four kings on its own, so the board lights
 * the group up for all three together — only the engine, given the three at
 * once, says "too many wilds". That answer used to land as a line of red text
 * in the action area, off-screen from the meld, while the cards snapped home
 * as though the drag had simply failed. It now opens the same sheet a drop the
 * client can refuse up front opens.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `drop-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function seededTable(request: Ctx) {
  const host = await guest(request);
  const other = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'canastax', options: { targetScore: 10000 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${other.accessToken}` },
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  const p1 = host.userId as string;
  const p2 = other.userId as string;
  const state = {
    status: 'active',
    variation: 'canastax',
    players: [p1, p2],
    turnOrder: [p1, p2],
    current: p1,
    phase: 'meld',
    teams: [
      {
        id: 0,
        players: [p1],
        score: 0,
        melds: [{ id: 't0-K', teamId: 0, kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC'] }],
        redThrees: [],
        hasMelded: true,
      },
      { id: 1, players: [p2], score: 0, melds: [], redThrees: [], hasMelded: false },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(20).fill('8S'),
    discardPile: ['9H'],
    hands: { [p1]: ['2C', '2D', 'JOKER1', '9S', 'JD', '7C'], [p2]: ['8H', '9H', 'TH'] },
    frozen: false,
    laidThisTurn: 0,
    meldsAtTurnStart: true,
    tookPileThisTurn: false,
    handSize: 15,
    targetScore: 10000,
    canastasToGoOut: 2,
    dealNumber: 0,
    dealer: 0,
    pause: true,
    openDiscard: false,
    seed: 1,
  };
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: { state },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.setViewportSize({ width: 1280, height: BOARD_FITS_HEIGHT });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
      window.localStorage.setItem('zolik_seen_intro', '1');
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'drop',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId('group-t0-K')).toBeVisible({ timeout: 15_000 });
}

async function meldCards(request: Ctx, host: any, matchId: string): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, {
    headers: { Authorization: `Bearer ${host.accessToken}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const groups = (body.view?.zones ?? []).flatMap((z: any) => z.groups ?? []);
  return groups.find((g: any) => g.id === 't0-K')?.cards ?? [];
}

test('three wilds dropped on a group open the sheet saying why', async ({ page, request }) => {
  const { matchId, host } = await seededTable(request);
  await openMatch(page, host, matchId);
  await handCards(page);

  await selectOnly(page, ['2C', '2D', 'JOKER1']);
  await dragLocatorTo(page, cardByCode(page, 'JOKER1'), page.getByTestId('group-t0-K'));

  const sheet = page.getByTestId('why-sheet');
  await expect(sheet).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId('why-reason')).toHaveText('Too many wild cards in that meld');
  // The sheet replaces the red line rather than adding to it.
  await expect(page.getByTestId('match-error')).toHaveCount(0);

  // Refused, so nothing moved.
  expect(await meldCards(request, host, matchId)).toEqual(['KH', 'KD', 'KS', 'KC']);
});

test('two wilds on the same group are taken, and nothing is explained', async ({ page, request }) => {
  const { matchId, host } = await seededTable(request);
  await openMatch(page, host, matchId);
  await handCards(page);

  await selectOnly(page, ['2C', '2D']);
  await dragLocatorTo(page, cardByCode(page, '2D'), page.getByTestId('group-t0-K'));

  await expect.poll(() => meldCards(request, host, matchId), { timeout: 10_000 }).toHaveLength(6);
  await expect(page.getByTestId('why-sheet')).toHaveCount(0);
});
