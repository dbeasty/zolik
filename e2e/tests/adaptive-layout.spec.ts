import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * The board on a phone.
 *
 * `board-layout.spec.ts` proves the shell lays a zone out by its *kind*; this
 * proves the same shell stays usable once the screen it's laid out on is a
 * phone's — every control reachable, nothing drawn for a hand nobody can see,
 * two players' spreads sharing a row instead of each claiming one, and a
 * panel a player put away staying away without ever hiding a drop target from
 * a card actually headed for it.
 */

test.use({ viewport: { width: 375, height: 812 } });

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 4) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `adaptive-${Math.random().toString(36).slice(2, 10)}` },
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
      username: 'adaptive',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

test.describe('the board on a phone', () => {
  test('every control fits inside its own rectangle, wrapping onto more than one line', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const bar = page.getByTestId('action-bar');
    await expect(bar).toBeVisible();

    const offers = page.locator('[data-testid^="offer-"]');
    const count = await offers.count();
    expect(count).toBeGreaterThan(1);

    const boxes = [];
    for (let i = 0; i < count; i++) {
      const box = await offers.nth(i).boundingBox();
      expect(box, `offer ${i} has a bounding box`).toBeTruthy();
      boxes.push(box!);
    }

    const screen = await page.getByTestId('match-screen').boundingBox();
    for (const box of boxes) {
      // Every control ends inside the viewport — none of them is sitting off
      // the edge behind an invisible horizontal scroller.
      expect(box.x + box.width).toBeLessThanOrEqual(screen!.x + screen!.width + 1);
    }

    // And they didn't all fit on one line — the row genuinely wrapped rather
    // than merely fitting by coincidence in this particular deal's offer set.
    // Grouped with a tolerance rather than compared exactly: two controls on
    // the same line can differ by a pixel or two from their own heights.
    const rowTops: number[] = [];
    for (const box of boxes) {
      if (!rowTops.some((y) => Math.abs(y - box.y) < 8)) rowTops.push(box.y);
    }
    expect(rowTops.length).toBeGreaterThan(1);
  });

  test('nothing is drawn for a hand nobody can see', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const bodyText = await page.locator('body').innerText();
    expect(bodyText.toLowerCase()).not.toContain('hidden');

    // The only hand zone on screen is the viewer's own.
    const hands = page.locator('[data-testid^="zone-hand:"]');
    await expect(hands).toHaveCount(1);
    await expect(page.getByTestId(`zone-hand:${host.userId}`)).toBeVisible();
  });

  test('two spreads share a row instead of each claiming a full-width line', async ({
    page,
    request,
  }) => {
    // Canasta's team melds zones are sent from the very first deal, whether
    // or not either team has melded yet — deterministic, unlike waiting on a
    // rummy deal to produce one.
    const { matchId, host } = await tableWithBots(request, 'canasta');
    await openMatch(page, host, matchId);
    await handCards(page);

    const spreads = page.locator('[data-testid^="zone-melds:"]');
    await expect(spreads).toHaveCount(2, { timeout: 15_000 });

    const first = await spreads.nth(0).boundingBox();
    const second = await spreads.nth(1).boundingBox();
    expect(first && second).toBeTruthy();

    expect(Math.abs(first!.y - second!.y)).toBeLessThan(8);
    expect(second!.x).toBeGreaterThan(first!.x + first!.width - 1);
  });

  test('a panel put away stays away across a reload', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const panelId = `zone:melds:${host.userId}`;
    const toggle = page.getByTestId(`panel-toggle-${panelId}`);
    await expect(toggle).toBeVisible();
    // Collapsed, this panel has nothing under its header — no drop-here hint,
    // since nothing has been melded yet.
    const zone = page.getByTestId(`zone-melds:${host.userId}`);
    await toggle.click();
    await expect(zone.getByTestId(`drop-here-melds:${host.userId}`)).toHaveCount(0);
    await expect(toggle).toHaveText('▸');

    await page.reload();
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId(`panel-toggle-${panelId}`)).toHaveText('▸');
  });

  test('minimizing a panel never hides it from a card actually headed there', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    const before = await handCards(page);
    test.skip(before.length === 0, 'no cards in hand to drag');

    // Laying a meld is only ever offered after a draw — before that it's
    // disabled, and a disabled offer names no drop target at all. Drawing
    // first is what a player has to do anyway, and what makes the melds
    // zone a live target to test against.
    await page.getByTestId('offer-draw:deck').click();
    await expect(page.locator('[data-testid^="card-hand:"]')).toHaveCount(before.length + 1, {
      timeout: 10_000,
    });

    const panelId = `zone:melds:${host.userId}`;
    await page.getByTestId(`panel-toggle-${panelId}`).click();
    await expect(page.getByTestId(`panel-toggle-${panelId}`)).toHaveText('▸');

    const card = page.locator('[data-testid^="card-hand:"]').first();
    const target = page.getByTestId(`zone-melds:${host.userId}`);

    // A composite lay-meld offer targets the viewer's own melds zone by its
    // whole zone id, not a meldId, and takes any card in hand — so even one
    // card being dragged there is a live (if not yet "ready") drop, and the
    // panel opens for it despite being minimized. Dropped on the panel as it
    // was when the drag began — its collapsed header, which is exactly where
    // a player who cannot yet see the panel's contents would actually aim.
    await dragLocatorTo(page, card, target);

    // Not enough cards for a whole meld, so the drop didn't submit anything —
    // it staged the card, which is the one other thing a drop on a
    // *not-yet-ready* composite target does. That only happens if the drop
    // actually landed on the zone, which only happens if the panel was a
    // registered target despite being minimized.
    await expect(card).toHaveAttribute('aria-selected', 'true');
  });
});
