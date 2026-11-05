import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Tapping a meld spreads its cards open and tapping it again folds them back,
 * but nothing on an open meld said the second tap existed. An open meld now
 * draws a ▲ above its cards (`group-collapse-<id>`); a closed one draws none.
 *
 * Seeded through the debug-state hatch, as `canasta-folded-face` does, so the
 * melds are on the table on the first frame.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `arrow-${Math.random().toString(36).slice(2, 10)}` },
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
          { id: 't0-9', teamId: 0, rank: '9', cards: ['9S', '9C', '9D', '2H'] },
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
      username: 'arrow',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

const SHOTS = process.env.ZOLIK_E2E_SHOTS;

for (const width of [1280, 375]) {
  test(`an open meld shows a collapse arrow, and tapping folds it (${width}px)`, async ({ page, request }) => {
    const { matchId, host } = await seededTable(request);
    await openMatch(page, host, matchId, width);

    for (const id of ['t0-9', 't0-5']) {
      const toggle = page.getByTestId(`group-toggle-${id}`);
      const arrow = page.getByTestId(`group-collapse-${id}`);
      await expect(toggle).toBeVisible({ timeout: 15_000 });
      await expect(arrow).toHaveCount(0);

      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-expanded', 'true');
      await expect(arrow).toBeVisible();
      // Inside the toggle, so the arrow is the toggle's own hit area.
      const box = (await arrow.boundingBox())!;
      const hit = await page.evaluate(
        ({ x, y, id }) => !!document.elementFromPoint(x, y)?.closest(`[data-testid="group-toggle-${id}"]`),
        { x: box.x + box.width / 2, y: box.y + box.height / 2, id },
      );
      expect(hit).toBe(true);
      if (SHOTS) await page.getByTestId(`group-${id}`).screenshot({ path: `${SHOTS}/open-${id}-${width}.png` });

      await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
      await expect(toggle).toHaveAttribute('aria-expanded', 'false');
      await expect(arrow).toHaveCount(0);
    }
  });
}
