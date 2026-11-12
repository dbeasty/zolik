import { expect, test, type Locator, type Page } from '@playwright/test';

import { grabPoint, handCards, release } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { cardByCode } from '../helpers/hand';

/**
 * A carried card stays under the pointer while the board scrolls under it.
 *
 * Reported: with enough melds on the table that the board runs past the
 * window, carry a card toward the edge, the board scrolls itself (see
 * drag-edge-autoscroll.spec.ts) — and the card slides off the pointer, by
 * exactly as far as the board has scrolled. The drop is judged at the pointer,
 * so the card the player is looking at is not where it will land.
 *
 * Two carries, because they are drawn two different ways: a card out of the
 * hand is pinned to the hand's row, and a card lifted off a meld moves by how
 * far the pointer has travelled. Both were anchored to content that the
 * scroll moved and neither was told it had moved.
 */

const WIDTH = 1280;
const HEIGHT = 700;

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

async function seedTallBoard(request: Ctx) {
  const host = await (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `grip-${Math.random().toString(36).slice(2, 10)}` },
    })
  ).json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const { matchId } = await (
    await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'zolik', options: {} } })
  ).json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });

  const live = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const me = host.userId as string;
  const bot = live.players.find((p: any) => p.id !== me).id as string;
  // Enough melds that the board is several windows tall: there is room to
  // scroll whichever way the carry goes.
  const botMelds = [
    ['3D', '4D', '5D', '6D'],
    ['7H', '8H', '9H', 'TH'],
    ['2S', '3S', '4S', '5S'],
    ['JC', 'QC', 'KC', 'AC'],
    ['8D', '9D', 'TD', 'JD'],
    ['6S', '7S', '8S', '9S'],
    ['2C', '3C', '4C', '5C'],
  ];
  const myMelds = [['QH', 'KH', 'AH', '2H']];
  const state = {
    rules: {
      Rules: CONTINENTAL,
      Status: 'active',
      Phase: 'meld',
      GameNumber: 1,
      Round: 1,
      CurrentTurn: me,
      TurnOrder: [me, bot],
      Hands: { [me]: ['7C', '8C', 'KD', 'KS', '4H'], [bot]: ['2D', '3H', '4D', '9H'] },
      Melds: { [bot]: botMelds, [me]: myMelds },
      MeldMeta: {
        [bot]: botMelds.map((_, i) => ({ MeldID: `meld_b${i}`, Type: 'run', OwnerID: bot })),
        [me]: myMelds.map((_, i) => ({ MeldID: `meld_m${i}`, Type: 'run', OwnerID: me })),
      },
      RoundReqMet: { [me]: true, [bot]: true },
      DrawPile: ['5H', '6H', 'QS'],
      DiscardPile: ['JS'],
      DeckSeed: 42,
      GameScores: { [me]: [], [bot]: [] },
      TotalScores: { [me]: 0, [bot]: 0 },
    },
    players: live.players.map((p: any) => ({ id: p.id, name: p.name, isAI: !!p.isAI })),
  };
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, { headers: auth, data: { state } });
  expect(res.ok(), await res.text()).toBeTruthy();
  return { matchId, host };
}

