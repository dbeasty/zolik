import { expect, test, type Page } from '@playwright/test';

import { BOARD_FITS_HEIGHT, dragLocatorTo, grabPoint, release, tapCard } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * A natural carried from one of a side's melds onto the place a wild holds in
 * its sequence trades with the wild (CanastaX): the natural takes its place in
 * the run, the wild takes the natural's place in the meld it left.
 *
 * And a closed meld — stacked, or a canasta folded to its face — gives up a
 * card to a long hold, the mouse included: the meld's plain press opens it, so
 * only a press held still for a moment starts a drag.
 *
 * Seeded through the debug-state hatch as `canasta-move-wild` is; the server's
 * own copy of the melds is what is checked afterwards.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `wild-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

type MeldSeed = { id: string; kind: 'set' | 'run' | 'wild'; rank?: string; suit?: string; low?: number; cards: string[] };

async function seededTable(request: Ctx, melds: MeldSeed[]) {
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
        melds: melds.map((m) => ({ teamId: 0, rank: '', ...m })),
        redThrees: [],
        hasMelded: true,
      },
      { id: 1, players: [p2], score: 0, melds: [], redThrees: [], hasMelded: false },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(20).fill('8S'),
    discardPile: ['9H'],
    hands: { [p1]: ['9S', 'JD', '7C', '6H'], [p2]: ['8H', '9H', 'TH'] },
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

async function openMatch(page: Page, host: any, matchId: string, waitFor: string) {
  // Tall enough that an opened canasta, stacked top to bottom, leaves the
  // meld it is dragged to on screen.
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
      username: 'wild',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId(`group-${waitFor}`)).toBeVisible({ timeout: 15_000 });
}

/** The server's cards for one meld, from the host's own view. */
async function meldCards(request: Ctx, host: any, matchId: string, meldId: string): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, {
    headers: { Authorization: `Bearer ${host.accessToken}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const groups = (body.view?.zones ?? []).flatMap((z: any) => z.groups ?? []);
  return groups.find((g: any) => g.id === meldId)?.cards ?? [];
}

/** Opens a meld's stack, which is what makes its cards pickable. */
async function openMeld(page: Page, meldId: string) {
  const toggle = page.getByTestId(`group-toggle-${meldId}`);
  if ((await toggle.getAttribute('data-expanded')) !== 'true') await toggle.click();
  await expect(toggle).toHaveAttribute('data-expanded', 'true');
}

/** The meld card showing `card`, wherever the group draws it. */
function meldCard(page: Page, meldId: string, card: string) {
  return page.getByTestId(`group-${meldId}`).locator(`[data-card="${card}"]`).first();
}


/** The joker in the hearts run stands for the 6H. */
const RUN = { id: 't0-seq1', kind: 'run' as const, suit: 'H', low: 1, cards: ['5H', 'JOKER1', '7H', '8H'] };
const SIXES = { id: 't0-6', kind: 'set' as const, rank: '6', cards: ['6S', '6D', '6C', '6H'] };
/** A canasta of sixes, folded to its last natural, the 6H. */
const SIX_CANASTA = { id: 't0-6', kind: 'set' as const, rank: '6', cards: ['6S', '6D', '6C', '6S', '6D', '6C', '6H'] };

/** Presses the card, holds it still past the hold, then carries it onto `to`. */
async function holdAndDrag(page: Page, from: ReturnType<Page['locator']>, to: ReturnType<Page['locator']>) {
  await from.scrollIntoViewIfNeeded();
  const grab = await grabPoint(from);
  const box = await to.boundingBox();
  if (!box) throw new Error('no target box');
  const target = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  await page.mouse.move(grab.x, grab.y);
  await page.mouse.down();
  await page.waitForTimeout(600);
  await page.mouse.move((grab.x + target.x) / 2, (grab.y + target.y) / 2, { steps: 10 });
  await page.waitForTimeout(150);
  await page.mouse.move(target.x, target.y, { steps: 15 });
  await page.waitForTimeout(200);
  await release(page);
}

async function expectTraded(request: Ctx, host: any, matchId: string) {
  await expect.poll(() => meldCards(request, host, matchId, 't0-seq1')).toEqual(['5H', '6H', '7H', '8H']);
  expect(await meldCards(request, host, matchId, 't0-6')).toContain('JOKER1');
}

test('an open meld\'s natural trades with the wild in a run, by drag', async ({ page, request }) => {
  const { matchId, host } = await seededTable(request, [RUN, SIXES]);
  await openMatch(page, host, matchId, 't0-seq1');
  await openMeld(page, 't0-6');
  await dragLocatorTo(page, meldCard(page, 't0-6', '6H'), page.getByTestId('group-t0-seq1'));
  await expectTraded(request, host, matchId);
});

test('a closed meld gives up a card to a long hold', async ({ page, request }) => {
  const { matchId, host } = await seededTable(request, [RUN, SIXES]);
  await openMatch(page, host, matchId, 't0-seq1');
  await expect(page.getByTestId('group-toggle-t0-6')).toHaveAttribute('data-expanded', 'false');
  await holdAndDrag(page, meldCard(page, 't0-6', '6H'), page.getByTestId('group-t0-seq1'));
  await expectTraded(request, host, matchId);
  // The press that ended the drag did not also open the meld it came from.
  await expect(page.getByTestId('group-toggle-t0-6')).toHaveAttribute('data-expanded', 'false');
});

test('a folded canasta gives up its face to a long hold', async ({ page, request }) => {
  const { matchId, host } = await seededTable(request, [RUN, SIX_CANASTA]);
  await openMatch(page, host, matchId, 't0-seq1');
  await expect(page.getByTestId('group-folded-t0-6')).toBeVisible();
  await holdAndDrag(page, meldCard(page, 't0-6', '6H'), page.getByTestId('group-t0-seq1'));
  await expectTraded(request, host, matchId);
});

test('without the hold a closed meld does not drag, and its press still opens it', async ({ page, request }) => {
  const { matchId, host } = await seededTable(request, [RUN, SIXES]);
  await openMatch(page, host, matchId, 't0-seq1');
  const toggle = page.getByTestId('group-toggle-t0-6');
  await expect(toggle).toHaveAttribute('data-expanded', 'false');
  await dragLocatorTo(page, meldCard(page, 't0-6', '6H'), page.getByTestId('group-t0-seq1'));
  await page.waitForTimeout(500);
  expect(await meldCards(request, host, matchId, 't0-seq1')).toEqual(['5H', 'JOKER1', '7H', '8H']);

  await page.mouse.click(0, 0);
  await tapCard(page, meldCard(page, 't0-6', '6S'));
  await expect(toggle).toHaveAttribute('data-expanded', 'true');
});
