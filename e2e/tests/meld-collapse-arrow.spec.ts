import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Tapping a meld spreads its cards open and tapping it again folds them back,
 * but nothing on a meld said so. A closed meld now draws a ▼ above its cards
 * (`group-expand-<id>`) and an open one a ▲ (`group-collapse-<id>`).
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

async function tapArrow(page: Page, id: string, testId: string) {
  const arrow = page.getByTestId(testId);
  await expect(arrow).toBeVisible();
  await arrow.scrollIntoViewIfNeeded();
  const box = (await arrow.boundingBox())!;
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  // The arrow is inside the toggle, so a press on it is the toggle's press.
  const hit = await page.evaluate(
    ({ x, y, id }) => !!document.elementFromPoint(x, y)?.closest(`[data-testid="group-toggle-${id}"]`),
    { x, y, id },
  );
  expect(hit).toBe(true);
  if (SHOTS) {
    await page.getByTestId(`group-${id}`).screenshot({ path: `${SHOTS}/${testId}-${page.viewportSize()!.width}.png` });
  }
  await page.mouse.click(x, y);
}

for (const width of [1280, 375]) {
  test(`a meld shows an expand arrow closed and a collapse arrow open (${width}px)`, async ({ page, request }) => {
    const { matchId, host } = await seededTable(request);
    await openMatch(page, host, matchId, width);

    // t0-9 is a plain meld, t0-5 a finished canasta that folds to one card.
    for (const id of ['t0-9', 't0-5']) {
      const toggle = page.getByTestId(`group-toggle-${id}`);
      await expect(toggle).toBeVisible({ timeout: 15_000 });
      await expect(toggle).toHaveAttribute('aria-expanded', 'false');
      await expect(page.getByTestId(`group-collapse-${id}`)).toHaveCount(0);

      await tapArrow(page, id, `group-expand-${id}`);
      await expect(toggle).toHaveAttribute('aria-expanded', 'true');
      await expect(page.getByTestId(`group-expand-${id}`)).toHaveCount(0);

      await tapArrow(page, id, `group-collapse-${id}`);
      await expect(toggle).toHaveAttribute('aria-expanded', 'false');
      await expect(page.getByTestId(`group-collapse-${id}`)).toHaveCount(0);
      await expect(page.getByTestId(`group-expand-${id}`)).toBeVisible();
    }
  });
}
