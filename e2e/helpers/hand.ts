import { expect, type Locator, type Page } from '@playwright/test';

/**
 * Naming and picking the viewer's own cards.
 *
 * Two facts about the hand break the obvious approaches, and both cost a
 * green-but-meaningless test before this file existed:
 *
 * **The hand is not in the server's order.** A player may rearrange it, and
 * auto-arrange does so unasked, so `hand[i]` on the server and the i-th card
 * on screen are different cards. A spec that reads a card off the server and
 * then clicks by index clicks something else — and still passes its
 * `aria-selected` check, because *some* card got selected.
 *
 * **Something may already be selected.** A drawn card lands selected (that is
 * the point of it), and a tap *adds* to the selection rather than replacing
 * it, because that is how several cards are gathered into one meld. So a spec
 * that taps one card and expects exactly that card to be selected is asserting
 * something the app never promised.
 *
 * The fix for both is to work in the server's own vocabulary — the card code,
 * which the app publishes as each slot's accessibility label — and to say
 * explicitly when a clean selection is wanted.
 */

/** Every hand card on screen, by card code, in the order they are drawn. */
export async function handCodes(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll('[data-testid^="card-hand:"]')].map(
      (c) => c.closest('[aria-label]')?.getAttribute('aria-label') ?? '',
    ),
  );
}

/**
 * One hand card, named the way the server names it — "TD", "JOKER2" — rather
 * than by where it happens to sit or how it happens to read on screen ("10♦",
 * "JKR★").
 *
 * `.first()` because two decks are in play: a hand can hold the same code
 * twice, and either copy will do for anything that asks for "a card that
 * fits here".
 */
export function cardByCode(page: Page, code: string): Locator {
  return page.locator(`[aria-label="${code}"] [data-testid^="card-hand:"]`).first();
}

/** The codes of the cards currently selected. */
export async function selectedCodes(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll('[data-testid^="card-hand:"][aria-selected="true"]')].map(
      (c) => c.closest('[aria-label]')?.getAttribute('aria-label') ?? '',
    ),
  );
}

/**
 * Empties the selection, so what follows starts from nothing selected.
 *
 * Needed because a card arriving in hand lands selected. Any spec whose
 * subject is "what happens when *this* card is selected" has to say so, or it
 * is really testing "this card plus whatever was already picked".
 */
export async function clearHandSelection(page: Page) {
  const selected = page.locator('[data-testid^="card-hand:"][aria-selected="true"]');
  // Deselect one at a time, re-reading between clicks: each click re-renders
  // the fan, so a list of handles captured up front goes stale.
  for (let guard = 0; guard < 20; guard++) {
    if ((await selected.count()) === 0) return;
    await selected.first().click();
  }
  await expect(selected).toHaveCount(0);
}

/** Selects exactly `codes`, and nothing else. */
export async function selectOnly(page: Page, codes: string[]) {
  await clearHandSelection(page);
  for (const code of codes) await cardByCode(page, code).click();
  await expect
    .poll(async () => (await selectedCodes(page)).slice().sort())
    .toEqual(codes.slice().sort());
}
