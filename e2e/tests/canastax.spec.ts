import { expect, test, type Page } from '@playwright/test';

import { handCards, tapCard } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { selectOnly } from '../helpers/hand';

/**
 * CanastaX, end to end: every house move made with the pointer, and every one
 * checked against the server's own copy of the table rather than against what
 * the screen happens to draw (docs/wild-samba-plan.md).
 *
 * The board is seeded through the dev-only debug-state hatch, two humans and
 * no bots, so nothing moves while the spec is looking:
 *
 *   ours    K K K K 2♣        5♥ 6♥ 7♥ 8♥        9♣ 9♦ 9♠
 *   theirs  Q Q Q JOKER1      5♠ JOKER2 7♠ 8♠   (the joker is the 6♠)
 *   hand    6♠ Q♣ 2♦ 2♥ 2♠ JOKER3 A♣ 10♣ 4♠
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx, name: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName: name } });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function canastaXTable(request: Ctx, mutate?: (state: any, p1: string, p2: string) => void) {
  const host = await guest(request, `cx-${Math.random().toString(36).slice(2, 8)}`);
  const other = await guest(request, `cy-${Math.random().toString(36).slice(2, 8)}`);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'canastax', options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  expect(
    (await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${other.accessToken}` },
    })).ok(),
  ).toBeTruthy();
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

  const p1 = host.userId as string;
  const p2 = other.userId as string;
  const state: any = {
    status: 'active',
    variation: 'canastax',
    players: [p1, p2],
    turnOrder: [p1, p2],
    current: p1,
    phase: 'meld',
    teams: [
      {
        id: 0, players: [p1], score: 0, hasMelded: true, meldSeq: 1,
        melds: [
          { id: 't0-K', teamId: 0, kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', '2C'] },
          { id: 't0-seq1', teamId: 0, kind: 'run', suit: 'H', cards: ['5H', '6H', '7H', '8H'] },
          { id: 't0-9', teamId: 0, kind: 'set', rank: '9', cards: ['9C', '9D', '9S'] },
        ],
      },
      {
        id: 1, players: [p2], score: 0, hasMelded: true, meldSeq: 1,
        melds: [
          { id: 't1-Q', teamId: 1, kind: 'set', rank: 'Q', cards: ['QH', 'QD', 'QS', 'JOKER1'] },
          { id: 't1-seq1', teamId: 1, kind: 'run', suit: 'S', low: 1, cards: ['5S', 'JOKER2', '7S', '8S'] },
        ],
      },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(40).fill('4C'),
    discardPile: ['4D', '8C'],
    hands: {
      [p1]: ['6S', 'QC', '2D', '2H', '2S', 'JOKER3', 'AC', 'TC', '4S'],
      [p2]: ['8H', '9H', 'TH', 'JD', 'QD'],
    },
    handSize: 15,
    targetScore: 10000,
    canastasToGoOut: 2,
    dealNumber: 1,
    dealer: 1,
    meldsAtTurnStart: true,
    pause: true,
    seed: 7,
  };
  mutate?.(state, p1, p2);
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, { headers: auth, data: { state } });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string, width = 1280) {
  await page.setViewportSize({ width, height: 1100 });
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
  await handCards(page);
}

/** The table and the viewer's hand, as the server has them. */
async function onServer(request: Ctx, matchId: string, host: any) {
  const body = await (
    await request.get(`${API_BASE}/matches/${matchId}`, { headers: { Authorization: `Bearer ${host.accessToken}` } })
  ).json();
  const melds: Record<string, string> = {};
  let hand: string[] = [];
  let pile = 0;
  for (const z of body.view?.zones ?? []) {
    for (const g of z.groups ?? []) melds[g.id] = g.cards.join(' ');
    if (z.kind === 'hand' && z.ownerId === host.userId) hand = (z.cards ?? []).map((c: any) => c.card);
    if (z.id === 'discard') pile = z.count ?? 0;
  }
  return { melds, hand, pile, offers: body.legalActions as any[] };
}