/** A CanastaX table with more melds on it than the window holds. */
async function seedCanastaXMelds(request: Ctx) {
  const host = await (
    await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `grip-${Math.random().toString(36).slice(2, 10)}` },
    })
  ).json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const { matchId } = await (
    await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'canasta', variation: 'canastax' } })
  ).json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  const live = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
  const me = host.userId as string;
  const bot = live.players.find((p: any) => p.id !== me).id as string;
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
          meld('t0-Q', 'set', ['QH', 'QD', 'QC'], { rank: 'Q' }),
          meld('t0-seq1', 'run', ['5H', '6H', '7H', '8H'], { suit: 'H' }),
          meld('t0-seq2', 'run', ['9H', 'TH', 'JH'], { suit: 'H' }),
          meld('t0-6', 'set', ['6D', '6C', '6S'], { rank: '6' }),
          meld('t0-7', 'set', ['7D', '7C', '7S'], { rank: '7' }),
        ],
      },
      {
        id: 1, players: [bot], score: 0, hasMelded: true,
        melds: [
          meld('t1-9', 'set', ['9C', '9D', '9S'], { rank: '9' }),
          meld('t1-5', 'set', ['5C', '5D', '5S'], { rank: '5' }),
          meld('t1-A', 'set', ['AC', 'AD', 'AS'], { rank: 'A' }),
          meld('t1-seq', 'run', ['5S', '6S', '7S', '8S'], { suit: 'S' }),
        ],
      },
    ],
    teamOf: { [me]: 0, [bot]: 1 },
    drawPile: Array.from({ length: 30 }, () => '8S'),
    discardPile: ['3C'],
    hands: { [me]: ['4S', 'QS', 'JD', '6C', '7D'], [bot]: ['8C', '8D', 'TC', 'QD'] },
  };
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, { headers: auth, data: { state } });
  expect(res.ok(), await res.text()).toBeTruthy();
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.setViewportSize({ width: WIDTH, height: HEIGHT });
  await page.addInitScript(
    (s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)),
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'grip',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** How far the board has scrolled: the largest scrollTop on the page. */
const scrollOf = (page: Page) =>
  page.evaluate(() => Math.max(0, ...Array.from(document.querySelectorAll('*')).map((e) => e.scrollTop)));

/** Distance from a point to the middle of an element, as drawn right now. */
async function offsetFrom(el: Locator, p: { x: number; y: number }) {
  const b = (await el.boundingBox())!;
  return { dx: b.x + b.width / 2 - p.x, dy: b.y + b.height / 2 - p.y };
}

/**
 * Down on `from`, then in small steps to `to`, holding the button the whole
 * way — the drag helper's carry, but stopping where it is told to.
 */
async function carryTo(page: Page, from: { x: number; y: number }, to: { x: number; y: number }) {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.waitForTimeout(150);
  await page.mouse.move(from.x + 12, from.y - 12, { steps: 4 });
  await page.waitForTimeout(100);
  await page.mouse.move(to.x, to.y, { steps: 20 });
  await page.waitForTimeout(100);
}

/**
 * Holds the pointer at `p` until the board has scrolled by at least `by`, then
 * pulls it back out of the edge band, so nothing moves while it is measured.
 */
async function holdUntilScrolled(page: Page, p: { x: number; y: number }, start: number, by: number) {
  await expect
    .poll(async () => Math.abs((await scrollOf(page)) - start), {
      timeout: 10_000,
      message: 'the premise: the board scrolls while a card is held at the edge',
    })
    .toBeGreaterThan(by);
}

