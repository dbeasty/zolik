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
  const fromBox = await from.boundingBox();
  const toBox = await to.boundingBox();
  if (!fromBox) throw new Error('drag source has no bounding box (not visible?)');
  if (!toBox) throw new Error('drag target has no bounding box (not visible?)');

  const target = { x: toBox.x + toBox.width / 2, y: toBox.y + toBox.height / 2 };
  await carryPointOver(
    page,
    { x: fromBox.x + fromBox.width / 2, y: fromBox.y + fromBox.height / 2 },
    target,
  );
  return target;
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
  return (await cards.allTextContents()).map((t) => t.replace(/\s+/g, ''));
}
