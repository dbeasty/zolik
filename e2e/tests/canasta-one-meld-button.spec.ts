import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { selectOnly } from '../helpers/hand';
import { API_BASE } from '../helpers/env';

/**
 * Two things a Canasta table used to get wrong about its own width.
 *
 * One Meld control. The module offers a new meld per meldable rank, and the
 * bar drew each as its own button — "Meld · K", "Meld · Q", side by side,
 * reading as different moves. They are one move with several possible hands,
 * so they fold into one control: the selection says which, and a press that
 * the selection does not settle lists the candidates under it.
 *
 * Six seats on a wide screen. The strip scrolled sideways with no bar and no
 * arrow, so an 800-pixel window showed four of six players and gave no sign
 * the other two existed.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx, name: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName: name } });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function openMatch(page: Page, host: any, matchId: string, width: number) {
  await page.setViewportSize({ width, height: 1000 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
      window.localStorage.setItem('zolik_seen_intro', '1');
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: host.guestName,
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** Host on turn, already opened, holding three kings and three queens. */
async function kingsAndQueens(request: Ctx) {
  const host = await guest(request, `meld-${Math.random().toString(36).slice(2, 8)}`);
  const other = await guest(request, `other-${Math.random().toString(36).slice(2, 8)}`);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'classic', options: { targetScore: 10000 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  expect((await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${other.accessToken}` },
  })).ok()).toBeTruthy();
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

  const p1 = host.userId as string;
  const p2 = other.userId as string;
  const state = {
    status: 'active',
    variation: 'classic',
    players: [p1, p2],
    turnOrder: [p1, p2],
    current: p1,
    phase: 'meld',
    teams: [
      {
        id: 0,
        players: [p1],
        score: 0,
        melds: [{ id: 't0-5', teamId: 0, rank: '5', cards: ['5H', '5D', '5S'] }],
        redThrees: [],
        hasMelded: true,
      },
      { id: 1, players: [p2], score: 0, melds: [], redThrees: [], hasMelded: false },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(20).fill('8S'),
    discardPile: ['9H'],
    hands: { [p1]: ['KH', 'KD', 'KS', 'QH', 'QD', 'QS', '7C', '9D', '4S'], [p2]: ['8H', '9H', 'TH'] },
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

async function serverHand(request: Ctx, matchId: string, host: any): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, {
    headers: { Authorization: `Bearer ${host.accessToken}` },
  });
  const body = await res.json();
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === host.userId);
  return (zone?.cards ?? []).map((c: any) => c.card);
}

test('several possible melds are one Meld control that asks which', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await kingsAndQueens(request);
  await openMatch(page, host, matchId, 1280);
  await handCards(page);

  const bar = page.getByTestId('action-bar');
  const meld = bar.getByTestId('offer-group:verb.lay_meld');
  await expect(meld).toBeVisible();
  await expect(meld).toHaveText('Meld');
  await expect(bar.locator('[data-testid^="offer-lay_meld"]'), 'no per-rank meld buttons').toHaveCount(0);

  // Nothing picked: the press lists the candidates rather than guessing one.
  await meld.click();
  await expect(bar.getByTestId('offer-choice-lay_meld:K')).toBeVisible();
  await expect(bar.getByTestId('offer-choice-lay_meld:Q')).toBeVisible();
  expect(await serverHand(request, matchId, host)).toHaveLength(9);

  // Cards that make no meld grey it out and say so.
  await selectOnly(page, ['7C', '9D']);
  await expect(meld).toHaveAttribute('aria-disabled', 'true');
  await expect(bar.getByTestId('why-group:verb.lay_meld')).toContainText("Those cards can't go here");
  await expect(bar.locator('[data-testid^="offer-choice-"]')).toHaveCount(0);

  // The queens settle it, and the press lays exactly those.
  await selectOnly(page, ['QH', 'QD', 'QS']);
  await expect(meld).not.toHaveAttribute('aria-disabled', 'true');
  await meld.click();
  await expect.poll(async () => (await serverHand(request, matchId, host)).filter((c) => c.startsWith('Q'))).toEqual([]);
  expect((await serverHand(request, matchId, host)).filter((c) => c.startsWith('K'))).toHaveLength(3);

  // One candidate left: an ordinary one-tap button again.
  const kings = bar.getByTestId('offer-lay_meld:K');
  await expect(kings).toBeVisible();
  await kings.click();
  await expect.poll(async () => (await serverHand(request, matchId, host)).filter((c) => c.startsWith('K'))).toEqual([]);
});

test('six seats all show on an 800-pixel screen', async ({ page, request }) => {
  test.setTimeout(60_000);
  const users = [];
  for (let i = 0; i < 6; i++) users.push(await guest(request, `Seat-${i}-${Math.random().toString(36).slice(2, 6)}`));
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'samba', options: { targetScore: 10000 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (const u of users.slice(1)) {
    const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${u.accessToken}` },
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();
  }
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

  await openMatch(page, host, matchId, 800);
  const strip = page.getByTestId('seat-strip');
  await expect(strip).toBeVisible();
  const box = await strip.boundingBox();
  expect(box).not.toBeNull();
  for (const u of users) {
    const tile = await page.getByTestId(`seat-${u.userId}`).boundingBox();
    expect(tile, `seat ${u.guestName} should be drawn`).not.toBeNull();
    expect(tile!.x, `seat ${u.guestName} starts inside the strip`).toBeGreaterThanOrEqual(box!.x - 1);
    expect(tile!.x + tile!.width, `seat ${u.guestName} ends inside the strip`).toBeLessThanOrEqual(box!.x + box!.width + 1);
  }
  // Everything fits, so there is nothing to scroll to.
  await expect(page.getByTestId('seat-strip-right')).toHaveCount(0);
});
