import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Where the controls sit, in every game and at every width.
 *
 * The rule is one sentence: **the controls are below your hand.** Choosing a
 * card and spending it are one motion, and they used to be at two ends of the
 * screen — the buttons shared the band beside the piles up top, with the whole
 * hand in between, which on a phone is most of the screen.
 *
 * This is asserted as a *geometric* fact rather than a DOM-order one on
 * purpose. Source order is not what a player experiences, and the previous
 * layout could have been reordered in the JSX while still rendering the
 * buttons above the cards — a row that puts its children side by side makes
 * the two orders disagree. What matters is which one is further down the
 * page, so that is what is measured.
 *
 * It runs over every module this repo ships, because "for all the card games"
 * is the actual requirement, and because the shell is supposed to reach a new
 * game without being edited — a game added tomorrow should pass this by
 * construction, and this spec is what would notice if it didn't.
 */

type Ctx = import('@playwright/test').APIRequestContext;

// Every module the shell plays. Named here rather than fetched from /modules
// so that a module quietly disappearing from the server fails this spec
// instead of silently shrinking its coverage to nothing.
const MODULES = ['zolik', 'prsi', 'canasta', 'holdem'] as const;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `ctrls-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  }
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
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
      username: 'ctrls',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/**
 * Which zone is the viewer's own hand, asked of the server rather than
 * guessed from its id.
 *
 * Žolíky, Prší and Canasta call it `hand:<user>`; Hold'em calls it
 * `hole:<user>`. A spec that hardcoded `hand:` would have been asserting
 * this layout for three games and silently skipping the fourth — the same
 * "counting clicks instead of progress" failure the e2e README warns about,
 * in a different costume. `kind === 'hand'` is the field every module
 * declares and the shell itself keys off, so it is what this keys off too.
 */
async function myHandZoneId(request: Ctx, matchId: string, userId: string): Promise<string> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const zone = (body.view?.zones ?? []).find(
    (z: any) => z.kind === 'hand' && z.ownerId === userId,
  );
  expect(zone, `the viewer has a hand zone in ${matchId}`).toBeTruthy();
  return zone.id as string;
}

/** Waits until the viewer's own cards are actually drawn in that zone. */
async function waitForHand(page: Page, zoneId: string) {
  await expect(page.locator(`[data-testid^="card-${zoneId}-"]`).first()).toBeVisible({
    timeout: 20_000,
  });
}

/**
 * The two rectangles this whole spec is about. Both are measured in page
 * coordinates, and the hand is deliberately *not* scrolled into view first:
 * scrolling moves one box and not the other if the second measurement
 * happens after a scroll, which is exactly the kind of drift that makes a
 * layout assertion lie.
 */
async function handAndControls(page: Page, zoneId: string) {
  const hand = await page.getByTestId(`zone-${zoneId}`).boundingBox();
  const controls = await page.getByTestId('controls-panel').boundingBox();
  expect(hand, 'the hand has a bounding box').toBeTruthy();
  expect(controls, 'the controls panel has a bounding box').toBeTruthy();
  return { hand: hand!, controls: controls! };
}

for (const moduleId of MODULES) {
  test.describe(`${moduleId}: the controls are below the hand`, () => {
    test('on a desktop screen', async ({ page, request }) => {
      const { matchId, host } = await tableWithBots(request, moduleId);
      const zoneId = await myHandZoneId(request, matchId, host.userId);
      await openMatch(page, host, matchId);
      await waitForHand(page, zoneId);

      const { hand, controls } = await handAndControls(page, zoneId);

      // Strictly below: the controls start after the hand ends. Not merely
      // "lower down", which a panel overlapping the hand would also satisfy.
      expect(
        controls.y,
        'controls start below the bottom of the hand',
      ).toBeGreaterThanOrEqual(hand.y + hand.height - 1);
    });

    test('on a phone screen', async ({ page, request }) => {
      await page.setViewportSize({ width: 375, height: 812 });

      const { matchId, host } = await tableWithBots(request, moduleId);
      const zoneId = await myHandZoneId(request, matchId, host.userId);
      await openMatch(page, host, matchId);
      await waitForHand(page, zoneId);

      const { hand, controls } = await handAndControls(page, zoneId);

      expect(
        controls.y,
        'controls start below the bottom of the hand',
      ).toBeGreaterThanOrEqual(hand.y + hand.height - 1);
    });
  });
}

test.describe('and the piles are still above it all', () => {
  test('the table band comes before the hand, which comes before the controls', async ({
    page,
    request,
  }) => {
    // The full running order, in one assertion, so that "controls moved down"
    // cannot quietly become "controls moved down and so did the draw pile".
    const { matchId, host } = await tableWithBots(request, 'zolik');
    const zoneId = await myHandZoneId(request, matchId, host.userId);
    await openMatch(page, host, matchId);
    await waitForHand(page, zoneId);

    const draw = await page.getByTestId('zone-draw').boundingBox();
    const { hand, controls } = await handAndControls(page, zoneId);
    expect(draw).toBeTruthy();

    expect(draw!.y, 'the piles sit above the hand').toBeLessThan(hand.y);
    expect(hand.y, 'the hand sits above the controls').toBeLessThan(controls.y);
  });
});
