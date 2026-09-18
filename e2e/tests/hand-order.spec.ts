import { expect, test, type Locator, type Page } from '@playwright/test';

import { dragLocatorTo, dragPointTo, grabPoint, handCards, visiblePart } from '../helpers/drag';
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
 *
 * ## Tall enough to see the hand
 *
 * Every test here is about a gesture that starts in the hand, so the hand has
 * to be on screen for the whole of it. The board has grown — cards are drawn
 * as large as the row will take now — and on Playwright's default 720 the hand
 * had slipped below the fold: `boundingBox()` still returned a box, the
 * pointer still went down at its y, and nothing was drawn there, so the drags
 * quietly did nothing. Half this file failed for that reason and the failures
 * read as "hand-drop-gap not found" and "nothing was lifted out" — a suite
 * that could no longer see what it was about, rather than a claim that had
 * stopped being true.
 */
test.use({ viewport: { width: 1280, height: 1400 } });

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

/**
 * Where the layout put an element, in page coordinates, ignoring any transform
 * that is carrying it somewhere else.
 *
 * Installed into the page because both `settledBoxes` and `layoutLeft` need
 * it, and an `evaluate` callback cannot see anything in this file.
 *
 * `offsetLeft` alone will not do: it is measured from the nearest positioned
 * ancestor, and react-native-web makes every View `position: relative`, so a
 * card's ring reports a handful of pixels relative to the box two levels up.
 * The chain has to be walked and summed as far as the row.
 */
async function installLayoutProbe(page: Page) {
  await page.addInitScript(() => {
    // Is this box being carried by the pointer rather than laid out in the
    // row? A card's test id is on its ring, a few levels inside the box the
    // layout actually positions, so the question has to be asked of its
    // ancestors up to the row rather than of the tagged element.
    //
    // Installed here, with `laidOutAt`, because three separate helpers need
    // the same answer and an `evaluate` callback cannot see anything defined
    // in this file.
    (window as any).floating = (el: Element) => {
      let node: Element | null = el;
      while (node && !(node as HTMLElement).dataset?.testid?.startsWith('hand-')) {
        if (getComputedStyle(node as HTMLElement).position === 'absolute') return true;
        node = node.parentElement;
      }
      return false;
    };
    (window as any).laidOutAt = (el: HTMLElement) => {
      const row = el.closest('[data-testid^="hand-hand:"]') as HTMLElement | null;
      let x = 0;
      let y = 0;
      let node: HTMLElement | null = el;
      while (node && node !== row) {
        x += node.offsetLeft;
        y += node.offsetTop;
        node = node.offsetParent as HTMLElement | null;
      }
      const origin = (row ?? document.body).getBoundingClientRect();
      return { x: origin.x + x, y: origin.y + y };
    };
  });
}

async function openMatch(page: Page, host: any, matchId: string) {
  await installLayoutProbe(page);
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
    // Read from `offsetLeft`/`offsetTop`, which is where the layout put a box,
    // rather than from `getBoundingClientRect`, which is where it ends up
    // being drawn.
    //
    // The two now differ on purpose. The hand comes apart to show the drop
    // spot, and it does that with a transform: the cards either side of the
    // hole are *drawn* a card-width apart from where they are laid out, and
    // nothing they are measured against moves. That is the whole trick, and
    // measuring the drawn position here would call it a broken invariant when
    // it is the invariant being kept — the positions the pointer is hit-tested
    // against, read once at pick-up, are the laid-out ones.
    const floating = (window as any).floating;

    return Array.from(
      document.querySelectorAll('[data-testid^="card-hand:"], [data-testid="hand-drop-gap"]'),
    )
      .filter((el) => !floating(el))
      .map((el) => {
        const p = (window as any).laidOutAt(el as HTMLElement);
        return `${Math.round(p.x)},${Math.round(p.y)},${(el as HTMLElement).offsetWidth}`;
      })
      .sort();
  });
}

/**
 * Where the layout put one element, in page coordinates — see `settledBoxes`
 * for why that is not the same as where it is drawn.
 */
