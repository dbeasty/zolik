import { devices, expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { grabPoint, handCards, touchHold } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * A card being carried stays under the pointer when the board moves the hand.
 *
 * The carried card is drawn inside the hand's row, so anything that moves the
 * row moves the card — and the board above the hand does change height
 * mid-drag. The one that was seen: on a fresh deal the moves box is not drawn
 * at all (it never shows an empty box), so when the bot's opening move lands
 * the box appears, the hand drops by its height, and the card dropped with it,
 * ending up a finger's width *below* the pointer and staying there for the
 * rest of the drag. On a phone that is a visible jump under the finger.
 *
 * So these deal until the bot is the one to open, take a card up before its
 * move lands, and hold it still while the hand moves underneath — the case
 * where nothing but the reflow can put the card right, because the pointer
 * gives it no reason to re-render.
 */

type Ctx = APIRequestContext;

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);
const row = (page: Page) => page.locator('[data-testid^="hand-hand:"]').first();

/** The carried card's box: the one card positioned out of the row's flow. */
async function carriedBox(page: Page) {
  return page.evaluate(() => {
    const floating = (el: Element) => {
      let node: Element | null = el;
      while (node && !(node as HTMLElement).dataset?.testid?.startsWith('hand-')) {
        if (getComputedStyle(node as HTMLElement).position === 'absolute') return true;
        node = node.parentElement;
      }
      return false;
    };
    const el = Array.from(document.querySelectorAll('[data-testid^="card-hand:"]')).find(floating);
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y, width: r.width, height: r.height };
  });
}

/**
 * A two-seat Žolíky table against a bot, open on screen with the bot to make
 * the first move and that move not yet made. Who opens a deal is the deal's
 * business, so this deals until it is the bot.
 *
 * The page is opened *before* the match starts, so the deal arrives on a
 * screen that is already up and the drag can begin inside the bot's think
 * time rather than after a page load has eaten it.
 */
