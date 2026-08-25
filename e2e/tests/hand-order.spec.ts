import { expect, test, type Locator, type Page } from '@playwright/test';

import { dragLocatorTo, dragPointTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * Arranging the cards in your own hand.
 *
 * A player rearranging their hand is the oldest habit in card games and the
 * one thing the generic shell could not do: it drew the hand in whatever order
 * the module happened to keep it, and redrew it from scratch every time anyone
 * at the table moved.
 *
 * The interesting claim is not that a card can be dragged — it is that doing so
 * is *not a move*. Arrangement is a view preference, so it never reaches the
 * server, no module knows the feature exists, and every game gets it anyway.
 * The third test is the one that actually pins that down.
 *
 * Žolíky is used here only because it deals a large hand. Nothing below knows
 * a rule of it, and the same drags work in any of the four.
 */

type Ctx = import('@playwright/test').APIRequestContext;

/**
 * Deals a match against bots.
 *
 * `as` seats an existing player instead of a fresh guest, which is what lets a
 * test put the *same* person at two tables — the only way to ask whether one
 * table's arrangement leaks into the other's.
 */
async function tableWithBots(request: Ctx, moduleId: string, seats: number, as?: any) {
  let host = as;
  if (!host) {
    const res = await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `hand-${Math.random().toString(36).slice(2, 10)}` },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
    host = await res.json();
  }
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    expect((await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth })).ok())
      .toBeTruthy();
  }
  expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok())
    .toBeTruthy();

  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'hand',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** The viewer's hand as the *server* holds it — the order nobody rearranged. */
async function serverHand(request: Ctx, matchId: string, userId: string): Promise<string[]> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const hand = (body.view?.zones ?? []).find(
    (z: any) => z.kind === 'hand' && z.ownerId === userId,
  );
  return (hand?.cards ?? []).map((c: any) => c.card);
}

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);

/**
 * Where every box in the hand sits, except one being carried by the pointer.
 *
 * "Settled" means still part of the layout: a card that has been picked up is
 * positioned absolutely and follows the pointer, so it is excluded — what is
 * being described here is the shape of the row it left behind, which is what
 * has to stay put.
 */
async function settledBoxes(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    // A card's test id is on its ring, a few levels inside the box the layout
    // actually positions — so "is this one being carried?" has to be asked of
    // its ancestors, up to the row itself, rather than of the tagged element.
    const floating = (el: Element) => {
      let node: Element | null = el;
      while (node && !(node as HTMLElement).dataset?.testid?.startsWith('hand-')) {
        if (getComputedStyle(node as HTMLElement).position === 'absolute') return true;
        node = node.parentElement;
      }
      return false;
    };

    return Array.from(
      document.querySelectorAll('[data-testid^="card-hand:"], [data-testid="hand-drop-gap"]'),
    )
      .filter((el) => !floating(el))
      .map((el) => {
        const r = el.getBoundingClientRect();
        return `${Math.round(r.x)},${Math.round(r.y)},${Math.round(r.width)}`;
      })
      .sort();
  });
}

/**
 * The card currently being carried, if any — the one lifted out of the layout.
 *
 * Found by asking which card is positioned absolutely rather than by index,
 * because that is the definition of "the one in your hand right now".
 */
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
 * Drops a card into the gap on one side of another card.
 *
 * A card lands *between* two cards rather than on one, so which half of the
 * target the pointer is in decides which of its two gaps is meant. Aiming at
 * the dead centre — which is all `dragLocatorTo` can express — is the one
 * ambiguous spot, so a test that means "put this first" has to say so by
 * aiming at the left of the first card, the same way a person would.
 */
async function dragToEdge(page: Page, from: Locator, to: Locator, side: 'before' | 'after') {
  await from.scrollIntoViewIfNeeded();
  const a = await from.boundingBox();
  const b = await to.boundingBox();
  if (!a || !b) throw new Error('drag needs both boxes');

  await dragPointTo(
    page,
    { x: a.x + a.width / 2, y: a.y + a.height / 2 },
    { x: side === 'before' ? b.x + b.width * 0.15 : b.x + b.width * 0.85, y: b.y + b.height / 2 },
  );
}

