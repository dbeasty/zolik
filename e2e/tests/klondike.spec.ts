import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { dragLocatorTo } from '../helpers/drag';
import { API_BASE, asViewer } from '../helpers/env';
import { openGame } from '../helpers/lobby';
import { loginAsFreshGuest, seedIntroSeen, type GuestIdentity } from '../helpers/login';

/**
 * End-to-end for Solitaire (Klondike).
 *
 * The Go suites prove the rules and the secrecy in memory
 * (`klondike/engine_test.go`, `klondike/view_test.go`), and the contract test
 * holds the module to every hosted-game term. What only this can prove is the
 * part that is new to the stack rather than to the rules:
 *
 *  1. **One seat.** The lobby deals it with one button and no bot.
 *  2. **The deck is the opponent.** No face-down card ever reaches the client.
 *  3. **The cards are on the table.** A run is picked up off a column and
 *     dropped on another, the stock is tapped to draw — none of which any game
 *     before this needed, because every card a player moved was in their hand.
 *
 * Asserted on what the player sees, with the socket's state used only to find
 * a move worth making and to confirm it landed.
 */

type Selector = { zone: string; zoneId?: string; meldId?: string; submit?: string[] };
type Offer = { id: string; verb: string; enabled: boolean; source?: Selector; target?: Selector };
type Zone = {
  id: string;
  kind: string;
  count: number;
  cards?: { card: string }[];
  groups?: { id: string; cards: string[]; hidden?: number }[];
};
type MatchState = {
  status: string;
  winners?: string[];
  view: { zones: Zone[] };
  legalActions: Offer[];
};

const CARD = /^(10|[2-9TJQKA])[SHDC]$/;

async function stateOf(request: APIRequestContext, matchId: string, me: GuestIdentity): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(me));
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  return body.state ?? body;
}

/** Every card code anywhere in a board, wherever the server put it. */
function codesIn(value: unknown, out: string[] = []): string[] {
  if (typeof value === 'string') {
    if (CARD.test(value)) out.push(value);
  } else if (Array.isArray(value)) {
    value.forEach((v) => codesIn(v, out));
  } else if (value && typeof value === 'object') {
    Object.values(value).forEach((v) => codesIn(v, out));
  }
  return out;
}