test.describe('a carried card keeps its grip while the board scrolls', () => {
  test('a card from the hand stays under the pointer', async ({ page, request }) => {
    const { matchId, host } = await seedTallBoard(request);
    await openMatch(page, host, matchId);
    await handCards(page);

    const card = cardByCode(page, 'KD');
    await card.scrollIntoViewIfNeeded();
    const grab = await grabPoint(card);
    // Toward the edge the hand is furthest from, where the board has the most
    // to scroll: the hand is docked at one end of the window or the other.
    const towardTop = grab.y > HEIGHT / 2;
    const edge = { x: WIDTH / 2, y: towardTop ? 30 : HEIGHT - 30 };
    const before = await scrollOf(page);

    await carryTo(page, grab, edge);
    // Carried far from the row, the card is drawn with its middle on the
    // pointer (no lift) — this is the grip it must keep.
    const grip = await offsetFrom(card, edge);
    expect(Math.abs(grip.dy), `the card starts under the pointer (${JSON.stringify(grip)})`).toBeLessThan(40);

    await holdUntilScrolled(page, edge, before, 150);
    const after = await offsetFrom(card, edge);
    await release(page);

    expect(
      Math.abs(after.dy - grip.dy),
      `the card slid off the pointer as the board scrolled: grip ${JSON.stringify(grip)} became ${JSON.stringify(after)}`,
    ).toBeLessThan(25);
    expect(Math.abs(after.dx - grip.dx)).toBeLessThan(25);
  });

  test('a card lifted off a meld stays under the pointer', async ({ page, request }) => {
    const { matchId, host } = await seedCanastaXMelds(request);
    await openMatch(page, host, matchId);
    await handCards(page);

    // CanastaX lets a side move its own cards between its melds; the end of
    // a run is a card the board lets you pick up and carry.
    const toggle = page.getByTestId('group-toggle-t0-seq2');
    await toggle.scrollIntoViewIfNeeded();
    await toggle.click();
    const card = page.getByTestId('pick-t0-seq2-2');
    await expect(card).toBeVisible();
    await card.scrollIntoViewIfNeeded();
    const grab = await grabPoint(card);
    const towardTop = grab.y > HEIGHT / 2;
    const edge = { x: WIDTH / 2, y: towardTop ? 30 : HEIGHT - 30 };
    const before = await scrollOf(page);

    await carryTo(page, grab, edge);
    const grip = await offsetFrom(card, edge);
    await holdUntilScrolled(page, edge, before, 150);
    const after = await offsetFrom(card, edge);
    await release(page);

    expect(
      Math.abs(after.dy - grip.dy),
      `the card slid off the pointer as the board scrolled: grip ${JSON.stringify(grip)} became ${JSON.stringify(after)}`,
    ).toBeLessThan(25);
    expect(Math.abs(after.dx - grip.dx)).toBeLessThan(25);
  });
});

/**
 * The player scrolls the board themself while carrying — a trackpad or wheel
 * on the Mac, with the button still held — and then lets go on the meld they
 * scrolled to. Reported on the Mac: the card came back. Only the edge
 * autoscroll re-measured the drop targets, so the drop was judged against
 * where the melds had been before the scroll.
 */
test('a card carried off a meld lands where the player scrolled to', async ({ page, request }) => {
  const { matchId, host } = await seedCanastaXMelds(request);
  await openMatch(page, host, matchId);
  await handCards(page);

  const toggle = page.getByTestId('group-toggle-t0-seq1');
  await toggle.scrollIntoViewIfNeeded();
  await toggle.click();
  const card = page.getByTestId('pick-t0-seq1-3');
  await expect(card).toBeVisible();
  await card.scrollIntoViewIfNeeded();
  const grab = await grabPoint(card);

  // Out to the middle of the window, clear of both edge bands, so nothing
  // but the wheel moves the board.
  const middle = { x: WIDTH / 2, y: HEIGHT / 2 };
  await carryTo(page, grab, middle);
  const before = await scrollOf(page);
  const by = before > 300 ? -240 : 240;
  for (let i = 0; i < 6; i++) {
    await page.mouse.wheel(0, by / 6);
    await page.waitForTimeout(40);
  }
  await expect
    .poll(async () => Math.abs((await scrollOf(page)) - before), { message: 'the premise: the wheel scrolled the board' })
    .toBeGreaterThan(150);

  const target = page.getByTestId('group-toggle-t0-seq2');
  const t = (await target.boundingBox())!;
  await page.mouse.move(t.x + t.width / 2, t.y + t.height / 2, { steps: 10 });
  await page.waitForTimeout(300);
  await release(page);

  const auth = { Authorization: `Bearer ${host.accessToken}` };
  await expect
    .poll(
      async () => {
        const live = await (await request.get(`${API_BASE}/matches/${matchId}`, { headers: auth })).json();
        const zones = live.view?.zones ?? [];
        const group = zones.flatMap((z: any) => z.groups ?? []).find((g: any) => g.id === 't0-seq2');
        return group?.cards ?? [];
      },
      { message: 'the 8 of hearts joined the run it was let go on', timeout: 8000 },
    )
    .toContain('8H');
});
