import { expect, test, type Page } from '@playwright/test';

import { handCards, tapCard } from '../helpers/drag';
import { API_BASE, asViewer } from '../helpers/env';
import { clearHandSelection } from '../helpers/hand';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * Picking a particular card, when two of them look identical.
 *
 * Žolíky deals from at least two decks and Canasta from exactly two plus four
 * jokers, so a hand holding two cards with the same name is ordinary rather
 * than a corner case. The server has always known this — `canasta.removeCards`
 * takes "one copy per request, not all matching copies, because with two decks
 * a player can hold two identical cards and mean only one of them".
 *
 * The shell did not. It tracked selection as a list of card *strings*, so
 * tapping one of a pair lit up both, and tapping the second ran
 * `filter(c => c !== card)` and cleared the pair. There was no sequence of taps
 * that selected exactly one of two identical cards, and none that selected both
 * — which is a meld a player is entitled to lay.
 *
 * Nothing here knows a rule of Žolíky. It is used only because it deals a big
 * hand from a multi-deck shoe, which is what makes a pair likely.
 */

type Ctx = import('@playwright/test').APIRequestContext;

type Table = { matchId: string; host: any };

async function newTable(request: Ctx): Promise<Table> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `sel-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host };
}

async function handOf(request: Ctx, t: Table): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${t.matchId}`, asViewer(t.host));
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const zone = (body.view?.zones ?? []).find(
    (z: any) => z.kind === 'hand' && z.ownerId === t.host.userId,
  );
  return (zone?.cards ?? []).map((c: any) => c.card);
}

/**
 * Deals until the host is holding the same card twice.
 *
 * A shuffled deal cannot be asked for a pair, so this deals repeatedly over
 * the API — cheap, and nothing is rendered — and keeps the first hand that has
 * one. Roughly half of all hands do, so a run of fifteen without one is far
 * more likely to mean the deck stopped containing duplicates than that the
 * test was unlucky; either way it says so rather than quietly passing.
 */
async function tableHoldingAPair(request: Ctx, apart = false) {
  for (let attempt = 0; attempt < 30; attempt++) {
    const table = await newTable(request);
    const hand = await handOf(request, table);
    const pair = hand.find(
      (c, i) =>
        hand.indexOf(c) !== i &&
        (!apart || (!c.startsWith('JOKER') && i - hand.indexOf(c) > 1)),
    );
    if (pair) return { ...table, pair };
  }
  throw new Error('dealt 30 hands from a multi-deck shoe without a single duplicate card');
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: 'sel',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);

const isSelected = async (page: Page, i: number) =>
  (await card(page, i).getAttribute('aria-selected')) === 'true';