/** Presses a lit meld near its top or bottom edge — which end of a run that is. */
async function tapNear(page: Page, target: import('@playwright/test').Locator, edge: 'top' | 'bottom') {
  await target.scrollIntoViewIfNeeded();
  const box = (await target.boundingBox())!;
  await page.mouse.click(box.x + box.width / 2, edge === 'top' ? box.y + 6 : box.y + box.height - 6);
}

/** Taps a meld open, so its cards are drawn whole and can be picked one by one. */
async function openMeld(page: Page, meldId: string) {
  const toggle = page.getByTestId(`group-toggle-${meldId}`);
  await toggle.scrollIntoViewIfNeeded();
  await toggle.click();
  // Read off its label: this react-native-web drops aria-expanded on the web build.
  await expect(toggle).toHaveAttribute('aria-label', 'Collapse this group');
}

test('the lobby offers CanastaX and states its rules', async ({ page, request }) => {
  const host = await guest(request, `cl-${Math.random().toString(36).slice(2, 8)}`);
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
      window.localStorage.setItem('zolik_seen_intro', '1');
    },
    { accessToken: host.accessToken, refreshToken: host.refreshToken, userId: host.userId, username: host.guestName, isGuest: true },
  );
  await page.goto('/lobby/setup?moduleId=canasta&mode=bots');
  await page.getByText('CanastaX', { exact: true }).click();
  await expect(page.getByText(/dirty samba, worth 700/)).toBeVisible();
  await expect(page.getByText(/2250 if all 2s, 1500 with jokers/)).toBeVisible();
  for (const option of ['Dirty sequences', 'Meld of 2s', 'Rearrange melds', 'Poach wilds', 'Top card or whole pile']) {
    await expect(page.getByText(option, { exact: true })).toBeVisible();
  }
});

test('a wild is moved off one meld onto another, and the move undone', async ({ page, request }) => {
  test.setTimeout(90_000);
  const { matchId, host } = await canastaXTable(request);
  await openMatch(page, host, matchId);

  // The move is not a button: it is said on the board, beside the controls.
  await expect(page.getByTestId('action-bar').locator('[data-testid^="offer-move:"]')).toHaveCount(0);
  await expect(page.getByTestId('move-hint')).toContainText('open a meld and tap the cards');

  await openMeld(page, 't0-K');
  await tapCard(page, page.getByTestId('card-t0-K-4'));
  await expect(page.getByTestId('move-hint')).toContainText('Now tap the meld');
  // Where the 2 may go lights up, and the meld it came from does not.
  await expect(page.getByTestId('group-press-t0-seq1')).toBeVisible();
  await expect(page.getByTestId('group-press-t0-K')).toHaveCount(0);

  // A wild can join a dirty run at either end; letting go on its bottom edge
  // says "after the 8".
  await tapNear(page, page.getByTestId('group-press-t0-seq1'), 'bottom');
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-seq1']).toBe('5H 6H 7H 8H 2C');
  expect((await onServer(request, matchId, host)).melds['t0-K']).toBe('KH KD KS KC');

  const undo = page.getByTestId('offer-undo_reshape');
  await expect(undo).toHaveText(/Undo move/);
  await undo.click();
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-K']).toBe('KH KD KS KC 2C');
  expect((await onServer(request, matchId, host)).melds['t0-seq1']).toBe('5H 6H 7H 8H');
});

