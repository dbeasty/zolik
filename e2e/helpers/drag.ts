import { Locator, Page, expect } from '@playwright/test';

// react-native-gesture-handler's web pointer manager needs real, spaced-out
// pointer events to recognize a pan gesture — firing mousedown/mousemove/
// mouseup back-to-back in the same tick (Playwright's default) makes the
// drag never activate at all (the card silently snaps back, no error). A
// short pause after each step is enough for its RAF-driven tracking to
// register the gesture; the small settle pause after mouseup lets the
// resulting WS round-trip (drop -> server action -> broadcast -> re-render)
// land before the caller asserts on the result.
export async function dragLocatorTo(page: Page, from: Locator, to: Locator) {
  // Raw page.mouse coordinates don't auto-scroll like locator actions
  // (.click(), etc.) do — staging a card grows the staging box, which can
  // push the hand row below the fold, so without this a drag that worked
  // for the first card silently targets blank scrolled-out space for the
  // second. Scroll the source into view, then re-measure *both* boxes in
  // that settled scroll position (scrolling `to` into view next could
  // otherwise move `from` again if the page isn't tall enough to fit both
  // at once).
  await from.scrollIntoViewIfNeeded();
  const fromBox = await from.boundingBox();
  const toBox = await to.boundingBox();
  if (!fromBox) throw new Error('drag source has no bounding box (not visible?)');
  if (!toBox) throw new Error('drag target has no bounding box (not visible?)');
  await dragPointTo(
    page,
    { x: fromBox.x + fromBox.width / 2, y: fromBox.y + fromBox.height / 2 },
    { x: toBox.x + toBox.width / 2, y: toBox.y + toBox.height / 2 },
  );
}

export async function dragPointTo(
  page: Page,
  from: { x: number; y: number },
  to: { x: number; y: number },
) {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  // The pause after mousedown (before any movement at all) is load-bearing
  // — without it the pan gesture never activates at all, no error, the
  // card just silently snaps back on mouseup. Verified empirically; not
  // documented behavior of gesture-handler's web pointer manager.
  await page.waitForTimeout(200);
  const midX = from.x + (to.x - from.x) / 2;
  const midY = from.y + (to.y - from.y) / 2;
  await page.mouse.move(midX, midY, { steps: 10 });
  await page.waitForTimeout(150);
  await page.mouse.move(to.x, to.y, { steps: 15 });
  await page.waitForTimeout(200);
  await page.mouse.up();
  await page.waitForTimeout(400);
}

// A fast short upward flick — HandRow's QUICK_SWIPE_UP_DISTANCE/VELOCITY
// shortcut for discarding without aiming at the discard pile.
export async function flickUp(page: Page, from: Locator) {
  await from.scrollIntoViewIfNeeded();
  const box = await from.boundingBox();
  if (!box) throw new Error('flick source has no bounding box');
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  // Same load-bearing pause as dragPointTo (see its comment) so the pan
  // gesture activates at all — then one large, fast upward jump with no
  // settle pause before mouseup, so gesture-handler's velocity sample (taken
  // from the last couple of pointermove events) reads as fast/upward enough
  // to clear QUICK_SWIPE_UP_DISTANCE/VELOCITY.
  await page.waitForTimeout(200);
  await page.mouse.move(x, y - 200, { steps: 2 });
  await page.mouse.up();
  await page.waitForTimeout(400);
}

// Waits past the app's own WS connect/reconnect race (the very first
// connect attempt can lose to the session token not being bound to
// apiClient yet — the app's own backoff-retry already recovers from this in
// production; tests just need to wait it out instead of hitting a stale
// "Loading game…" screen).
export async function waitForGameLoaded(page: Page) {
  await expect(page.getByTestId('hand-card-0')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('offline')).not.toBeVisible();
}
