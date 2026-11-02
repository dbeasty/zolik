import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { clearHandSelection } from '../helpers/hand';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * The board on a phone.
 *
 * `board-layout.spec.ts` proves the shell lays a zone out by its *kind*; this
 * proves the same shell stays usable once the screen it's laid out on is a
 * phone's — every control reachable, nothing drawn for a hand nobody can see,
 * two players' spreads sharing a row instead of each claiming one, a
 * partnership's spreads gathered into a panel of their own, and a
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

type Box = { id: string; x: number; y: number; width: number; height: number };

/** The spread zones inside `container`, in the order they are drawn. */
async function spreadBoxes(page: Page, container: string): Promise<Box[]> {
  return page.locator(container).evaluate((root) =>
    [...root.querySelectorAll('[data-testid^="zone-melds:"], [data-testid^="zone-redThrees:"]')].map((e) => {
      const r = e.getBoundingClientRect();
      return { id: (e as HTMLElement).dataset.testid!, x: r.x, y: r.y, width: r.width, height: r.height };
    }),
  );
}

/** Nothing spills off the screen, and spreads on one line sit side by side rather than over each other. */
async function expectPackedWithinScreen(page: Page, boxes: Box[]) {
  const screen = (await page.getByTestId('match-screen').boundingBox())!;
  for (const b of boxes) {
    expect(b.x, `${b.id} starts on screen`).toBeGreaterThanOrEqual(screen.x - 1);
    expect(b.x + b.width, `${b.id} ends on screen`).toBeLessThanOrEqual(screen.x + screen.width + 1);
  }
  for (let i = 1; i < boxes.length; i++) {
    const [a, b] = [boxes[i - 1], boxes[i]];
    if (Math.abs(a.y - b.y) < 8) expect(b.x, `${b.id} sits beside ${a.id}`).toBeGreaterThanOrEqual(a.x + a.width - 1);
  }
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

  // Canasta's melds zones are sent from the very first deal, whether or not
  // anyone has melded yet — deterministic, unlike waiting on a rummy deal to
  // produce one. Its red-threes zones come and go with the deal, so neither
  // test below depends on whether there are any.
  //
  // A spread is no longer kept narrower than the screen: one that wraps onto
  // a line of its own grows across it (see `stretchesInRow`), which on a phone
  // is the whole width. What still holds is that spreads with room beside
  // each other share a line, and that nothing spills off the screen.

  test('two spreads share a row instead of each claiming a full-width line', async ({
    page,
    request,
  }) => {
    // Heads-up: every player is their own side, so no side panels — the
    // spreads stand on their own in one wrapping row.
    const { matchId, host } = await tableWithBots(request, 'canasta', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    await expect(page.locator('[data-testid^="zone-melds:"]')).toHaveCount(2, { timeout: 15_000 });
    await expect(page.locator('[data-testid^="section-side-"]')).toHaveCount(0);

    // Two spreads always fit side by side on a phone, so the first two share
    // a line whichever two they are — both melds zones, or a melds zone and
    // its red threes.
    const boxes = await spreadBoxes(page, '[data-testid="section-spreads"]');
    expect(boxes.length).toBeGreaterThanOrEqual(2);
    expect(Math.abs(boxes[0].y - boxes[1].y), 'the first two spreads share a line').toBeLessThan(8);
    await expectPackedWithinScreen(page, boxes);
  });

  test('a partnership’s spreads are one panel of their own, yours first', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'canasta', 4);
    await openMatch(page, host, matchId);
    await handCards(page);

    const mine = page.getByTestId('section-side-mine');
    const theirs = page.locator('[data-testid^="section-side-"]:not([data-testid="section-side-mine"])');
    await expect(mine).toBeVisible({ timeout: 15_000 });
    await expect(theirs).toHaveCount(1);

    // Each side is a line of its own, and the viewer's side comes first —
    // nearest the hand, which is what they play into.
    const m = (await mine.boundingBox())!;
    const t = (await theirs.boundingBox())!;
    expect(t.y, 'the other side starts below yours').toBeGreaterThanOrEqual(m.y + m.height - 1);

    // A side's own zones — its melds, and its red threes when it has any —
    // are inside its panel and share its row.
    for (const panel of ['[data-testid="section-side-mine"]', '[data-testid^="section-side-"]:not([data-testid="section-side-mine"])']) {
      const boxes = await spreadBoxes(page, panel);
      expect(boxes.length).toBeGreaterThanOrEqual(1);
      if (boxes.length > 1) expect(Math.abs(boxes[0].y - boxes[1].y), 'a side’s zones share a line').toBeLessThan(8);
      await expectPackedWithinScreen(page, boxes);
    }
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
    await waitForOfferEnabled(page, 'offer-draw:deck');
    await page.getByTestId('offer-draw:deck').click();
    await expect(page.locator('[data-testid^="card-hand:"]')).toHaveCount(before.length + 1, {
      timeout: 10_000,
    });

    // The drawn card lands *selected*, and a selected card lights up
    // everywhere it could go — including this very panel, which is then held
    // open as a live target and cannot be minimized at all. That is the panel
    // rule working (a drop target may never be hidden by a preference), but
    // it leaves nothing minimized to test, so put the selection down first.
    // Without this the toggle below reads "▾" and the whole point of the test
    // is lost.
    await clearHandSelection(page);

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

  test('a card in the fan does not block the page from being scrolled past it', async ({
    page,
    request,
  }) => {
    // The bug this guards against: attaching a drag gesture to an element
    // makes react-native-gesture-handler set that element's own `touch-action`
    // to `none` on the web build, unconditionally — every touch that begins
    // on a card is claimed for the gesture before the browser's native touch
    // scrolling ever gets a say, vertical swipe included. On a phone the hand
    // is the biggest touch target on the page and sits mid-scroll, so that
    // silently made the board unscrollable the moment a finger landed on any
    // of a dozen-plus cards. `pan-y` hands vertical panning back to the
    // browser; a horizontal drag (rearranging the fan) is untouched, since
    // `touch-action` only ever gives up the axis it's told to.
    //
    // Asserting the CSS rather than driving an actual touch-scroll gesture:
    // native touch-scroll-vs-gesture arbitration is a browser compositor
    // decision that CDP-driven touch synthesis does not reliably reproduce,
    // where the `touch-action` value react-native-gesture-handler committed
    // to the DOM is exactly the fact a regression here would change.
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    const touchActions = await page
      .locator('[data-testid^="card-hand:"]')
      .first()
      .evaluate((card) => {
        // Several ancestors between the card's own ring and the hand row set
        // their own `touch-action` for reasons that have nothing to do with
        // this gesture (the ring itself uses `manipulation`, to suppress
        // double-tap-to-zoom) — collect every non-default value on the way up
        // rather than assuming which level the gesture's own is at.
        const seen: string[] = [];
        let node: HTMLElement | null = card as HTMLElement;
        for (let i = 0; i < 6 && node; i++, node = node.parentElement) {
          const value = getComputedStyle(node).touchAction;
          if (value !== 'auto') seen.push(value);
        }
        return seen;
      });

    expect(touchActions).toContain('pan-y');
    expect(touchActions).not.toContain('none');
  });
});