async function layoutLeft(page: Page, testId: string): Promise<number | null> {
  return page.evaluate((id) => {
    const el = document.querySelector(`[data-testid="${id}"]`) as HTMLElement | null;
    if (!el) return null;
    return Math.round((window as any).laidOutAt(el).x);
  }, testId);
}

/**
 * The card currently being carried, if any — the one lifted out of the layout.
 *
 * Found by asking which card is positioned absolutely rather than by index,
 * because that is the definition of "the one in your hand right now".
 */
async function carriedBox(page: Page) {
  return page.evaluate(() => {
    const floating = (window as any).floating;
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
 *
 * Both ends of the drag are expressed against the part of a card you can see
 * (`visiblePart`), not its box. In a closed hand those are very different
 * things: a card is 151px wide and sits 49px from the next, so a fraction of
 * the box lands on a card two places along, and "the left of the first card"
 * meant a point inside the third. That is also where the app puts the tipping
 * point between two gaps — halfway across the visible strip — so measuring
 * the same way is what makes "before" and "after" mean here what they mean
 * on screen.
 */
async function dragToEdge(page: Page, from: Locator, to: Locator, side: 'before' | 'after') {
  await from.scrollIntoViewIfNeeded();
  const a = await grabPoint(from);
  const b = await visiblePart(to);

  await dragPointTo(page, a, {
    x: side === 'before' ? b.x + b.width * 0.15 : b.x + b.width * 0.85,
    y: b.y + b.height / 2,
  });
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

    // "Put it down after the fourth card", said the way a person says it —
    // by aiming at the part of that card they can see. A closed hand has no
    // unambiguous middle to aim at; see `dragToEdge`.
    await dragToEdge(page, card(page, 0), card(page, 3), 'after');

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
    const first = await grabPoint(card(page, 0));
    if (!last) throw new Error('no boxes');

    // Past the right-hand edge of the last card. There is one more gap than
    // there are cards, and this is the extra one — the position that simply
    // does not exist if a drop is thought of as landing *on* a card, which is
    // why the end of the fan used to be unreachable.
    await dragPointTo(page, first, { x: last.x + last.width + 16, y: last.y + last.height / 2 });

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
    const first = await grabPoint(card(page, 0));
    if (!row) throw new Error('no boxes');

    // The obvious way to say "put this at the end": drop it in the empty
    // space to the right of the last card. That space is a long way past the
    // last card — the row runs the width of the screen — and judging "is this
    // still the hand?" by where the *cards* are meant it fell outside, so the
    // card sprang back and the end of the fan could not be reached by hand,
    // however well the gaps themselves worked.
    await dragPointTo(page, first, { x: row.x + row.width - 8, y: row.y + row.height / 2 });

    expect(await handCards(page)).toEqual([...before.slice(1), before[0]]);
  });

  test('the card being dragged goes where the pointer goes', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    await card(page, 0).scrollIntoViewIfNeeded();
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

  test('a card taken out of a closed hand is held clear of the row, and settles under the finger as it leaves it', async ({
    page,
    request,
  }) => {
    // The same claim as the test above, asked of the hand as it is actually
    // held: closed up, every card but the last covered by the one after it,
    // so the only part of a card you can take hold of is the strip down its
    // left edge.
    //
    // That is not an edge case — a dealt hand is closed at every width this
    // suite runs at (13 cards, 151px wide, sitting 49px apart at 1280). The
    // test above grabs card 0 in its dead centre, which is the one place in a
    // closed hand where a card's own middle and the strip you can see happen
    // to be the same column, and so it is the one grip that cannot tell the
    // two apart.
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    await card(page, 0).scrollIntoViewIfNeeded();
    const first = await card(page, 0).boundingBox();
    const second = await card(page, 1).boundingBox();
    if (!first || !second) throw new Error('no boxes');

    // The hand really is closed. Without this the test could pass on a board
    // that had quietly gone back to laying the fan out, which is the one
    // shape the bug it is here for never had.
    const pitch = second.x - first.x;
    expect(pitch, 'the fan is laid out, so this is not a closed hand').toBeLessThan(first.width);

    // A middle card, taken by the strip of it a player can see — the whole of
    // the gesture that was wrong. Everything to the right of this strip is
    // another card; aiming at the middle of card 6 would be pressing card 8.
    const held = await card(page, 6).boundingBox();
    if (!held) throw new Error('no box');
    const grabX = held.x + Math.min(12, pitch / 2);
    const grabY = held.y + held.height / 2;

    await page.mouse.move(grabX, grabY);
    await page.mouse.down();
    await page.waitForTimeout(200);

    try {
      // Carried slowly, and asked the same question the whole way up: a drag
      // that is right at the end and wrong in the middle is still a card you
      // cannot aim. The path goes up over the board as well as sideways,
      // because that is where a card being played is taken.
      const path = [
        { dx: 40, dy: -20 },
        { dx: 120, dy: -80 },
        { dx: 260, dy: -160 },
        { dx: 420, dy: -240 },
        { dx: 620, dy: -300 },
      ];

      let lift: number | null = null;
      const lifts: number[] = [];
      for (const { dx, dy } of path) {
        await page.mouse.move(grabX + dx, grabY + dy, { steps: 12 });
        await page.waitForTimeout(150);

        const carried = await carriedBox(page);
        expect(carried, `nothing was lifted out after ${dx}px`).toBeTruthy();

        const offX = carried!.x + carried!.width / 2 - (grabX + dx);
        const offY = carried!.y + carried!.height / 2 - (grabY + dy);
        expect(
          Math.abs(offX),
          `the finger is ${Math.round(offX)}px across from the middle of the card it is carrying`,
        ).toBeLessThan(8);
        lifts.push(offY);

        // Vertically it is not centred on the finger, and deliberately. The
        // hole a card is aimed at opens where the pointer is, so a card drawn
        // around the pointer is a card drawn over the answer to its own
        // question. In the hand it is held above the row instead, the way a
        // card pulled out of a hand is; carried up over the board, where
        // there is no hole to keep off and the card is the only thing saying
        // which meld you mean, it settles back under the finger.
        //
        // So the claim is that it only ever settles — never bobs back up,
        // which is what a lift switched on and off at a boundary would do.
        if (lift !== null) {
          expect(
            offY,
            `the card rose again — ${Math.round(offY)}px above the finger after ${Math.round(lift)}px`,
          ).toBeGreaterThanOrEqual(lift - 2);
        }
        lift = offY;
      }

      // Held well clear while the pointer was still in the hand...
      expect(lifts[0], 'the card was never held clear of the row').toBeLessThan(
        -held.height * 0.3,
      );

      // ...and back under the finger once it has reached the board. Polled
      // rather than read once: every other measurement here is of a card in
      // motion, where a frame's lag only ever reads as *more* lift than the
      // pointer has earned, but this one is about where the card comes to
      // rest — and under a full parallel run the last move's render can still
      // be a frame behind the mouse.
      const last = path[path.length - 1];
      await expect
        .poll(
          async () => {
            const at = await carriedBox(page);
            return at ? Math.abs(at.y + at.height / 2 - (grabY + last.dy)) : null;
          },
          {
            message: `the card is still being held above the finger out over the board (${lifts
              .map((v) => Math.round(v))
              .join(', ')})`,
          },
        )
        .toBeLessThan(8);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(300);
    }
  });

  /**
   * The hole the hand opens where a card would land, and the cards either
   * side of it — measured as they are drawn, not as they are laid out.
   */
  async function dropSpot(page: Page) {
    return page.evaluate(() => {
      const gap = document.querySelector('[data-testid="hand-drop-gap"]');
      if (!gap) return null;
      const g = gap.getBoundingClientRect();

      const floating = (window as any).floating;

      // Only the cards on the hole's own row. A hand long enough to wrap has
      // cards further left on the next line down, and sorting the lot by x
      // alone would hand back one of those as "the card after the hole".
      const settled = Array.from(document.querySelectorAll('[data-testid^="card-hand:"]'))
        .filter((el) => !floating(el))
        .map((el) => el.getBoundingClientRect())
        .filter((r) => Math.abs(r.y - g.y) < g.height / 2)
        .sort((a, b) => a.x - b.x);

      // The cards that actually shoulder the hole: the last one starting left
      // of it, and the first one starting right of it.
      const before = [...settled].reverse().find((r) => r.x < g.x) ?? null;
      const after = settled.find((r) => r.x > g.x) ?? null;
      return {
        gap: { x: g.x, width: g.width },
        // How much clear space the hole really has. With nothing after it,
        // the hole runs to the end of the row and is as wide as it looks.
        opening: after ? after.x - g.x : g.width,
        cardWidth: settled[0]?.width ?? 0,
      };
    });
  }

  test('the hand comes apart to show where the card will land', async ({ page, request }) => {
    // The drop spot was being drawn correctly and could not be seen. A closed
    // hand has no room in it: the gap standing in for the dragged card is a
    // whole card wide, but it sits on the same pitch as everything else, so
    // all of it but a thirty-pixel strip was covered by the card after it —
    // and that strip was covered again by the card you are carrying, which is
    // directly over it, because that is where your finger is.
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    const before = await handCards(page);

    await card(page, 0).scrollIntoViewIfNeeded();
    const first = await card(page, 0).boundingBox();
    const second = await card(page, 1).boundingBox();
    const target = await card(page, 8).boundingBox();
    if (!first || !second || !target) throw new Error('no boxes');

    const pitch = second.x - first.x;
    expect(pitch, 'the fan is laid out, so this is not a closed hand').toBeLessThan(first.width);

    // Card 1, taken by the strip of it you can see, carried along the fan —
    // not up over the board. This is the "where in my hand does it go?" case.
    const held = await card(page, 1).boundingBox();
    if (!held) throw new Error('no box');
    const grabX = held.x + Math.min(12, pitch / 2);
    const grabY = held.y + held.height / 2;
    const dropX = target.x + 10;

    await page.mouse.move(grabX, grabY);
    await page.mouse.down();
    await page.waitForTimeout(200);
    await page.mouse.move(grabX + 100, grabY, { steps: 10 });
    await page.waitForTimeout(150);
    await page.mouse.move(dropX, grabY, { steps: 20 });
    await page.waitForTimeout(300);

    let landed: string[];
    try {
      const spot = await dropSpot(page);
      expect(spot, 'no drop spot was drawn at all').toBeTruthy();

      // A hole a card could go in, rather than the strip of one. Judged
      // against a card's own width so it holds at any size the board draws
      // them at — this is the whole of the complaint.
      expect(
        spot!.opening,
        `the hole is ${Math.round(spot!.opening)}px of clear space, against a ${Math.round(spot!.cardWidth)}px card`,
      ).toBeGreaterThan(spot!.cardWidth * 0.9);

      // And it is under the finger, not two cards along from it.
      expect(dropX, 'the hole is not where the pointer is').toBeGreaterThanOrEqual(spot!.gap.x - 8);
      expect(dropX).toBeLessThanOrEqual(spot!.gap.x + spot!.gap.width + 8);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(400);
      landed = await handCards(page);
    }

    // The point of showing a hole at all: the card goes where the hole was.
    // Without this the test would be happy with a hole drawn somewhere
    // decorative, which is the bug it replaced wearing a bigger costume.
    const moved = before[1];
    expect(landed).not.toEqual(before);
    expect(landed.indexOf(moved), `${moved} did not land where the hole was`).toBeGreaterThan(1);
    expect([...landed].sort()).toEqual([...before].sort());
  });

  /**
   * How much of the hole is left in the clear by the card being carried into
   * it, asked of the browser rather than worked out from the numbers.
   *
   * What matters is whether a player can see the hole, and the only thing that
   * settles that is what is actually painted at a point — the same reason
   * `visiblePart` walks a card in from its left edge rather than trusting its
   * box.
   */
  async function holeInTheClear(page: Page) {
    return page.evaluate(() => {
      const gap = document.querySelector('[data-testid="hand-drop-gap"]');
      if (!gap) return null;
      const g = gap.getBoundingClientRect();
      const floating = (window as any).floating;
      const held = Array.from(document.querySelectorAll('[data-testid^="card-hand:"]')).find(
        (el) => floating(el),
      );
      if (!held) return null;

      // The strip of the hole below the card being carried: the card is held
      // above the row, so this is where the hole shows.
      const card = held.getBoundingClientRect();
      const strip = g.bottom - card.bottom;
      const y = Math.round(card.bottom + strip / 2);

      // Clear means clear of *any* card, not just the one being carried. The
      // cards in a fan overlap, so the card before the hole is a whole card
      // wide and the rest of it reaches on underneath — a space with the tail
      // of its neighbour lying in it is a picture of two cards in one place.
      let clear = 0;
      let sampled = 0;
      for (let x = Math.ceil(g.x) + 2; x < g.x + g.width - 2; x += 4) {
        const at = document.elementFromPoint(x, y);
        sampled += 1;
        if (!at || !at.closest('[data-testid^="card-hand:"]')) clear += 1;
      }
      return { strip: Math.round(strip), height: Math.round(g.height), clear, sampled };
    });
  }

  test('the hole you are aiming at is empty, and not hidden under the card you are carrying', async ({
    page,
    request,
  }) => {
    // The hand did come apart — a whole card of clear space, on every screen
    // with felt to spare — and it could still not be seen, because the hole
    // opens where the pointer is and the card was drawn around the pointer.
    // A card sitting in its own hole looks exactly like no hole at all, which
    // is what was actually being reported.
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    await card(page, 0).scrollIntoViewIfNeeded();
    const first = await card(page, 0).boundingBox();
    const second = await card(page, 1).boundingBox();
    const target = await card(page, 8).boundingBox();
    if (!first || !second || !target) throw new Error('no boxes');

    const pitch = second.x - first.x;
    expect(pitch, 'the fan is laid out, so this is not a closed hand').toBeLessThan(first.width);

    const held = await card(page, 1).boundingBox();
    if (!held) throw new Error('no box');
    const grabX = held.x + Math.min(12, pitch / 2);
    const grabY = held.y + held.height / 2;

    await page.mouse.move(grabX, grabY);
    await page.mouse.down();
    await page.waitForTimeout(200);
    await page.mouse.move(grabX + 100, grabY, { steps: 10 });
    await page.waitForTimeout(150);
    await page.mouse.move(target.x + 10, grabY, { steps: 20 });
    await page.waitForTimeout(300);

    try {
      const seen = await holeInTheClear(page);
      expect(seen, 'no hole, or nothing being carried into it').toBeTruthy();

      // Enough of the hole below the carried card to read as a card-shaped
      // hole — its bottom edge and both lower corners — rather than as a line
      // peeking out from under something.
      expect(
        seen!.strip,
        `only ${seen!.strip}px of a ${seen!.height}px hole is below the card being carried`,
      ).toBeGreaterThan(seen!.height * 0.4);

      // And the whole width of that strip really is empty — of the card being
      // carried, and of the neighbour whose far end reaches underneath it.
      expect(
        `${seen!.clear}/${seen!.sampled}`,
        'there is still a card painted inside the hole',
      ).toBe(`${seen!.sampled}/${seen!.sampled}`);
    } finally {
      await page.mouse.up();
      await page.waitForTimeout(400);
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
      //
      // "In the same places" is a claim about the layout, not about the paint
      // — the hand does come visibly apart around the hole, by transform. See
      // `settledBoxes`.
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

        // Which column the gap was *laid out* in. The hand slides apart around
        // it to show the hole, so where it is drawn is a card-width from where
        // it sits in the row — and it is the row this test is about.
        const gap = await layoutLeft(page, 'hand-drop-gap');
        expect(gap, `no gap while over card ${index}`).not.toBeNull();

        // The column the pointer is in, and the one after it: dropping on the
        // right of a card means "after this one", so either is a truthful
        // answer. Anything further away is the gap having come adrift.
        const here = columns.findIndex((c, i) => x >= c && (i === columns.length - 1 || x < columns[i + 1]));
        const allowed = [columns[here], columns[Math.min(here + 1, columns.length - 1)]];
        expect(
          allowed,
          `gap at ${gap} while pointing at ${Math.round(x)}, expected one of ${allowed}`,
        ).toContain(gap);
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
      //
      // Exact, and it stays exact because `splitFan` spends the empty felt at
      // the end of the row before it tightens anything: on a board with room,
      // the hole opens where the pointer put it and does not move. Spreading
      // the opening evenly along the fan instead would slide the hole up to
      // half a card away from the finger, and this is the assertion that
      // would say so.
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

  test('Auto-arrange tidies the hand without moving anything to the server', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);

    // Scramble it first, so a no-op auto-arrange (the hand already happened
    // to be tidy) can't pass this test by accident.
    await dragLocatorTo(page, card(page, 0), card(page, 3));
    const scrambled = await handCards(page);
    const serverBefore = await serverHand(request, matchId, host.userId);

    await page.getByTestId(`hand-auto-arrange-hand:${host.userId}`).click();

    const tidied = await handCards(page);
    expect(tidied).not.toEqual(scrambled);
    // Same cards, just reordered — a tidy that lost or duplicated one would
    // be a far worse bug than a merely unwelcome arrangement.
    expect([...tidied].sort()).toEqual([...scrambled].sort());

    // A view preference, exactly like a manual drag: the server's own copy of
    // the hand never moves.
    const serverAfter = await serverHand(request, matchId, host.userId);
    expect(serverAfter).toEqual(serverBefore);

    // And it is remembered the same way a manual arrangement is.
    await page.reload();
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    await expect
      .poll(async () => (await handCards(page)).join(','), { timeout: 15_000 })
      .toBe(tidied.join(','));
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

  /**
   * The same claim, on a row with no felt to spare.
   *
   * `scaleFor` solves the card size so that a full hand closed up *exactly*
   * fills its row, so for a band of widths above the desktop breakpoint there
   * is no empty space at either end to open a hole with — and, until
   * `splitFan`, no hole. 800 is inside that band and clear of the widths where
   * a thirteen-card hand wraps. Everything here has to come out of the fan
   * closing up on itself, which is the half of the behaviour 1280 cannot see.
   */
  test.describe('on a row the hand fills to both ends', () => {
    test.use({ viewport: { width: 800, height: 1400 } });

    test('the hand still comes apart by a whole card', async ({ page, request }) => {
      const { matchId, host } = await tableWithBots(request, 'zolik', 2);
      await openMatch(page, host, matchId);
      await handCards(page);

      await card(page, 0).scrollIntoViewIfNeeded();
      const first = await card(page, 0).boundingBox();
      const second = await card(page, 1).boundingBox();
      const target = await card(page, 5).boundingBox();
      if (!first || !second || !target) throw new Error('no boxes');

      const pitch = second.x - first.x;
      expect(pitch, 'the fan is laid out, so this is not a closed hand').toBeLessThan(first.width);

      const held = await card(page, 1).boundingBox();
      if (!held) throw new Error('no box');
      const grabX = held.x + Math.min(12, pitch / 2);
      const grabY = held.y + held.height / 2;

      await page.mouse.move(grabX, grabY);
      await page.mouse.down();
      await page.waitForTimeout(200);
      await page.mouse.move(grabX + 60, grabY, { steps: 10 });
      await page.waitForTimeout(150);
      await page.mouse.move(target.x + 10, grabY, { steps: 20 });
      await page.waitForTimeout(300);

      try {
        const spot = await dropSpot(page);
        expect(spot, 'no drop spot was drawn at all').toBeTruthy();
        expect(
          spot!.opening,
          `the hole is ${Math.round(spot!.opening)}px of clear space, against a ${Math.round(spot!.cardWidth)}px card`,
        ).toBeGreaterThan(spot!.cardWidth * 0.9);
      } finally {
        await page.mouse.up();
        await page.waitForTimeout(400);
      }
    });
  });
});
