import { expect, type Locator, type Page } from '@playwright/test';

/**
 * Pointer drags that react-native-gesture-handler actually recognizes.
 *
 * Both of the pauses below are load-bearing, and neither is documented
 * behaviour of gesture-handler's web pointer manager — they were found by
 * watching drags silently do nothing. This file is kept as the one place that
 * knowledge lives, so the next spec that needs a drag does not rediscover it.
 */

/** Drags the centre of one element onto the centre of another, and lets go. */
export async function dragLocatorTo(page: Page, from: Locator, to: Locator) {
  await carryLocatorOver(page, from, to);
  await release(page);
}

/**
 * Everything `dragLocatorTo` does except letting go: the pointer is left down,
 * over the centre of `to`, with the card still in flight. Returns that point.
 *
 * For the assertions that are only true *during* a drag — where the carried
 * card is drawn, and what it is drawn over — which a completed drag has
 * already thrown away by the time it can be looked at. Callers must `release`
 * after, or the next thing the page does inherits a stuck pointer.
 *
 * Both boxes are measured *after* scrolling the source into view, and in that
 * settled position: scrolling the target into view afterwards could move the
 * source again on a page too short to show both at once, which produces a drag
 * that starts from wherever the source used to be.
 */
export async function carryLocatorOver(page: Page, from: Locator, to: Locator) {
  await from.scrollIntoViewIfNeeded();
  const grab = await grabPoint(from);
  const toBox = await to.boundingBox();
  if (!toBox) throw new Error('drag target has no bounding box (not visible?)');

  const target = { x: toBox.x + toBox.width / 2, y: toBox.y + toBox.height / 2 };
  await carryPointOver(page, grab, target);
  return target;
}

/**
 * A point on this element that a person could actually put a finger on.
 *
 * Not its centre. The hand is held closed — the cards sit 49px apart and are
 * 151px wide — so the middle of a card is under the two or three cards drawn
 * over it, and a press there is a press on one of *those*. Aiming at the
 * centre meant a test that said "drag the first card" dragged the third, and
 * then asserted the hand had changed, which it had.
 *
 * It went unnoticed for the same reason those assertions could not see it:
 * until `handCards` was fixed to read the cards rather than their (empty)
 * rendered text, every one of those tests was comparing one list of empty
 * strings to another.
 *
 * So the point is found the way the browser finds it: walk in from the left
 * edge until whatever is on top at that point is this element or something
 * inside it. In a laid-out hand that is the first pixel tried, and this is the
 * centre-press it always was.
 */
export async function grabPoint(locator: Locator): Promise<{ x: number; y: number }> {
  const seen = await visiblePart(locator);
  return { x: seen.x + seen.width / 2, y: seen.y + seen.height / 2 };
}

/**
 * Taps a card where a person could tap it.
 *
 * `locator.click()` aims at the middle of an element and refuses to click when
 * something else is on top there — which in a closed hand is every card but
 * the last, so a tap on one timed out with "subtree intercepts pointer events"
 * after thirty seconds of retrying. The element was never going to become
 * clickable in the middle: that is what a fanned hand *is*.
 */
export async function tapCard(page: Page, locator: Locator) {
  await locator.scrollIntoViewIfNeeded();
  const { x, y } = await grabPoint(locator);
  await page.mouse.click(x, y);
}

/**
 * The part of an element that is actually on top — its own box, narrowed to
 * the run of it nothing else is drawn over.
 *
 * Found by asking the browser what is on top, column by column, rather than by
 * knowing which element covers which: that way it says the same true thing
 * about a fanned hand, a stacked meld and a card standing on its own.
 *
 * This is the same strip the app itself reasons about — `insertionAtPoint` in
 * `src/lib/hand.ts` puts the tipping point between two gaps halfway across
 * *what you can see* of a card, not halfway across its box. A test aiming at a
 * fraction of the box is therefore asking about somewhere the app has a
 * different name for, and somewhere no finger can land.
 */
