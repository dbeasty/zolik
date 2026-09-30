import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * A folded canasta shows a natural, not a wild.
 *
 * A finished canasta folds down to one card and a count (`Group.complete`).
 * That card used to be the last one laid, so a mixed canasta finished with a
 * two or a joker folded to the wild, and the one card left in view said
 * nothing about what the meld was made of. The module now names the card to
 * keep (`Group.face`), and for a canasta that is the last natural.
 *
 * Seeded through the debug-state hatch, as `canasta-black-three-going-out`
 * does, so the canasta is there on the first frame rather than by luck.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `fold-${Math.random().toString(36).slice(2, 10)}` },
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
    data: { moduleId: 'canasta', variation: 'classic', options: { targetScore: 10000 } },
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
    variation: 'classic',
    players: [p1, p2],
    turnOrder: [p1, p2],
    current: p1,
    phase: 'draw',
    teams: [
      {
        id: 0,
        players: [p1],
        score: 0,
        // Mixed canasta finished with a wild: the last card is a joker.
        melds: [
          {
            id: 't0-5',
            teamId: 0,
            rank: '5',
            cards: ['5H', '5D', '5S', '5C', '5H', '2C', 'JOKER1'],
          },
        ],
        redThrees: [],
        hasMelded: true,
      },
      { id: 1, players: [p2], score: 0, melds: [], redThrees: [], hasMelded: false },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(20).fill('8S'),
    discardPile: ['9H'],
    hands: { [p1]: ['KH', 'KD', 'QS', '7C'], [p2]: ['8H', '9H', 'TH'] },
    frozen: false,
    laidThisTurn: 0,
    meldsAtTurnStart: true,
    tookPileThisTurn: false,
    handSize: 11,
    targetScore: 10000,
    canastasToGoOut: 1,
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

async function openMatch(page: Page, host: any, matchId: string, width: number) {
  await page.setViewportSize({ width, height: 1200 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
      window.localStorage.setItem('zolik_seen_intro', '1');
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'fold',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The card codes a group is drawing right now, from their spoken labels. */
async function drawnCards(page: Page, groupId: string): Promise<string[]> {
  const toggle = page.getByTestId(`group-toggle-${groupId}`);
  await expect(toggle).toBeVisible();
  return toggle.locator('[aria-label]').evaluateAll((els) =>
    els.map((e) => e.getAttribute('aria-label') ?? '').filter((l) => /^[0-9TJQKA]+[HDSC]$|^JOKER/.test(l)),
  );
}

for (const width of [1280, 375]) {
  test(`a folded mixed canasta shows its natural, not the wild laid last (${width}px)`, async ({
    page,
    request,
  }) => {
    const { matchId, host } = await seededTable(request);
    await openMatch(page, host, matchId, width);

    // Folded: one card in view, and it is a five.
    await expect(page.getByTestId('group-folded-t0-5')).toBeVisible({ timeout: 15_000 });
    await expect.poll(() => drawnCards(page, 't0-5')).toEqual(['5H']);

    // Opened: every card, in the order it was laid, wilds included.
    await page.getByTestId('group-toggle-t0-5').click();
    await expect(page.getByTestId('group-folded-t0-5')).toHaveCount(0);
    await expect
      .poll(() => drawnCards(page, 't0-5'))
      .toEqual(['5H', '5D', '5S', '5C', '5H', '2C', 'JOKER1']);
  });
}