test('a card out of the middle of a sequence cannot be moved', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await canastaXTable(request, (s) => {
    // The 5♥ off the bottom of the run could join the fives, so the run's
    // cards can be picked up at all; the 6♥ in its middle could join the
    // sixes, but taking it out would leave a gap.
    s.teams[0].melds.push({ id: 't0-6', teamId: 0, kind: 'set', rank: '6', cards: ['6C', '6D', '6S'] });
    s.teams[0].melds.push({ id: 't0-5', teamId: 0, kind: 'set', rank: '5', cards: ['5C', '5D', '5S'] });
  });
  await openMatch(page, host, matchId);
  await openMeld(page, 't0-seq1');
  await tapCard(page, page.getByTestId('card-t0-seq1-1')); // the 6♥
  await expect(page.getByTestId('move-hint')).toContainText('Now tap the meld');
  // Nowhere to put it: the run would come apart.
  await expect(page.locator('[data-testid^="group-press-"]')).toHaveCount(0);
  // Whereas the 5♥ off the end goes onto the fives.
  await tapCard(page, page.getByTestId('card-t0-seq1-1')); // put the 6♥ back down
  await tapCard(page, page.getByTestId('card-t0-seq1-0'));
  await page.getByTestId('group-press-t0-5').click();
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-5']).toBe('5C 5D 5S 5H');
  expect((await onServer(request, matchId, host)).melds['t0-seq1']).toBe('6H 7H 8H');
});

test("the 6♠ buys the joker out of the other side's sequence", async ({ page, request }) => {
  test.setTimeout(90_000);
  const { matchId, host } = await canastaXTable(request);
  await openMatch(page, host, matchId);

  await selectOnly(page, ['6S']);
  const target = page.getByTestId('group-press-t1-seq1');
  await expect(target).toBeVisible();
  await target.click();

  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t1-seq1']).toBe('5S 6S 7S 8S');
  const after = await onServer(request, matchId, host);
  expect(after.hand).toContain('JOKER2');
  expect(after.hand).not.toContain('6S');

  await page.getByTestId('offer-undo_reshape').click();
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t1-seq1']).toBe('5S JOKER2 7S 8S');
  expect((await onServer(request, matchId, host)).hand).toContain('6S');
});

test('three 2s go down as the meld of 2s', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await canastaXTable(request);
  await openMatch(page, host, matchId);

  await page.getByTestId('action-bar').getByTestId('offer-group:verb.lay_meld').click();
  const choice = page.getByTestId('offer-choice-lay_meld:wild');
  await expect(choice).toContainText('Meld of 2s');
  await choice.click();
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-wild']).toBe('2D 2H 2S');
});

test('a wild let go on the top of a sequence goes below it', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await canastaXTable(request);
  await openMatch(page, host, matchId);

  await selectOnly(page, ['JOKER3']);
  const target = page.getByTestId('group-press-t0-seq1');
  await expect(target).toBeVisible();
  // The run is drawn low card first, top to bottom: its top edge is "front".
  await tapNear(page, target, 'top');
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-seq1']).toBe('JOKER3 5H 6H 7H 8H');
});

test('the top card alone is taken, and the rest of the pile stays', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await canastaXTable(request, (s, p1) => {
    s.phase = 'draw';
    s.discardPile = ['4D', '9C', 'TS', 'QH'];
    s.hands[p1] = ['QD', 'QS', '5C', '7D', 'AC'];
  });
  await openMatch(page, host, matchId);

  const top = page.getByTestId('action-bar').getByText('Take only the top card', { exact: true });
  await expect(top).toBeVisible();
  await top.click();
  await expect.poll(async () => (await onServer(request, matchId, host)).pile).toBe(3);
  const after = await onServer(request, matchId, host);
  expect(after.hand.sort()).toEqual(['5C', '7D', 'AC']);
  expect(Object.values(after.melds)).toContain('QH QD QS');
});

test('moving cards between melds works at phone width', async ({ page, request }) => {
  test.setTimeout(90_000);
  const { matchId, host } = await canastaXTable(request);
  await openMatch(page, host, matchId, 390);

  await openMeld(page, 't0-K');
  await tapCard(page, page.getByTestId('card-t0-K-4'));
  const target = page.getByTestId('group-press-t0-9');
  await target.scrollIntoViewIfNeeded();
  await target.click();
  await expect.poll(async () => (await onServer(request, matchId, host)).melds['t0-9']).toBe('9C 9D 9S 2C');
});