async function botToOpen(page: Page, request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `reflow-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: 'reflow',
    isGuest: true,
  });

  for (let attempt = 0; attempt < 12; attempt++) {
    const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'zolik', options: {} } });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();
    expect((await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth })).ok()).toBeTruthy();

    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();
    await handCards(page);

    const draw = page.getByTestId('offer-draw:deck');
    await expect(draw).toBeVisible({ timeout: 10_000 });
    const botsTurn = (await draw.getAttribute('aria-disabled')) === 'true';
    const nothingSaid = (await page.getByTestId('announce-0').count()) === 0;
    if (botsTurn && nothingSaid) return;
  }
  throw new Error('the bot never opened a deal in twelve tries');
}

/**
 * The claim, for either kind of pointer: pick a card up, rest it in the hand
 * while the bot's move brings the moves box in above, then carry it up over
 * the board. At every stop the card stays across the pointer within 8px, it
 * is never pushed below the finger, and it ends up under it.
 */
async function carryThroughReflow(
  page: Page,
  pointer: {
    down(at: { x: number; y: number }): Promise<void>;
    move(to: { x: number; y: number }): Promise<void>;
    up(): Promise<void>;
  },
) {
  const offsets: string[] = [];
  const check = async (at: { x: number; y: number }, when: string) => {
    const carried = await carriedBox(page);
    expect(carried, `nothing was lifted out ${when}`).toBeTruthy();
    const offX = carried!.x + carried!.width / 2 - at.x;
    const offY = carried!.y + carried!.height / 2 - at.y;
    offsets.push(`${when}: ${Math.round(offX)},${Math.round(offY)}`);
    expect(Math.abs(offX), `the card is ${Math.round(offX)}px across from the pointer ${when}`).toBeLessThan(8);
    // Held above the finger in the hand, settled under it over the board —
    // never below it, which is where riding down with the hand put it.
    expect(offY, `the card is ${Math.round(offY)}px below the pointer ${when} (${offsets.join('; ')})`).toBeLessThan(3);
    return offY;
  };

  // The bot's think time is a second or two, and the card has to be in hand
  // before it is up. When it wins the race, the drag reflowed nothing and
  // proves nothing, so it is put down and dealt again.
  let grab = { x: 0, y: 0 };
  let rest = grab;
  let rowBefore: { y: number } | null = null;
  for (let attempt = 0; ; attempt++) {
    await botToOpen(page, page.request);
    await card(page, 6).scrollIntoViewIfNeeded();
    grab = await grabPoint(card(page, 6));
    rowBefore = await row(page).boundingBox();
    if (!rowBefore) throw new Error('no hand row');

    await pointer.down(grab);
    rest = { x: grab.x + 40, y: grab.y - 20 };
    await pointer.move(rest);
    if ((await page.getByTestId('announce-0').count()) === 0) break;
    await pointer.up();
    if (attempt === 8) throw new Error('the bot always moved before the card was picked up');
  }

  try {
    await check(rest, 'before the bot moved');

    // Held still: the bot's move lands and the moves box appears above.
    await expect(page.getByTestId('announce-0')).toBeVisible({ timeout: 15_000 });
    await page.waitForTimeout(250);
    const rowAfter = await row(page).boundingBox();
    if (!rowAfter || !rowBefore) throw new Error('no hand row');
    expect(
      rowAfter.y - rowBefore.y,
      'the hand did not move when the moves box appeared, so this tested nothing',
    ).toBeGreaterThan(20);
    await check(rest, `after the hand moved ${Math.round(rowAfter.y - rowBefore.y)}px, pointer still`);

    // ...and carried on up over the board, where it settles under the finger.
    const path = [
      { dx: 120, dy: -80 },
      { dx: 260, dy: -160 },
      { dx: 420, dy: -240 },
      { dx: 520, dy: -320 },
    ].map(({ dx, dy }) => ({ x: Math.min(grab.x + dx, page.viewportSize()!.width - 20), y: grab.y + dy }));
    for (const at of path) {
      await pointer.move(at);
      await check(at, `at ${Math.round(at.x)},${Math.round(at.y)}`);
    }
    const last = path[path.length - 1];
    await expect
      .poll(
        async () => {
          const at = await carriedBox(page);
          return at ? Math.abs(at.y + at.height / 2 - last.y) : null;
        },
        { message: `the card never settled under the finger (${offsets.join('; ')})` },
      )
      .toBeLessThan(8);
  } finally {
    await pointer.up();
  }
}

test.describe('a card carried while the board reflows', () => {
  test.describe('with a mouse', () => {
    // Tall enough that the whole board, moves box and all, fits without
    // scrolling — so the hand moving is the board's doing, not the page's.
    test.use({ viewport: { width: 1280, height: 1400 } });

    test('stays under the pointer when the moves box appears above the hand', async ({ page }) => {
      await carryThroughReflow(page, {
        down: async (at) => {
          await page.mouse.move(at.x, at.y);
          await page.mouse.down();
          await page.waitForTimeout(100);
        },
        move: async (to) => {
          await page.mouse.move(to.x, to.y, { steps: 12 });
          await page.waitForTimeout(150);
        },
        up: async () => {
          await page.mouse.up();
          await page.waitForTimeout(300);
        },
      });
    });
  });

  test.describe('with a finger', () => {
    const { defaultBrowserType: _ignored, viewport: _phone, ...pixel } = devices['Pixel 7'];
    // A phone, but a tall one: on a short screen the board scrolls, and the
    // browser's scroll anchoring may hold the hand still on screen by
    // scrolling the page for it — which would make this pass for nothing.
    test.use({ ...pixel, viewport: { width: 412, height: 1800 } });

    test('stays under the finger when the moves box appears above the hand', async ({ page }) => {
      let finger: Awaited<ReturnType<typeof touchHold>> | null = null;
      await carryThroughReflow(page, {
        down: async (at) => {
          // Just past the client's HOLD_TO_DRAG_MS: the bot is thinking.
          finger = await touchHold(page, at, { holdMs: 350 });
        },
        move: async (to) => {
          await finger!.move(to, { steps: 12 });
        },
        up: async () => {
          await finger?.lift();
        },
      });
    });
  });
});
