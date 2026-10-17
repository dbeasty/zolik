import { devices, expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { grabPoint, handCards, touchCarry, touchDragLocatorTo } from '../helpers/drag';
import { API_BASE, asViewer, type Viewer } from '../helpers/env';
import { openGame } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * Dragging cards with a finger, on a phone-sized touch screen.
 *
 * Every other drag spec drives a mouse, and a mouse never scrolls a page by
 * dragging — so they all passed while a phone could not carry a card onto the
 * board at all. The browser read the vertical swipe toward the table as a
 * scroll, cancelled the pointer, and the card snapped back into the hand.
 *
 * On a touch screen a card is picked up by resting a finger on it (see
 * `useCardPan` in the client); a swipe that does not rest first is still the
 * page's to scroll. These specs use real touch events, so the browser's own
 * scroll arbitration is in the loop the way it is on a phone.
 */

const { defaultBrowserType: _ignored, ...pixel } = devices['Pixel 7'];
test.use({ ...pixel, contextOptions: { reducedMotion: 'reduce' } });

type Ctx = APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `touch-${Math.random().toString(36).slice(2, 10)}` },
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
    username: 'touch',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function board(request: Ctx, matchId: string, viewer: Viewer) {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(viewer));
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  return body.state ?? body;
}

async function serverHand(request: Ctx, matchId: string, viewer: Viewer): Promise<string[]> {
  const body = await board(request, matchId, viewer);
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === viewer.userId);
  return (zone?.cards ?? []).map((c: any) => c.card);
}

async function offerFor(request: Ctx, matchId: string, viewer: Viewer, verb: string) {
  const body = await board(request, matchId, viewer);
  const offer = (body.legalActions ?? []).find((o: any) => o.verb === verb && o.enabled);
  return { offer, cards: (offer?.source?.cards ?? []) as string[] };
}

/**
 * Presses whatever is live — drawing, mostly — and waits through the bots'
 * turns, until the engine offers `verb`.
 */
async function playUntilOffered(page: Page, request: Ctx, matchId: string, viewer: Viewer, verb: string) {
  const deadline = Date.now() + 40_000;
  while (Date.now() < deadline) {
    const { offer } = await offerFor(request, matchId, viewer, verb);
    if (offer) return offer;
    const live = page
      .locator('[data-testid^="offer-"]:not([data-testid$="-title"]):not([aria-disabled="true"])')
      .first();
    if (await live.count()) {
      try {
        await live.click({ timeout: 3000 });
      } catch {
        /* the board moved under us; the next pass re-reads it */
      }
    }
    await page.waitForTimeout(500);
  }
  return null;
}

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);

const scrollTop = (page: Page) =>
  page.evaluate(() => {
    // Whichever is scrolling: the document, or the match screen's scroll view.
    const all = [document.scrollingElement, ...document.querySelectorAll('*')] as Element[];
    return all.reduce((sum, el) => sum + (el?.scrollTop ?? 0), 0);
  });