test.describe('choosing cards out of a hand', () => {
  test('tapping one of two identical cards selects that one and not its twin', async ({
    page,
    request,
  }) => {
    const table = await tableHoldingAPair(request);
    await openMatch(page, table.host, table.matchId);

    const shown = await handCards(page);
    // The pair as the *screen* has them, rather than trusting that the display
    // order matches the server's.
    const first = shown.findIndex((c, i) => shown.indexOf(c) !== i);
    const twin = shown.indexOf(shown[first]);
    expect(twin).toBeGreaterThanOrEqual(0);
    expect(twin).not.toBe(first);

    await tapCard(page, card(page, first));

    expect(await isSelected(page, first)).toBe(true);
    // The assertion the old code could not satisfy: the other copy of the same
    // card is untouched. Selecting by card string lit up both.
    expect(await isSelected(page, twin)).toBe(false);
  });

  test('both copies of a card can be selected at once, and released one at a time', async ({
    page,
    request,
  }) => {
    const table = await tableHoldingAPair(request);
    await openMatch(page, table.host, table.matchId);

    const shown = await handCards(page);
    const first = shown.findIndex((c, i) => shown.indexOf(c) !== i);
    const twin = shown.indexOf(shown[first]);

    await tapCard(page, card(page, first));
    await tapCard(page, card(page, twin));

    // A pair in one meld — a submission the string-keyed selection could not
    // express at all, because the second tap was read as undoing the first.
    expect(await isSelected(page, first)).toBe(true);
    expect(await isSelected(page, twin)).toBe(true);

    await tapCard(page, card(page, first));

    // And releasing one releases exactly one. The old `filter(c => c !== card)`
    // dropped every copy at once.
    expect(await isSelected(page, first)).toBe(false);
    expect(await isSelected(page, twin)).toBe(true);
  });

  test('discarding one of two identical cards takes that one out of the fan, not its twin', async ({
    page,
    request,
  }) => {
    // The push after a discard only says one 7♥ went, not which. The fan used
    // to drop the later copy whatever was picked, so discarding the left one
    // of a pair made the right one vanish. Only visible when something sits
    // between the two, hence a pair dealt apart.
    test.setTimeout(90_000);
    const table = await tableHoldingAPair(request, true);
    await openMatch(page, table.host, table.matchId);
    await handCards(page);

    await waitForOfferEnabled(page, 'offer-draw:deck');
    await page.getByTestId('offer-draw:deck').click();
    await expect.poll(async () => (await handCards(page)).length).toBeGreaterThan(0);
    await clearHandSelection(page);

    const shown = await handCards(page);
    const first = shown.indexOf(table.pair);
    const twin = shown.lastIndexOf(table.pair);
    expect(twin - first, `the ${table.pair} pair sits apart in ${shown.join(' ')}`).toBeGreaterThan(1);

    await tapCard(page, card(page, first));
    expect(await isSelected(page, first)).toBe(true);
    expect(await isSelected(page, twin)).toBe(false);
    const discard = page.getByTestId('offer-discard');
    await expect(discard).not.toHaveAttribute('aria-disabled', 'true');
    await discard.click();

    const expected = shown.filter((_, i) => i !== first);
    await expect.poll(() => handCards(page)).toEqual(expected);
  });

  test('a tap clearly away from the hand puts the whole selection down, a near miss does not', async ({
    page,
    request,
  }) => {
    // Unpicking four cards one tap at a time was the only way back to an empty
    // hand. A tap on the bare board now does it at once, as Escape does — but a
    // tap that lands in the hand between cards is a near miss, not a change of
    // mind, and keeps the selection.
    const table = await newTable(request);
    await openMatch(page, table.host, table.matchId);
    await handCards(page);

    await tapCard(page, card(page, 0));
    await tapCard(page, card(page, 2));
    expect(await isSelected(page, 0)).toBe(true);
    expect(await isSelected(page, 2)).toBe(true);

    // Inside the hand's panel, on its own empty margin: hit-tested, because a
    // point that turned out to be a card would toggle it and prove nothing.
    const hand = page.locator('[data-testid^="zone-hand:"]').first();
    const box = (await hand.boundingBox())!;
    const miss = { x: box.x + box.width - 6, y: box.y + box.height - 6 };
    const missHit = await page.evaluate(({ x, y }) => {
      const el = document.elementFromPoint(x, y);
      return el?.closest('[data-card],[role="button"],[role="checkbox"],button') ? 'control' : 'bare';
    }, miss);
    expect(missHit, 'the hand margin chosen is bare').toBe('bare');
    await page.mouse.click(miss.x, miss.y);
    expect(await isSelected(page, 0)).toBe(true);
    expect(await isSelected(page, 2)).toBe(true);

    // The game's name above the board: plainly not the hand.
    await page.getByTestId('match-module').click();
    await expect.poll(() => isSelected(page, 0)).toBe(false);
    expect(await isSelected(page, 2)).toBe(false);
  });

  test('tapping a card still selects and deselects it at all', async ({ page, request }) => {
    // The plain case, which nothing covered before: the shell's tap handling
    // was rewritten to address slots instead of strings, and a regression that
    // broke selection outright would otherwise only show up in a game.
    const table = await newTable(request);
    await openMatch(page, table.host, table.matchId);
    await handCards(page);

    expect(await isSelected(page, 0)).toBe(false);
    await tapCard(page, card(page, 0));
    expect(await isSelected(page, 0)).toBe(true);
    await tapCard(page, card(page, 0));
    expect(await isSelected(page, 0)).toBe(false);
  });
});