async function dealSolitaire(page: Page, request: APIRequestContext) {
  const me = await loginAsFreshGuest(page, request, `sol-${Math.random().toString(36).slice(2, 8)}`);
  await openGame(page, 'klondike');
  // One seat: no table to open, no bots to play.
  await expect(page.getByTestId('play-friends-klondike')).toHaveCount(0);
  await expect(page.getByTestId('play-bots-klondike')).toHaveCount(0);
  await page.getByTestId('play-solo-klondike').click();
  await expect(page.getByTestId('setup-section-klondike-bots')).toHaveCount(0);
  await page.getByTestId('deal-me-in-klondike').click();
  await page.waitForURL(/\/match\//, { timeout: 30_000 });
  const matchId = page.url().split('/match/')[1]!.split(/[?#]/)[0]!;
  await expect(page.getByTestId('group-t7')).toBeVisible({ timeout: 30_000 });
  return { me, matchId };
}

test.describe('solitaire', () => {
  test('deals one seat, keeps the deck secret, and plays off the table', async ({ page, request }) => {
    const { me, matchId } = await dealSolitaire(page, request);

    // Seven columns, twenty-one cards face down under seven face up — drawn
    // as backs, and never sent.
    for (let c = 1; c <= 7; c++) {
      await expect(page.getByTestId(`group-t${c}`)).toBeVisible();
      await expect(page.locator(`[data-testid^="group-hidden-t${c}-"]`)).toHaveCount(c - 1);
    }
    let state = await stateOf(request, matchId, me);
    expect(state.view.zones.find((z) => z.id === 'stock')?.count).toBe(24);
    expect(codesIn(state.view.zones)).toHaveLength(7);

    // The stock is tapped, not hunted for in the controls.
    await page.getByTestId('zone-press-stock').click();
    await expect(page.getByTestId('zone-count-stock')).toHaveText('23');
    state = await stateOf(request, matchId, me);
    // Still nothing face down on the wire: seven columns' tops and one waste card.
    expect(codesIn(state.view.zones).length).toBeLessThanOrEqual(8);

    // Find a card on the table with somewhere to go, drawing until there is one.
    let move: Offer | undefined;
    for (let i = 0; i < 30 && !move; i++) {
      move = state.legalActions.find(
        (o) => o.enabled && o.verb === 'move' && o.target?.meldId?.startsWith('t') && o.source?.submit?.length,
      );
      if (move) break;
      const draw = state.legalActions.find((o) => o.verb === 'draw' && o.enabled);
      const recycle = state.legalActions.find((o) => o.verb === 'recycle' && o.enabled);
      if (!draw && !recycle) break;
      await page.getByTestId('zone-press-stock').click();
      await page.waitForTimeout(400);
      state = await stateOf(request, matchId, me);
    }
    test.skip(!move, 'this deal never offered a move onto a column');

    // Carried there by hand: the run's head, dropped on the column.
    const head = move!.source!.submit![0]!;
    const to = move!.target!.meldId!;
    await dragLocatorTo(page, page.getByTestId(`lift-${head}`), page.getByTestId(`group-${to}`));
    await expect
      .poll(async () => {
        const s = await stateOf(request, matchId, me);
        return s.view.zones.find((z) => z.id === 'tableau')?.groups?.find((g) => g.id === to)?.cards ?? [];
      })
      .toContain(head);
    // And the player sees it there.
    // `.first()`: a card that can move again is labelled by its lift control too.
    await expect(page.getByTestId(`group-${to}`).getByLabel(head, { exact: true }).first()).toBeVisible();

    // Giving up ends the game, lost and won by nobody.
    await page.getByTestId('offer-giveup').click();
    await expect.poll(async () => (await stateOf(request, matchId, me)).status).toBe('completed');
    expect((await stateOf(request, matchId, me)).winners ?? []).toEqual([]);
  });

  test('a finished game can be dealt again, card for card', async ({ page, request }) => {
    const { me, matchId } = await dealSolitaire(page, request);
    const tableau = (s: MatchState) => JSON.stringify(s.view.zones.find((z) => z.id === 'tableau'));
    const opening = tableau(await stateOf(request, matchId, me));

    await page.getByTestId('offer-giveup').click();
    await expect(page.getByTestId('match-over')).toBeVisible({ timeout: 15_000 });
    await page.getByTestId('match-over-deal-again').click();

    // A new table, dealt the same cards — and saying it is a deal seen before
    // once it is over.
    await page.waitForURL((url) => url.pathname.startsWith('/match/') && !url.pathname.endsWith(matchId), {
      timeout: 30_000,
    });
    const againId = page.url().split('/match/')[1]!.split(/[?#]/)[0]!;
    await expect(page.getByTestId('group-t7')).toBeVisible({ timeout: 30_000 });
    expect(tableau(await stateOf(request, againId, me))).toBe(opening);

    await page.getByTestId('offer-giveup').click();
    await expect(page.getByTestId('match-over-repeat')).toBeVisible({ timeout: 15_000 });
  });

  test('a finished deal can be sent to somebody else, who learns nothing of where it came from', async ({
    page,
    request,
    browser,
  }) => {
    const { me, matchId } = await dealSolitaire(page, request);
    const tableau = (s: MatchState) => JSON.stringify(s.view.zones.find((z) => z.id === 'tableau'));
    const opening = tableau(await stateOf(request, matchId, me));

    await page.getByTestId('offer-giveup').click();
    await expect(page.getByTestId('match-over')).toBeVisible({ timeout: 15_000 });
    await page.getByTestId('match-over-send-deal').click();
    const url = (await page.getByTestId('match-over-deal-url').textContent())!.trim();
    expect(url).toContain('/deal/');
    // The link names no match: not its id, not its join code.
    expect(url).not.toContain(matchId);

    // Somebody else, on a device of their own.
    const theirs = await browser.newContext({ reducedMotion: 'reduce' });
    const friendPage = await theirs.newPage();
    const friend = await loginAsFreshGuest(friendPage, request, `friend-${Math.random().toString(36).slice(2, 8)}`);
    // Opened on a device with nobody signed in, it says what it is, and
    // playing it goes through guest sign-in first.
    const stranger = await browser.newContext({ reducedMotion: 'reduce' });
    const strangerPage = await stranger.newPage();
    await seedIntroSeen(strangerPage);
    await strangerPage.goto(url);
    await expect(strangerPage.getByTestId('deal-game')).toBeVisible({ timeout: 30_000 });
    await strangerPage.screenshot({ path: 'test-results/klondike-deal-link.png' });
    await strangerPage.getByTestId('deal-play').click();
    await strangerPage.waitForURL(/\/auth\/guest/, { timeout: 30_000 });
    await stranger.close();

    await friendPage.goto(url);
    await expect(friendPage.getByTestId('deal-game')).toBeVisible({ timeout: 30_000 });
    await friendPage.getByTestId('deal-play').click();
    await friendPage.waitForURL(/\/match\//, { timeout: 30_000 });
    const friendMatch = friendPage.url().split('/match/')[1]!.split(/[?#]/)[0]!;
    expect(friendMatch).not.toBe(matchId);
    await expect(friendPage.getByTestId('group-t7')).toBeVisible({ timeout: 30_000 });

    // The same cards...
    const seen = await request.get(`${API_BASE}/matches/${friendMatch}`, asViewer(friend));
    const raw = await seen.text();
    expect(tableau(JSON.parse(raw).state ?? JSON.parse(raw))).toBe(opening);
    // ...and nothing on their board that leads back to the sender's game.
    expect(raw).not.toContain(matchId);

    // Finished, it is a first attempt for them, not a repeat.
    await friendPage.getByTestId('offer-giveup').click();
    await expect(friendPage.getByTestId('match-over')).toBeVisible({ timeout: 15_000 });
    await expect(friendPage.getByTestId('match-over-repeat')).toHaveCount(0);

    // And they see how the sender got on with the same cards, beside their own.
    await expect(friendPage.getByTestId('same-deal')).toBeVisible({ timeout: 15_000 });
    await expect(friendPage.locator('[data-testid^="same-deal-row-"]')).toHaveCount(2);
    await expect(friendPage.getByTestId('same-deal')).toContainText('(you)');
    await friendPage.getByTestId('same-deal').scrollIntoViewIfNeeded();
    await friendPage.waitForTimeout(4000); // past the results flash, for the picture
    await friendPage.screenshot({ path: 'test-results/klondike-same-deal.png' });
    await theirs.close();
  });

  test('a link that is not one says so', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, `stale-${Math.random().toString(36).slice(2, 8)}`);
    await page.goto('/deal/not-a-real-deal');
    await expect(page.getByTestId('deal-error')).toBeVisible({ timeout: 30_000 });
  });

  test('seven columns fit a phone without scrolling sideways', async ({ page, request }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await dealSolitaire(page, request);

    const width = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(width).toBeLessThanOrEqual(375);
    let lastRight = 0;
    for (let c = 1; c <= 7; c++) {
      const box = await page.getByTestId(`group-t${c}`).boundingBox();
      expect(box, `column ${c} is drawn`).not.toBeNull();
      // One row, left to right, inside the screen.
      expect(box!.x).toBeGreaterThanOrEqual(lastRight - 1);
      expect(box!.x + box!.width).toBeLessThanOrEqual(375);
      lastRight = box!.x + box!.width;
    }
    await page.screenshot({ path: 'test-results/klondike-375.png', fullPage: false });
  });
});
