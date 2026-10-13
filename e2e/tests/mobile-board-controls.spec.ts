import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * Three things a phone-width board got wrong.
 *
 * - The skin toggle was pushed past the right edge by the header's links.
 * - The log box's arrow opened a box that showed one line after your own move,
 *   instead of the previous plays.
 * - A standing requirement ("your side's first meld must reach 50") shared the
 *   log box with the moves; it now has a sheet of its own, unmet only, most
 *   relevant first.
 */

test.use({ viewport: { width: 375, height: 812 } });

type Ctx = import('@playwright/test').APIRequestContext;

async function canastaTable(request: Ctx, moduleId = 'canasta', seats = 4) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `mobctl-${Math.random().toString(36).slice(2, 10)}` },
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
  for (let i = 1; i < seats; i++) await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript(
    (s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)),
    { accessToken: host.accessToken, refreshToken: host.refreshToken, userId: host.userId, username: 'mobctl', isGuest: true },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await handCards(page);
}

test('the skin toggle is on screen and is what a tap at its centre hits', async ({ page, request }) => {
  const { matchId, host } = await canastaTable(request);
  await openMatch(page, host, matchId);

  const toggle = page.getByTestId('skin-toggle');
  await toggle.scrollIntoViewIfNeeded();
  const box = (await toggle.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(375);
  const hit = await page.evaluate(
    ([x, y]) => !!document.elementFromPoint(x, y)?.closest('[data-testid="skin-toggle"]'),
    [box.x + box.width / 2, box.y + box.height / 2],
  );
  expect(hit).toBe(true);

  // And it works from there.
  const before = await toggle.innerText();
  await toggle.click();
  await expect(toggle).not.toHaveText(before);
});

test('opening the log shows earlier plays, still, after your own move', async ({ page, request }) => {
  const { matchId, host } = await canastaTable(request);
  await openMatch(page, host, matchId);

  await page.getByTestId('moves-toggle').click();

  // One full turn, so there are moves on both sides of ours.
  await waitForOfferEnabled(page, 'offer-draw:deck', 60_000);
  await page.getByTestId('offer-draw:deck').click();
  const discard = page.locator('[data-testid^="offer-discard"]').first();
  await expect(discard).toBeVisible({ timeout: 15_000 });
  await handCards(page);
  await page.locator('[data-testid^="card-hand:"]').first().click({ position: { x: 4, y: 20 } });
  await expect(discard).not.toHaveAttribute('aria-disabled', 'true', { timeout: 10_000 });
  await discard.click();

  // Our own move is the newest or near it; the open box must still list the
  // plays before it, not collapse to one line.
  await expect.poll(async () => page.locator('[data-testid^="announce-"]').count(), { timeout: 30_000 }).toBeGreaterThan(2);
  // The newest line is in view (the region scrolls to it).
  const lines = page.locator('[data-testid^="announce-"]');
  const scroll = (await page.getByTestId('moves-scroll').boundingBox())!;
  const lastBox = (await lines.last().boundingBox())!;
  expect(lastBox.y + lastBox.height).toBeLessThanOrEqual(scroll.y + scroll.height + 1);

  // Folding goes back to a single line.
  await page.getByTestId('moves-toggle').click();
  await expect(lines).toHaveCount(1);
});

test('requirement hints live in their own sheet, not the log', async ({ page, request }) => {
  const { matchId, host } = await canastaTable(request);
  await openMatch(page, host, matchId);

  // Nothing in the log box is a requirement…
  const box = page.getByTestId('move-announcements');
  await expect(box).not.toContainText('first meld');
  await expect(box).not.toContainText('canastas before');

  // …they are behind a button that counts them.
  const open = page.getByTestId('hints-open');
  await expect(open).toBeVisible();
  await expect(open).toContainText('2');
  await open.click();

  const sheet = page.getByTestId('hints-sheet');
  await expect(sheet).toBeVisible();
  // Most relevant first: the first meld, then the canastas needed to go out.
  await expect(page.getByTestId('hint-0')).toContainText('first meld');
  await expect(page.getByTestId('hint-1')).toContainText(/canastas?/);
  await page.getByTestId('hints-close').click();
  await expect(sheet).toBeHidden();
});

test('Rummy Tiles: the first-lay requirement is a hint, the turn prompt stays in the log', async ({ page, request }) => {
  const { matchId, host } = await canastaTable(request, 'rummytiles');
  await openMatch(page, host, matchId);

  const box = page.getByTestId('move-announcements');
  await expect(box).not.toContainText('first lay');

  const open = page.getByTestId('hints-open');
  await expect(open).toContainText('1');
  await open.click();
  await expect(page.getByTestId('hint-0')).toContainText('first lay');
  await page.getByTestId('hints-close').click();

  // The turn prompt is worded, not a raw key (it had no wording before).
  await expect(box).not.toContainText('rummytiles.prompt');
});

test('with no move above it, a long turn prompt stops short of the corner buttons', async ({ page, request }) => {
  // Mariáš words its opening prompt at length ("Say dobrá, or take it over with
  // a higher game"); when it is the box's first line it shares the row with the
  // hints, repeat and fold buttons and used to run under them.
  //
  // That needs the host to act first, before anyone has played: three seats
  // (at four the dealer sits the deal out and holds nothing), and a fresh table
  // until the deal opens on the host. Any other opening puts a bot's move above
  // the prompt, or leaves no prompt at all, and the row under test is not there.
  let table: { matchId: string; host: any } | undefined;
  for (let attempt = 0; attempt < 15 && !table; attempt++) {
    const t = await canastaTable(request, 'marias', 3);
    const state = await (
      await request.get(`${API_BASE}/matches/${t.matchId}`, { headers: { Authorization: `Bearer ${t.host.accessToken}` } })
    ).json();
    if ((state.legalActions ?? []).some((o: any) => o.enabled)) table = t;
  }
  expect(table, 'a deal that opens on the host').toBeTruthy();
  const { matchId, host } = table!;
  await openMatch(page, host, matchId);

  const prompt = page.getByTestId('prompt-0');
  await expect(prompt).toBeVisible();
  await expect(page.locator('[data-testid^="announce-"]'), 'no move above the prompt').toHaveCount(0);
  const clash = await page.evaluate(() => {
    const p = document.querySelector('[data-testid="prompt-0"]');
    const corner = document.querySelector('[data-testid="moves-repeat"]');
    if (!p || !corner) return 'missing';
    const range = document.createRange();
    range.selectNodeContents(p);
    const c = corner.getBoundingClientRect();
    for (const r of Array.from(range.getClientRects())) {
      const overlapsRow = r.top < c.bottom && r.bottom > c.top;
      if (overlapsRow && r.right > c.left) return `text ends at ${r.right}, corner starts at ${c.left}`;
    }
    return '';
  });
  expect(clash).toBe('');
});
