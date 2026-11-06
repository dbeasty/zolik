import { expect, test, type Page } from '@playwright/test';

import { BOARD_FITS_HEIGHT, dragLocatorTo } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * A 2 moves between a side's own melds the way a joker does.
 *
 * CanastaX lets an opened side move cards between its melds. Both a 2 and a
 * joker are wild there, and the engine treats them alike — yet at the table a
 * joker could be carried from one meld to another while a 2 could not.
 *
 * Seeded through the debug-state hatch (see `canasta-folded-face`): a group of
 * kings holding one of each wild, and a group of queens to move them onto.
 * Each wild is moved by a tap-then-target and by a drag, and the server's own
 * copy of the melds is what is checked afterwards.
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

/**
 * The layouts each wild is moved in. `from` holds both wilds, so the same table
 * asks the question of each; `to` is where they go. Every row is one the engine
 * accepts for either wild (see the canasta package's move tests), so a refusal
 * here is the client's.
 */
const LAYOUTS: { name: string; melds: MeldSeed[]; from: string; to: string }[] = [
  {
    name: 'group to group',
    melds: [
      { id: 't0-K', kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', '2C', 'JOKER1'] },
      { id: 't0-Q', kind: 'set', rank: 'Q', cards: ['QH', 'QD', 'QS'] },
    ],
    from: 't0-K',
    to: 't0-Q',
  },
  {
    name: 'group to a group already holding the same 2',
    melds: [
      { id: 't0-K', kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', '2C', 'JOKER1'] },
      { id: 't0-Q', kind: 'set', rank: 'Q', cards: ['QH', 'QD', 'QS', 'QC', '2C'] },
    ],
    from: 't0-K',
    to: 't0-Q',
  },
  {
    name: 'folded canasta to group',
    melds: [
      { id: 't0-K', kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', 'KH', '2C', 'JOKER1', 'KD'] },
      { id: 't0-Q', kind: 'set', rank: 'Q', cards: ['QH', 'QD', 'QS'] },
    ],
    from: 't0-K',
    to: 't0-Q',
  },
  {
    name: 'group to a sequence, either end',
    melds: [
      { id: 't0-K', kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', '2C', 'JOKER1'] },
      { id: 't0-seq1', kind: 'run', suit: 'C', cards: ['8C', '9C', 'TC'] },
    ],
    from: 't0-K',
    to: 't0-seq1',
  },
  {
    name: 'group to the meld of 2s',
    melds: [
      { id: 't0-K', kind: 'set', rank: 'K', cards: ['KH', 'KD', 'KS', 'KC', '2C', 'JOKER1'] },
      { id: 't0-2', kind: 'wild', cards: ['2S', '2D', '2H'] },
    ],
    from: 't0-K',
    to: 't0-2',
  },
];

/** A sequence's two wilds sit on its ends: the 2 stands for the 4, the joker for the 9. */
const RUN_LAYOUT = {
  name: 'sequence end to group',
  melds: [
    { id: 't0-seq1', kind: 'run' as const, suit: 'H', low: 0, cards: ['2C', '5H', '6H', '7H', '8H', 'JOKER1'] },
    { id: 't0-Q', kind: 'set' as const, rank: 'Q', cards: ['QH', 'QD', 'QS'] },
  ],
  from: 't0-seq1',
  to: 't0-Q',
};
LAYOUTS.push(RUN_LAYOUT);

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

for (const layout of LAYOUTS) {
  for (const wild of ['2C', 'JOKER1']) {
    test(`${layout.name}: ${wild} moves by tap`, async ({ page, request }) => {
      const { matchId, host } = await seededTable(request, layout.melds);
      await openMatch(page, host, matchId, layout.to);
      const before = (await meldCards(request, host, matchId, layout.to)).filter((c) => c === wild).length;
      await openMeld(page, layout.from);

      await meldCard(page, layout.from, wild).click();
      await expect(page.getByTestId(`group-press-${layout.to}`)).toBeVisible({ timeout: 5_000 });
      await page.getByTestId(`group-press-${layout.to}`).click();

      await expect
        .poll(async () => (await meldCards(request, host, matchId, layout.to)).filter((c) => c === wild).length)
        .toBe(before + 1);
    });

    test(`${layout.name}: ${wild} moves by drag`, async ({ page, request }) => {
      const { matchId, host } = await seededTable(request, layout.melds);
      await openMatch(page, host, matchId, layout.to);
      const before = (await meldCards(request, host, matchId, layout.to)).filter((c) => c === wild).length;
      await openMeld(page, layout.from);

      await dragLocatorTo(page, meldCard(page, layout.from, wild), page.getByTestId(`group-${layout.to}`));

      await expect
        .poll(async () => (await meldCards(request, host, matchId, layout.to)).filter((c) => c === wild).length)
        .toBe(before + 1);
    });
  }
}
