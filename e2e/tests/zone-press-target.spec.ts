import { expect, test, type Page } from '@playwright/test';

import { tapCard } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * An offer that names a whole zone has to be reachable on the board.
 *
 * Most of Canasta's targets are a group — a lay-off names the meld it
 * extends, and that meld draws its own press overlay. Two do not: opening a
 * brand-new meld, and the going-out meld of black threes, both of which name
 * the spread itself (`Selector.zoneId`, no `meldId`). `ZoneView` draws one
 * zone-wide overlay for those, and drew it *under* the spread so that a
 * group's own overlay — painted later, and so on top — would still win a tap
 * inside it.
 *
 * Which meant that on any board with melds already on it, the zone-wide
 * overlay caught nothing at all. The groups painted over it are inert when no
 * group is a target, so a tap aimed at the spread landed on a meld that does
 * nothing and died there, silently: the exact shape of the bug the card row
 * had already been fixed for, where a tap meant to discard died on the pile's
 * top card.
 *
 * The visible symptom was a player holding nothing but black threes, with the
 * canastas to go out, who could not lay them by tapping their own spread —
 * the one gesture the hand's own caption tells them to use ("or onto the
 * board to play it"). The control in the bar did work, which is why this
 * survived: every test there was watched the control.
 *
 * So this asserts the thing that regressed invisibly — that the overlay is
 * the element actually under the finger, hit-tested rather than merely
 * present in the DOM — and then that pressing it sends the move.
 */

type Ctx = import('@playwright/test').APIRequestContext;

/**
 * Samba, seeded into "this side has its canastas and this hand is nothing but
 * black threes". Seeded rather than played toward for the reason
 * `canasta-black-three-going-out.spec.ts` sets out at length: six black
 * threes exist in the whole shoe, split across every seat.
 *
 * Two canastas, so the spread has groups on it — which is the whole point:
 * an empty spread has nothing to hide the overlay behind.
 */
async function sambaGoingOutTable(request: Ctx) {
  const guest = async () => {
    const res = await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `zpt-${Math.random().toString(36).slice(2, 10)}` },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
    return res.json();
  };
  const host = await guest();
  const other = await guest();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'samba', options: { targetScore: 10000 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${other.accessToken}` },
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  const melds = ['4', '5'].map((rank) => ({
    id: `t0-${rank}`,
    teamId: 0,
    rank,
    cards: [`${rank}H`, `${rank}D`, `${rank}S`, `${rank}C`, `${rank}H`, `${rank}D`, `${rank}S`],
  }));
  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: {
      state: {
        status: 'active',
        variation: 'samba',
        players: [host.userId, other.userId],
        turnOrder: [host.userId, other.userId],
        current: host.userId,
        phase: 'meld',
        teams: [
          { id: 0, players: [host.userId], score: 0, melds, redThrees: [], hasMelded: true },
          { id: 1, players: [other.userId], score: 0, melds: [], redThrees: [], hasMelded: false },
        ],
        teamOf: { [host.userId]: 0, [other.userId]: 1 },
        drawPile: Array(20).fill('8S'),
        discardPile: ['9H'],
        hands: {
          [host.userId]: ['3C', '3C', '3C', '3S', '3S', '3S'],
          [other.userId]: ['8H', '9H', 'TH'],
        },
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
        winnerTeam: 0,
        seed: 1,
      },
    },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'zpt',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function serverHand(request: Ctx, matchId: string, userId: string): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === userId);
  return (zone?.cards ?? []).map((c: any) => c.card);
}

/**
 * What is actually under the finger, all the way down the overlay.
 *
 * `toBeVisible` would have passed throughout the bug: the overlay was in the
 * DOM, laid out, painted, the right size — and under everything. Only
 * `elementFromPoint` tells a target apart from a target nothing can reach,
 * which is why this reads the hit test rather than the element.
 */
async function whatIsUnderTheSpread(page: Page) {
  await page.locator('[data-testid="zone-melds:t0"]').scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  return page.evaluate(() => {
    const el = document.querySelector('[data-testid="zone-press-melds:t0"]') as HTMLElement | null;
    if (!el) return ['no overlay at all'];
    const r = el.getBoundingClientRect();
    const out: string[] = [];
    for (let share = 0.1; share < 1; share += 0.2) {
      const y = Math.round(r.top + r.height * share);
      const x = Math.round(r.x + r.width / 2);
      if (y < 0 || y > window.innerHeight) continue;
      let n = document.elementFromPoint(x, y) as HTMLElement | null;
      let id: string | null = null;
      while (n && !id) {
        id = n.getAttribute('data-testid');
        n = n.parentElement;
      }
      out.push(id ?? 'nothing');
    }
    return out;
  });
}

/** The middle of whatever part of the spread's overlay is on screen. */
async function pressTheSpread(page: Page) {
  await page.locator('[data-testid="zone-melds:t0"]').scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  const pt = await page.evaluate(() => {
    const el = document.querySelector('[data-testid="zone-press-melds:t0"]') as HTMLElement | null;
    if (!el) return null;
    const r = el.getBoundingClientRect();
    const top = Math.max(r.top, 0);
    const bottom = Math.min(r.bottom, window.innerHeight);
    if (bottom <= top) return null;
    return { x: Math.round(r.x + r.width / 2), y: Math.round((top + bottom) / 2) };
  });
  expect(pt, 'the spread has no reachable press target on screen').not.toBeNull();
  await page.mouse.click(pt!.x, pt!.y);
}

test.describe('an offer that names the whole spread', () => {
  test('is reachable on a board that already has melds, and laying the black threes goes out', async ({
    page,
    request,
  }) => {
    test.setTimeout(90_000);
    const { matchId, host } = await sambaGoingOutTable(request);
    await openMatch(page, host, matchId);
    await expect(page.locator('[data-testid^="card-hand:"]')).toHaveCount(6);

    const cards = page.locator('[data-testid^="card-hand:"]');
    for (let i = 0; i < 6; i++) await tapCard(page, cards.nth(i));
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(6);

    // The assertion the bug would have failed: every point down the spread
    // has to reach the overlay, not the inert melds drawn over it.
    const under = await whatIsUnderTheSpread(page);
    expect(under.length, 'no part of the spread was on screen to hit-test').toBeGreaterThan(2);
    expect(new Set(under), `the spread's own press target is unreachable: ${under.join(', ')}`).toEqual(
      new Set(['zone-press-melds:t0']),
    );

    await pressTheSpread(page);
    await expect.poll(async () => (await serverHand(request, matchId, host.userId)).length).toBe(0);
  });

  test('says what is missing when the pick is not a whole submission yet', async ({ page, request }) => {
    test.setTimeout(90_000);
    const { matchId, host } = await sambaGoingOutTable(request);
    await openMatch(page, host, matchId);
    await expect(page.locator('[data-testid^="card-hand:"]')).toHaveCount(6);

    // One of the six. A drag would gather this — `endDrag` keeps the cards
    // and waits for the next — but a press has nothing left to gather, and
    // used to return in silence while the spread sat there lit.
    await tapCard(page, page.locator('[data-testid^="card-hand:"]').first());
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(1);

    await pressTheSpread(page);
    await expect(page.getByTestId('why-sheet')).toBeVisible();
    await expect(page.getByTestId('why-reason')).toHaveText('Select 6 card(s)');
    expect(await serverHand(request, matchId, host.userId)).toHaveLength(6);
  });
});
