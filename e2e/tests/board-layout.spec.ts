import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * How the board is laid out, which the shell decides from a zone's *kind* and
 * nothing else.
 *
 * A stack and a pile are a couple of cards across; a hand or a spread of melds
 * needs the width. So the small ones share a line and the wide ones get their
 * own, and a game added tomorrow is laid out by the same rule without this
 * screen being edited. Nothing here — or there — names a draw pile or a
 * discard pile; those are just the zones the games in this repo happen to send
 * with those kinds.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `layout-${Math.random().toString(36).slice(2, 10)}` },
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
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: 'layout',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** How many cards the server says a zone holds, and how many it sent. */
async function zoneCards(request: Ctx, matchId: string, userId: string, zoneId: string) {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const zone = (body.view?.zones ?? []).find((z: any) => z.id === zoneId);
  return { count: zone?.count ?? 0, sent: (zone?.cards ?? []).length as number };
}

/** Cards currently drawn inside one zone. */
const drawnIn = (page: Page, zoneId: string) =>
  page.locator(`[data-testid^="card-${zoneId}-"]`);

test.describe('the shape of the board', () => {
  test('a stack and a pile share a line instead of taking one each', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const draw = await page.getByTestId('zone-draw').boundingBox();
    const discard = await page.getByTestId('zone-discard').boundingBox();
    expect(draw && discard).toBeTruthy();

    // Same line: their tops agree. Stacked vertically they would be a zone's
    // height apart, which is what this used to be.
    expect(Math.abs(draw!.y - discard!.y)).toBeLessThan(8);
    // And genuinely beside each other rather than overlapping.
    expect(discard!.x).toBeGreaterThan(draw!.x + draw!.width - 1);

    // Neither one hogs the row: a two-card zone stretched across the screen is
    // the thing being fixed.
    const screen = await page.getByTestId('match-screen').boundingBox();
    expect(draw!.width).toBeLessThan(screen!.width / 2);
  });

  test('a pile shows its top card, and opens to show what is under it', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    // A deal starts with one card face up, so play on until something is
    // buried under it. Which control does that is the engine's business; this
    // presses whatever is live, as any player would.
    const deadline = Date.now() + 40_000;
    let sent = 0;
    while (Date.now() < deadline) {
      ({ sent } = await zoneCards(request, matchId, host.userId, 'discard'));
      if (sent > 1) break;
      const live = page.locator('[data-testid^="offer-"]:not([aria-disabled="true"])').first();
      if (await live.count()) {
        try {
          await live.click({ timeout: 5000 });
        } catch {
          /* the board moved under us; the next pass re-reads it */
        }
      }
      await page.waitForTimeout(500);
    }
    test.skip(sent <= 1, 'never got a second card onto the pile');

    // Folded: one card on screen however many the server sent.
    await expect(drawnIn(page, 'discard')).toHaveCount(1);
    const toggle = page.getByTestId('zone-toggle-discard');
    await expect(toggle).toBeVisible();

    await toggle.click();
    await expect(drawnIn(page, 'discard')).toHaveCount(sent);

    await toggle.click();
    await expect(drawnIn(page, 'discard')).toHaveCount(1);
  });

  test('a pile of one card has nothing to open', async ({ page, request }) => {
    // Prší and Canasta send only the top card, because what is under it is not
    // public in those games — so there is nothing to unfold and no control
    // offering to. The shell works that out from what it was sent rather than
    // from which game it is.
    const { matchId, host } = await tableWithBots(request, 'prsi');
    await openMatch(page, host, matchId);
    await handCards(page);

    const { sent } = await zoneCards(request, matchId, host.userId, 'discard');
    expect(sent).toBeLessThanOrEqual(1);
    await expect(page.getByTestId('zone-toggle-discard')).toHaveCount(0);
  });
});