test.describe('arranging your hand', () => {
  test('a card dragged along the hand lands where it was dropped, and takes nothing with it', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    expect(before.length).toBeGreaterThan(4);

    await dragLocatorTo(page, card(page, 0), card(page, 3));

    const after = await handCards(page);

    // The card that was first is now fourth, and everything it passed has
    // shifted down one. Asserting the whole row rather than just the moved
    // card is deliberate: the failure worth catching is a reorder that also
    // disturbs its neighbours.
    const expected = [before[1], before[2], before[3], before[0], ...before.slice(4)];
    expect(after).toEqual(expected);

    // Nothing gained, nothing lost, nothing duplicated. A slot model that
    // mints a new identity where it should have reused one shows up here as
    // a card appearing twice.
    expect([...after].sort()).toEqual([...before].sort());
  });

  test('the arrangement survives the state pushes that follow every move at the table', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    await dragToEdge(page, card(page, 5), card(page, 0), 'before');

    const arranged = await handCards(page);
    expect(arranged[0]).toBe(before[5]);

    // This is the whole reason arrangement had to be reconciled rather than
    // recomputed. The server re-pushes the entire board after every move by
    // anyone, in its own order; before this existed, each bot turn silently
    // reshuffled the player's hand back underneath them. Bots move on their
    // own, so waiting is all it takes to get several pushes.
    await page.waitForTimeout(4000);

    const later = await handCards(page);
    expect(later[0]).toBe(before[5]);
    // Compared as a subsequence, because a turn may legitimately have taken a
    // card out of the hand — what must hold is that the surviving cards are
    // still in the order they were put in, not that the hand is identical.
    expect(later.filter((c) => arranged.includes(c))).toEqual(
      arranged.filter((c) => later.includes(c)),
    );
  });

  test('a card can be dropped past the end of the fan, and lands last', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    const last = await card(page, before.length - 1).boundingBox();
    const first = await card(page, 0).boundingBox();
    if (!last || !first) throw new Error('no boxes');

    // Past the right-hand edge of the last card. There is one more gap than
    // there are cards, and this is the extra one — the position that simply
    // does not exist if a drop is thought of as landing *on* a card, which is
    // why the end of the fan used to be unreachable.
    await dragPointTo(
      page,
      { x: first.x + first.width / 2, y: first.y + first.height / 2 },
      { x: last.x + last.width + 16, y: last.y + last.height / 2 },
    );

    const after = await handCards(page);
    expect(after[after.length - 1]).toBe(before[0]);
    expect(after).toEqual([...before.slice(1), before[0]]);
  });

  test('a card put down where it was picked up stays where it was', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    const box = await card(page, 3).boundingBox();
    if (!box) throw new Error('no box');

    // Both gaps either side of a card mean "leave it alone", so a small wobble
    // that never really goes anywhere must not nudge it one place sideways.
    await dragPointTo(
      page,
      { x: box.x + box.width / 2, y: box.y + box.height / 2 },
      { x: box.x + box.width * 0.75, y: box.y + box.height / 2 },
    );

    expect(await handCards(page)).toEqual(before);
  });

  test('a card dropped in the empty space at the end of the hand lands last', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    const row = await page.getByTestId(`hand-hand:${host.userId}`).boundingBox();
    const first = await card(page, 0).boundingBox();
    if (!row || !first) throw new Error('no boxes');

    // The obvious way to say "put this at the end": drop it in the empty
    // space to the right of the last card. That space is a long way past the
    // last card — the row runs the width of the screen — and judging "is this
    // still the hand?" by where the *cards* are meant it fell outside, so the
    // card sprang back and the end of the fan could not be reached by hand,
    // however well the gaps themselves worked.
    await dragPointTo(
      page,
      { x: first.x + first.width / 2, y: first.y + first.height / 2 },
      { x: row.x + row.width - 8, y: row.y + row.height / 2 },
    );

    expect(await handCards(page)).toEqual([...before.slice(1), before[0]]);
  });

  test('the card being dragged goes where the pointer goes', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    const from = await card(page, 0).boundingBox();
    if (!from) throw new Error('no box');
    const grabX = from.x + from.width / 2;
    const grabY = from.y + from.height / 2;

    await page.mouse.move(grabX, grabY);
    await page.mouse.down();
    await page.waitForTimeout(200);

    try {
      // The plainest thing a drag has to do, and the one nothing here checked.
      // Every other test in this file watches the *gap*, so all of them passed
      // while the card sat motionless at the place it was picked up — the gap
      // tracking the pointer away from a card that never moved is what "they
      // diverge, and worse the further you go" actually was.
      for (const travelled of [120, 340, 620]) {
        await page.mouse.move(grabX + travelled, grabY, { steps: 10 });
        await page.waitForTimeout(200);

        const carried = await carriedBox(page);
        expect(carried, `nothing was lifted out after ${travelled}px`).toBeTruthy();
        const drift = Math.abs(carried!.x + carried!.width / 2 - (grabX + travelled));
        expect(drift, `card is ${Math.round(drift)}px from the pointer`).toBeLessThan(6);
      }
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(300);
    }
  });

  test('the row keeps its shape while a card is being dragged', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    const settledBefore = await settledBoxes(page);
    expect(settledBefore.length).toBeGreaterThan(4);

    const from = await card(page, 0).boundingBox();
    const over = await card(page, 4).boundingBox();
    if (!from || !over) throw new Error('no boxes');

    await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(200);
    await page.mouse.move(over.x, over.y + over.height / 2, { steps: 10 });
    await page.waitForTimeout(150);
    await page.mouse.move(over.x + over.width * 0.8, over.y + over.height / 2, { steps: 10 });
    await page.waitForTimeout(300);

    try {
      // The gap is open, so this is a real drag and not a no-op.
      await expect(page.getByTestId('hand-drop-gap')).toBeVisible();

      // The invariant the whole thing rests on: a picked-up card leaves the
      // layout and the gap stands in for it, so the row holds the same boxes
      // in the same places as before. The pointer is hit-tested against
      // positions measured once at pick-up, and this is what makes that
      // legitimate.
      //
      // Getting it wrong is not subtle once seen: leaving the dragged card in
      // the layout *and* inserting a gap made the row one card wider, shifted
      // everything right of the gap, and left the gap sitting a full card away
      // from the pointer.
      expect(await settledBoxes(page)).toEqual(settledBefore);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(300);
    }
  });

  test('the gap opens next to the pointer, however far the card is dragged', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    const shown = await handCards(page);

    // The positions the row's boxes occupy, before anything is picked up.
    // Every one of them stays put for the whole drag, so the gap must always
    // be sitting in one of them — and specifically in one of the two the
    // pointer is between.
    const columns = (await settledBoxes(page))
      .map((b) => Number(b.split(',')[0]))
      .sort((a, b) => a - b);

    const from = await card(page, 0).boundingBox();
    if (!from) throw new Error('no box');
    await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(200);

    try {
      // Checked at increasing distances, because a drift of one fixed card
      // width reads as "it is following me, badly" close up and as "it has
      // stopped following me" entirely by the far end of the fan.
      for (const index of [2, 4, shown.length - 1]) {
        const target = await card(page, index).boundingBox();
        if (!target) throw new Error('no box');
        const x = target.x + target.width * 0.8;
        await page.mouse.move(x, target.y + target.height / 2, { steps: 10 });
        await page.waitForTimeout(250);

        const gap = await page.getByTestId('hand-drop-gap').boundingBox();
        expect(gap, `no gap while over card ${index}`).toBeTruthy();

        // The column the pointer is in, and the one after it: dropping on the
        // right of a card means "after this one", so either is a truthful
        // answer. Anything further away is the gap having come adrift.
        const here = columns.findIndex((c, i) => x >= c && (i === columns.length - 1 || x < columns[i + 1]));
        const allowed = [columns[here], columns[Math.min(here + 1, columns.length - 1)]];
        expect(
          allowed,
          `gap at ${Math.round(gap!.x)} while pointing at ${Math.round(x)}, expected one of ${allowed}`,
        ).toContain(Math.round(gap!.x));
      }
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(300);
    }
  });

  test('the gap follows the card, not the pointer, when a card is picked up off-centre', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    const source = await card(page, 0).boundingBox();
    const column = await card(page, 5).boundingBox();
    if (!source || !column) throw new Error('no boxes');

    // Picked up near its left edge, which is the whole point: a card keeps
    // the grip it was taken by, so from here on the pointer is most of a card
    // to the left of the card itself. Every other test in this file grabs a
    // card dead centre, where the two coincide — which is exactly why they
    // all passed while the gap was visibly in the wrong place.
    const grabX = source.x + source.width * 0.15;
    const grabY = source.y + source.height / 2;

    // Carry it until the *card's centre* is just right of column 5's centre.
    const wanted = column.x + column.width * 0.6;
    const travelled = wanted - (source.x + source.width / 2);

    await page.mouse.move(grabX, grabY);
    await page.mouse.down();
    await page.waitForTimeout(200);
    await page.mouse.move(grabX + travelled / 2, grabY, { steps: 10 });
    await page.waitForTimeout(150);
    await page.mouse.move(grabX + travelled, grabY, { steps: 15 });
    await page.waitForTimeout(300);

    try {
      const gap = await page.getByTestId('hand-drop-gap').boundingBox();
      expect(gap).toBeTruthy();
      // Under the card. Following the pointer instead would put it a whole
      // column to the left, which is the divergence this exists to catch.
      expect(Math.round(gap!.x)).toBe(Math.round(column.x));
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(300);
    }
  });

  test('a card dragged from the end into the middle lands there', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    const last = before.length - 1;
    await dragToEdge(page, card(page, last), card(page, 3), 'before');

    const after = await handCards(page);
    const expected = [...before.slice(0, last)];
    expected.splice(3, 0, before[last]);
    expect(after).toEqual(expected);
  });

  test('an arrangement survives a reload', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const before = await handCards(page);
    await dragLocatorTo(page, card(page, 0), card(page, 4));
    const arranged = await handCards(page);
    expect(arranged).not.toEqual(before);

    // The whole point of arranging a hand is that it stays arranged. Held only
    // in the screen's memory, it lasted exactly as long as the screen did —
    // and coming back to a match to find the cards in the module's order again
    // is the same annoyance as never having arranged them.
    await page.reload();
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    await expect
      .poll(async () => (await handCards(page)).join(','), { timeout: 15_000 })
      .toBe(arranged.join(','));
  });

  test('an arrangement belongs to its own match and does not follow you', async ({
    page,
    request,
  }) => {
    const first = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, first.host, first.matchId);
    await dragLocatorTo(page, card(page, 0), card(page, 4));
    const arranged = await handCards(page);

    // A second table for the same player. Its hand is a different hand, so the
    // order recorded for the first has nothing to say about it — and a stored
    // arrangement applied across matches would be a rearrangement nobody asked
    // for, on cards that only coincidentally have the same names.
    const second = await tableWithBots(request, 'zolik', 2, first.host);
    await page.goto(`/match/${second.matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    const shown = await handCards(page);
    const served = await serverHand(request, second.matchId, first.host.userId);
    expect(shown).toHaveLength(served.length);
    expect(shown).not.toEqual(arranged);

    // Untouched means exactly the order the server dealt it in.
    expect(shown.length).toBe(served.length);

    // And the first table still remembers its own.
    await page.goto(`/match/${first.matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    await expect
      .poll(async () => (await handCards(page)).join(','), { timeout: 15_000 })
      .toBe(arranged.join(','));
  });

  test('rearranging is not a move: the server never hears about it', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    const serverBefore = await serverHand(request, matchId, host.userId);
    const shownBefore = await handCards(page);
    expect(serverBefore.length).toBe(shownBefore.length);

    await dragLocatorTo(page, card(page, 0), card(page, 2));

    const shownAfter = await handCards(page);
    expect(shownAfter).not.toEqual(shownBefore);

    // The point of the whole design: the drag changed what the player sees and
    // nothing else. If arrangement had been sent as an action — or worse, if
    // the shell had rebuilt a submission from screen positions — the server's
    // own copy would have moved too.
    const serverAfter = await serverHand(request, matchId, host.userId);
    expect(serverAfter).toEqual(serverBefore);
  });
});