test.describe('dragging a card with a finger', () => {
  // Gesture-handler reports a misconfigured gesture on the console and then
  // quietly drops the touch, which looks like a flaky drag rather than a bug.
  let gestureErrors: string[] = [];
  test.beforeEach(({ page }) => {
    gestureErrors = [];
    page.on('console', (m) => {
      if (m.type() === 'error' && m.text().includes('react-native-gesture-handler')) gestureErrors.push(m.text());
    });
  });
  test.afterEach(() => {
    expect(gestureErrors).toEqual([]);
  });

  test('a card held and carried up onto the discard pile is played', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'prsi');
    await openMatch(page, host, matchId);
    await handCards(page);

    test.setTimeout(90_000);
    const offer = await playUntilOffered(page, request, matchId, host, 'play_card');
    test.skip(!offer, 'never reached a turn where a card could be played');
    const { cards: playable } = await offerFor(request, matchId, host, 'play_card');
    const before = await serverHand(request, matchId, host);
    // A card that asks nothing when it lands: a queen asks for a suit.
    const index = before.findIndex((c) => playable.includes(c) && !c.startsWith('Q'));
    test.skip(index < 0, 'only queens were playable');
    const dragged = before[index];

    // The pile is above the hand, so this is the vertical drag a phone used
    // to turn into a scroll.
    const pile = page.getByTestId('zone-discard');
    const from = await (async () => {
      await card(page, index).scrollIntoViewIfNeeded();
      return grabPoint(card(page, index));
    })();
    const box = (await pile.boundingBox())!;
    expect(box.y + box.height / 2).toBeLessThan(from.y - 40);

    await touchDragLocatorTo(page, card(page, index), pile);

    await expect
      .poll(async () => (await serverHand(request, matchId, host)).join(','), { timeout: 10_000 })
      .not.toContain(before.join(','));
    const after = await serverHand(request, matchId, host);
    expect(after.filter((c) => c === dragged).length).toBe(before.filter((c) => c === dragged).length - 1);
  });

  test('a card in flight follows the finger up off the hand, and the page does not scroll', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'prsi');
    await openMatch(page, host, matchId);
    await handCards(page);

    const c = card(page, 1);
    await c.scrollIntoViewIfNeeded();
    const from = await grabPoint(c);
    const scrolled = await scrollTop(page);
    const to = { x: from.x, y: Math.max(40, from.y - 220) };
    const lift = await touchCarry(page, from, to);
    // Mid-flight: the card is with the finger — held a little above it, so
    // the finger does not hide it — and nothing scrolled.
    const box = (await c.boundingBox())!;
    expect(Math.abs(box.y + box.height / 2 - to.y)).toBeLessThan(box.height * 1.5);
    expect(box.y + box.height / 2).toBeLessThan(from.y - 120);
    expect(await scrollTop(page)).toBe(scrolled);
    await lift();
  });

  test.describe('on a screen too short for the whole board', () => {
    test.use({ viewport: { width: 412, height: 520 } });

    test('a quick swipe over the hand still scrolls, and picks nothing up', async ({ page, request }) => {
      const { matchId, host } = await tableWithBots(request, 'prsi');
      await openMatch(page, host, matchId);
      const before = await handCards(page);
      const serverBefore = await serverHand(request, matchId, host);

      // The hand sits below the fold here, so bringing it into view leaves
      // the page scrolled — with room to swipe back up toward the table.
      const c = card(page, 1);
      await c.scrollIntoViewIfNeeded();
      const scrolled = await scrollTop(page);
      expect(scrolled).toBeGreaterThan(50);
      const from = await grabPoint(c);

      // No rest before moving: a swipe, which belongs to the page.
      const lift = await touchCarry(page, from, { x: from.x, y: from.y + 200 }, { holdMs: 0, steps: 8 });
      await lift();

      expect(await scrollTop(page)).toBeLessThan(scrolled - 50);
      expect(await handCards(page)).toEqual(before);
      expect(await serverHand(request, matchId, host)).toEqual(serverBefore);
    });
  });

  test('a card held and carried along the hand is rearranged there', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'prsi');
    await openMatch(page, host, matchId);
    const before = await handCards(page);
    expect(before.length).toBeGreaterThan(3);

    const first = card(page, 0);
    const last = card(page, before.length - 1);
    await first.scrollIntoViewIfNeeded();
    const from = await grabPoint(first);
    const lastBox = (await last.boundingBox())!;
    const lift = await touchCarry(page, from, { x: lastBox.x + lastBox.width - 4, y: from.y });
    await lift();

    await expect.poll(async () => (await handCards(page))[before.length - 1]).toBe(before[0]);
  });

  test('a card held on the table is carried to another column', async ({ page, request }) => {
    const me = await loginAsFreshGuest(page, request, `tsol-${Math.random().toString(36).slice(2, 8)}`);
    await openGame(page, 'klondike');
    await page.getByTestId('play-solo-klondike').click();
    await page.getByTestId('deal-me-in-klondike').click();
    await page.waitForURL(/\/match\//, { timeout: 30_000 });
    const matchId = page.url().split('/match/')[1]!.split(/[?#]/)[0]!;
    await expect(page.getByTestId('group-t7')).toBeVisible({ timeout: 30_000 });

    let state = await board(request, matchId, me);
    let move: any;
    for (let i = 0; i < 30 && !move; i++) {
      move = state.legalActions.find(
        (o: any) =>
          o.enabled && o.verb === 'move' && o.target?.meldId?.startsWith('t') && o.source?.submit?.length,
      );
      if (move) break;
      const draw = state.legalActions.find((o: any) => (o.verb === 'draw' || o.verb === 'recycle') && o.enabled);
      if (!draw) break;
      await page.getByTestId('zone-press-stock').click();
      await page.waitForTimeout(400);
      state = await board(request, matchId, me);
    }
    test.skip(!move, 'this deal never offered a move onto a column');

    const head = move.source.submit[0] as string;
    const to = move.target.meldId as string;
    await touchDragLocatorTo(page, page.getByTestId(`lift-${head}`), page.getByTestId(`group-${to}`));
    await expect
      .poll(async () => {
        const s = await board(request, matchId, me);
        return s.view.zones.find((z: any) => z.id === 'tableau')?.groups?.find((g: any) => g.id === to)?.cards ?? [];
      })
      .toContain(head);
  });

  test('a Canasta card held and carried onto a meld joins that meld', async ({ page, request }) => {
    // A real hand has to be played until the partnership has melded and the
    // human holds a card that fits — see the mouse version in
    // drag-and-drop.spec.ts. Canasta is also where hand cards carry marks,
    // which is what put a second gesture on every card.
    test.setTimeout(150_000);
    const { matchId, host } = await tableWithBots(request, 'canasta', 4);
    await openMatch(page, host, matchId);
    await handCards(page);

    let offer: any = null;
    const deadline = Date.now() + 90_000;
    while (Date.now() < deadline && !offer) {
      const body = await board(request, matchId, host);
      offer =
        (body.legalActions ?? []).find(
          (o: any) => o.enabled && o.verb === 'lay_off' && o.target?.meldId && (o.source?.cards ?? []).length > 0,
        ) ?? null;
      if (offer) break;
      const live = page
        .locator('[data-testid^="offer-"]:not([data-testid$="-title"]):not([aria-disabled="true"])')
        .first();
      if (await live.count()) {
        try {
          await live.click({ timeout: 3000 });
        } catch {
          /* the board moved under us */
        }
      }
      await page.waitForTimeout(500);
    }
    test.skip(!offer, 'no lay-off came up before the deal moved on');

    const meldId = offer.target.meldId as string;
    const eligible = offer.source.cards as string[];
    const before = await serverHand(request, matchId, host);
    // A natural card. The offer's list is the pool to pick from, not a
    // promise that each card fits alone: a wild can be refused where it would
    // outnumber the naturals, which is the engine's call, not the gesture's.
    const wild = (c: string) => c.startsWith('2') || c.startsWith('JOKER');
    const index = before.findIndex((c) => eligible.includes(c) && !wild(c));
    test.skip(index < 0, 'only wild cards could be laid off');
    const dragged = before[index];

    // Pressing whatever was live may have opened a "why not" sheet over the
    // board, and a finger would land on that instead of the meld.
    const why = page.getByTestId('why-close');
    if (await why.isVisible()) await why.click();
    await expect(why).toBeHidden();

    // Both have to be on screen at once for one finger to span them.
    const meld = page.getByTestId(`group-${meldId}`);
    await meld.scrollIntoViewIfNeeded();
    await touchDragLocatorTo(page, card(page, index), meld);

    // By membership: a bot partner may lay onto the same meld meanwhile.
    await expect
      .poll(
        async () => {
          const body = await board(request, matchId, host);
          for (const z of body.view?.zones ?? []) for (const g of z.groups ?? []) if (g.id === meldId) return g.cards;
          return [];
        },
        { timeout: 10_000 },
      )
      .toContain(dragged);
  });
});
