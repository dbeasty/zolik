import { expect, test, type Page } from '@playwright/test';

import { tapCard } from '../helpers/drag';
import { API_BASE, asViewer } from '../helpers/env';

/**
 * End-to-end for Five-Card Draw, Poker's second variation
 * (docs/new-games-plan.md, P1).
 *
 * `holdem/draw_test.go` proves the rules in memory: the deal, the draw's turn
 * order, every refusal, what each viewer may see, and a bot that plays to the
 * end at every table size. What only this can prove is the screen: that the
 * one control in poker that takes cards — pick one to three, then swap — can
 * be worked by a person, and that what they are shown afterwards is their new
 * hand and the table's public count of what they took.
 */

type Ctx = import('@playwright/test').APIRequestContext;
type Session = { accessToken: string; refreshToken: string; userId: string; username?: string };

async function guest(request: Ctx): Promise<Session> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `draw-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function drawTableWithBot(request: Ctx) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'holdem', variation: 'draw', options: { handLimit: 30 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  expect(bot.ok(), await bot.text()).toBeTruthy();
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, host };
}

async function signIn(page: Page, s: Session) {
  await page.addInitScript((session) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(session));
  }, { ...s, username: s.username ?? 'drawer', isGuest: true });
}

/**
 * The viewer's hole cards by code, in the order drawn. helpers/hand.ts reads a
 * rummy hand (`card-hand:`); a poker hand is the seat's hole zone, so the same
 * reading — the card code each slot publishes as its accessibility label — is
 * done here against `card-hole:<player>`.
 */
async function holeCodes(page: Page, userId: string, selected = false): Promise<string[]> {
  return page.evaluate(
    ({ prefix, selected }) =>
      [...document.querySelectorAll(`[data-testid^="${prefix}"]${selected ? '[aria-selected="true"]' : ''}`)]
        .filter((c) => !c.getAttribute('data-testid')!.includes('concealed'))
        .map((c) => c.closest('[aria-label]')?.getAttribute('aria-label') ?? ''),
    { prefix: `card-hole:${userId}-`, selected },
  );
}

/** Selects exactly these hole cards. */
async function selectHole(page: Page, userId: string, codes: string[]) {
  for (let guard = 0; guard < 10; guard++) {
    const on = await holeCodes(page, userId, true);
    if (on.length === 0) break;
    await tapCard(page, page.locator(`[aria-label="${on[0]}"] [data-testid^="card-hole:${userId}-"]`).first());
  }
  for (const code of codes) {
    await tapCard(page, page.locator(`[aria-label="${code}"] [data-testid^="card-hole:${userId}-"]`).first());
  }
  await expect.poll(async () => (await holeCodes(page, userId, true)).sort()).toEqual(codes.slice().sort());
}

const live = (page: Page, id: string) =>
  page.locator(`[data-testid="offer-${id}"]:not([aria-disabled="true"])`);

/**
 * Plays the cheapest way to the draw: check when free, call when not, and go
 * on to the next hand if the bot folded this one away. Returns once this
 * player is the one drawing.
 */
async function untilDrawing(page: Page) {
  await expect
    .poll(
      async () => {
        // "Swap cards" stays dimmed until cards are picked, so the turn to
        // draw is told by "Stand pat" going live.
        if (await live(page, 'stand').isVisible()) return true;
        for (const id of ['check', 'call', 'continue']) {
          const o = live(page, id);
          if (await o.isVisible()) {
            await o.click();
            return false;
          }
        }
        return false;
      },
      { timeout: 90_000, intervals: [400] },
    )
    .toBe(true);
}

test.describe('five-card draw', () => {
  test('a player swaps two cards and the table is told only how many', async ({ page, request }) => {
    test.setTimeout(150_000);
    const { matchId, host } = await drawTableWithBot(request);
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    // Five face-up cards in my hand, and no board on the table.
    await expect.poll(async () => (await holeCodes(page, host.userId)).length, { timeout: 30_000 }).toBe(5);
    await expect(page.getByTestId('zone-board')).toHaveCount(0);

    await untilDrawing(page);
    await expect(page.getByTestId('offer-discard')).toBeVisible();

    const before = await holeCodes(page, host.userId);
    expect(before).toHaveLength(5);
    const thrown = before.slice(0, 2);
    await selectHole(page, host.userId, thrown);
    await live(page, 'discard').click();

    // My hand: still five, and the two I threw are gone.
    await expect
      .poll(async () => {
        const now = await holeCodes(page, host.userId);
        return now.length === 5 && thrown.every((c) => !now.includes(c));
      }, { timeout: 15_000 })
      .toBe(true);

    // The table: my seat says I drew two.
    await expect(page.getByTestId(`seat-${host.userId}`)).toContainText(/Drew\s*2/);

    // And the server's own view of my hand agrees with the screen.
    const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(host));
    expect(res.ok()).toBeTruthy();
    const state = await res.json();
    const mine = state.view.zones.find((z: { id: string }) => z.id === `hole:${host.userId}`);
    expect(mine.cards.map((c: { card: string }) => c.card)).not.toEqual(expect.arrayContaining(thrown));
  });
});