export async function visiblePart(
  locator: Locator,
): Promise<{ x: number; y: number; width: number; height: number }> {
  const part = await locator.evaluate((el) => {
    const r = el.getBoundingClientRect();
    if (!r.width || !r.height) return null;
    const y = r.y + r.height / 2;
    const mine = (x: number) => {
      const top = document.elementFromPoint(x, y);
      return !!top && (top === el || el.contains(top));
    };

    let start: number | null = null;
    let end: number | null = null;
    for (let x = r.x + 2; x < r.x + r.width; x += 2) {
      if (mine(x)) {
        if (start === null) start = x;
        end = x;
      } else if (start !== null) {
        break;
      }
    }
    // Covered edge to edge, or hidden behind something the size of the whole
    // board. Hand back the box and let the assertion that follows say what
    // went wrong, rather than throwing from a helper.
    if (start === null || end === null) return { x: r.x, y: r.y, width: r.width, height: r.height };
    return { x: start, y: r.y, width: Math.max(1, end - start), height: r.height };
  });
  if (!part) throw new Error('element has no bounding box (not visible?)');
  return part;
}

export async function dragPointTo(
  page: Page,
  from: { x: number; y: number },
  to: { x: number; y: number },
) {
  await carryPointOver(page, from, to);
  await release(page);
}

/** Lets go of whatever `carry*` picked up, and waits for the board to settle. */
export async function release(page: Page) {
  await page.mouse.up();
  await page.waitForTimeout(400);
}

/** As `carryLocatorOver`, between two raw points. */
export async function carryPointOver(
  page: Page,
  from: { x: number; y: number },
  to: { x: number; y: number },
) {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();

  // The pause after mousedown, before any movement at all, is load-bearing:
  // without it the pan gesture never activates, with no error — the card just
  // silently snaps back on mouseup. Playwright fires the whole sequence in one
  // tick by default, and gesture-handler's RAF-driven tracking needs real,
  // spaced-out pointer events to see a gesture at all.
  //
  // It is also deliberately shorter than the hand's long-press threshold, so a
  // horizontal drag here exercises the same immediate sideways activation a
  // person gets rather than quietly falling through to the press-and-hold
  // path.
  await page.waitForTimeout(200);

  // Moving in two stages with interpolated steps, rather than jumping: an
  // abrupt jump reads as no movement at all and cancels the gesture.
  const midX = from.x + (to.x - from.x) / 2;
  const midY = from.y + (to.y - from.y) / 2;
  await page.mouse.move(midX, midY, { steps: 10 });
  await page.waitForTimeout(150);
  await page.mouse.move(to.x, to.y, { steps: 15 });
  await page.waitForTimeout(200);
}

/** The cards of the viewer's own hand, left to right, as they read on screen. */
export async function handCards(page: Page): Promise<string[]> {
  const cards = page.locator('[data-testid^="card-hand:"]');
  await expect(cards.first()).toBeVisible({ timeout: 20_000 });
  // Read off the card's accessibility label, which is the card code itself —
  // not its rendered text.
  //
  // This used to be `allTextContents()`, and under the drawn faces that was
  // the card. The engraved deck is SVG and has no text in it at all, so every
  // card started coming back as an empty string. A test asserting the hand had
  // *changed* then failed against a board that was fine; far worse, every test
  // asserting the hand had a particular order started passing by comparing one
  // list of empty strings to another. The label is what a screen reader is
  // given (`HandZone`'s `accessibilityLabel`), so it is drawn from the same
  // slot the card is, whatever the skin draws.
  return cards.evaluateAll((els) =>
    els.map((el) => {
      const labelled = el.closest('[aria-label]');
      const label = labelled?.getAttribute('aria-label')?.trim();
      return label || (el as HTMLElement).innerText.replace(/\s+/g, '');
    }),
  );
}
