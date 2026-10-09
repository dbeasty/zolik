import { expect, test, type Locator, type Page } from '@playwright/test';

import { openGame, playAgainstBots } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * The board, played the way a blind player or a keyboard-only player plays it
 * (docs/accessibility-plan.md, Phases 2-4).
 *
 * Getting to a table goes through the lobby the usual way; everything after
 * that — reading the board, picking a card, playing it, drawing, asking why a
 * move is off, hearing what the bots did — is done here with nothing but
 * roles, accessible names and the keyboard. No coordinates and no test ids
 * for anything under test: if a step here cannot be found by what a screen
 * reader would call it, a screen-reader user cannot find it either.
 *
 * What is heard is read off the two live regions `announce()` writes to.
 */

/** A card's spoken name: "Seven of Clubs", "Joker", "Red 7", "Wild Draw Four". */
const SPOKEN = /^(Joker|Wild( Draw Four)?|(Ace|Two|Three|Four|Five|Six|Seven|Eight|Nine|Ten|Jack|Queen|King|Under|Over) of \w+|\w+ (\d+|Skip|Reverse|Draw Two))\b/;

const polite = (page: Page) => page.getByTestId('a11y-live-polite');
const assertive = (page: Page) => page.getByTestId('a11y-live-assertive');

async function sitDown(page: Page, request: import('@playwright/test').APIRequestContext, game: string) {
  await loginAsFreshGuest(page, request, `sr-${game}-${Math.random().toString(36).slice(2, 6)}`);
  await openGame(page, game);
  if (game === 'klondike') {
    await page.getByTestId('play-solo-klondike').click();
    await page.getByTestId('deal-me-in-klondike').click();
  } else {
    await playAgainstBots(page, game);
  }
  await expect(page).toHaveURL(/\/match\//, { timeout: 30_000 });
  await expect(page.getByRole('region', { name: 'Actions' })).toBeVisible({ timeout: 30_000 });
}

const hand = (page: Page) => page.getByRole('listbox', { name: /^Your hand, \d+ cards?$/ });

async function handSize(page: Page): Promise<number> {
  const name = (await hand(page).getAttribute('aria-label')) ?? '';
  return Number(/(\d+) cards?$/.exec(name)?.[1] ?? NaN);
}

/** The accessible name of whatever has keyboard focus. */
async function focusedName(page: Page): Promise<string> {
  return page.evaluate(() => document.activeElement?.getAttribute('aria-label') ?? '');
}

async function focusedRole(page: Page): Promise<string> {
  return page.evaluate(() => document.activeElement?.getAttribute('role') ?? document.activeElement?.tagName ?? '');
}

/** Tabs from the top of the page until a card in hand has focus: one stop for the whole hand. */
async function tabIntoHand(page: Page) {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  for (let i = 0; i < 80; i++) {
    await page.keyboard.press('Tab');
    if ((await focusedRole(page)) === 'option') return;
  }
  throw new Error('Tab never reached the hand');
}

/** An offer's button, by the name a screen reader says. */
const offer = (page: Page, name: string | RegExp) =>
  page.getByRole('region', { name: 'Actions' }).getByRole('button', { name, exact: typeof name === 'string' });

async function isLive(button: Locator): Promise<boolean> {
  return (await button.count()) > 0 && (await button.first().getAttribute('aria-disabled')) !== 'true';
}

/**
 * Waits until the table is waiting on this player — read where a screen
 * reader reads it, off the player's own seat: "Dana (you), …, your turn".
 */
async function waitForMyTurn(page: Page, timeout = 45_000) {
  await expect(page.getByRole('group', { name: /\(you\).*, your turn$/ })).toBeVisible({ timeout });
}

/** Presses the first answer in whatever sheet a move opened (where to play, which colour). */
async function answerAnySheet(page: Page) {
  const dialog = page.getByRole('dialog');
  // A sheet opens a moment after the key that asked for it.
  await dialog.first().waitFor({ state: 'visible', timeout: 1_500 }).catch(() => undefined);
  if (!(await dialog.count())) return;
  const first = dialog.getByRole('button').first();
  await first.focus();
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0, { timeout: 5_000 });
}

test.describe('screen-reader path', () => {
  test.setTimeout(180_000);

  test('Žolíky: hear the turn, draw with D, pick and play a card from the keyboard, hear the bot', async ({ page, request }) => {
    await sitDown(page, request, 'zolik');

    // The board is named regions, in the order the eye meets them.
    for (const region of ['Players', 'Table', 'Recent moves', 'Your hand', 'Actions']) {
      await expect(page.getByRole('region', { name: region })).toBeVisible();
    }

    // Every seat is one sentence; the bot says it is one.
    await expect(page.getByRole('group', { name: /bot \(/ }).first()).toBeVisible();
    // The pile says what is on it, in words.
    await expect(page.getByRole('group', { name: /^Discard pile, (top card .+|empty)/ })).toBeVisible();

    // The first deal is the player's turn: said, and live.
    await waitForMyTurn(page);
    await expect(assertive(page)).toContainText('Your turn', { timeout: 10_000 });

    // Cards are named, never coded.
    const options = hand(page).getByRole('option');
    const first = await options.first().getAttribute('aria-label');
    expect(first).toMatch(SPOKEN);
    expect(first).toMatch(/, 1 of \d+/);

    // A disabled move is still reachable, and says why it is off.
    const meld = offer(page, 'Meld');
    await expect(meld).toHaveAttribute('aria-disabled', 'true');
    await expect(meld).toHaveAccessibleDescription(/\w+/);
    // Its tooltip — the same reason — shows on keyboard focus and closes on
    // Escape. (The bubble is hidden from the accessibility tree: what it says
    // is already the button's description, so it is found by its id.)
    await meld.focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    await expect(page.getByTestId('tooltip')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('tooltip')).toHaveCount(0);
    // Enter on it opens the explanation, as a press would.
    await page.keyboard.press('Enter');
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toHaveCount(0);

    // D draws from the face-down pile.
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    const before = await handSize(page);
    await page.keyboard.press('d');
    await expect.poll(() => handSize(page), { timeout: 10_000 }).toBe(before + 1);

    // One Tab stop for the hand; arrows walk it; Space picks.
    // (The stop is the card just drawn — it arrives picked — so Home first.)
    await tabIntoHand(page);
    expect(await focusedName(page)).toMatch(/just drawn/);
    await page.keyboard.press('Home');
    expect(await focusedName(page)).toMatch(SPOKEN);
    const firstName = await focusedName(page);
    await page.keyboard.press('ArrowRight');
    const second = await focusedName(page);
    expect(second).not.toBe(firstName);
    expect(second).toMatch(/, 2 of \d+/);
    await page.keyboard.press(' ');
    await expect(page.locator('[role="option"]:focus')).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press(' ');
    await expect(page.locator('[role="option"]:focus')).toHaveAttribute('aria-selected', 'false');

    // Escape lets go of a selection — after closing the card's tooltip, if
    // one is open: the first Escape is the tooltip's.
    await page.keyboard.press(' ');
    await expect(hand(page).getByRole('option', { selected: true })).not.toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.keyboard.press('Escape');
    await expect(hand(page).getByRole('option', { selected: true })).toHaveCount(0);

    // Shift+arrow carries the card along the hand, and focus goes with it.
    const card = second.replace(/, \d+ of \d+.*$/, '');
    await page.keyboard.press('Shift+ArrowRight');
    await expect.poll(() => focusedName(page)).toMatch(new RegExp(`^${card}, 3 of \\d+`));

    // Enter plays a card: straight there when it has one place to go, a sheet
    // asking where when it has more. A joker may not be thrown away, so walk
    // to a card that is not one.
    for (let i = 0; i < 14 && /Joker/.test(await focusedName(page)); i++) await page.keyboard.press('ArrowRight');
    const size = await handSize(page);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog');
    if (await dialog.isVisible().catch(() => false)) {
      const toPile = dialog.getByRole('button', { name: /Discard pile/ });
      await toPile.focus();
      await page.keyboard.press('Enter');
    }
    await expect.poll(() => handSize(page), { timeout: 10_000 }).toBe(size - 1);
    // Focus is not dropped to the top of the page with the card that left:
    // it is on the card that took its place.
    await expect.poll(() => focusedRole(page)).toBe('option');

    // The bot's move is said, in a sentence with its name in it.
    const botName = ((await page.getByRole('group', { name: /bot \(/ }).first().getAttribute('aria-label')) ?? '').split(',')[0]!;
    await expect(polite(page)).toContainText(botName, { timeout: 45_000 });

    // The moves are a list to go back over.
    await waitForMyTurn(page);
    await expect(page.getByRole('region', { name: 'Recent moves' }).getByRole('listitem').first()).toBeVisible();

    // R reads the whole table; ? lists the keys.
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('r');
    await expect(assertive(page)).toContainText(/Your turn|Scores/, { timeout: 5_000 });
    await page.keyboard.press('?');
    await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toHaveCount(0);

    // M takes focus to the moves.
    await page.keyboard.press('m');
    expect(await focusedName(page)).toBe('Recent moves');
  });

  test('Last Card: play what fits, or draw', async ({ page, request }) => {
    await sitDown(page, request, 'lastcard');

    let played = 0;
    for (let turn = 0; turn < 4; turn++) {
      await waitForMyTurn(page);
      const options = hand(page).getByRole('option');
      expect(await options.first().getAttribute('aria-label')).toMatch(SPOKEN);
      const size = await handSize(page);

      // Which cards can go somewhere is in each card's description.
      let fitting = -1;
      const n = await options.count();
      for (let i = 0; i < n; i++) {
        const described = await options.nth(i).evaluate((el) =>
          (el.getAttribute('aria-describedby') ?? '')
            .split(' ')
            .map((id) => document.getElementById(id)?.textContent ?? '')
            .join(' '),
        );
        if (/Can go to/.test(described)) {
          fitting = i;
          break;
        }
      }

      if (fitting >= 0) {
        await tabIntoHand(page);
        await page.keyboard.press('Home');
        for (let i = 0; i < fitting; i++) await page.keyboard.press('ArrowRight');
        await page.keyboard.press('Enter');
        // Where to (more than one place), then which colour (a wild).
        await answerAnySheet(page);
        await answerAnySheet(page);
        await expect.poll(() => handSize(page), { timeout: 10_000 }).toBeLessThan(size);
        played++;
      } else {
        await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
        await page.keyboard.press('d');
        await expect.poll(() => handSize(page), { timeout: 10_000 }).not.toBe(size);
        // A drawn card that can be played may leave the turn open; pass it on.
        const pass = offer(page, /^(Pass|Keep|End turn)/);
        if (await isLive(pass)) {
          await pass.first().focus();
          await page.keyboard.press('Enter');
        }
      }
      if ((await page.getByRole('heading', { name: /over/i }).count()) > 0) break;
    }
    // Somebody else moved, and was heard.
    await expect(polite(page)).not.toBeEmpty({ timeout: 30_000 });
    test.info().annotations.push({ type: 'played', description: String(played) });
  });

  test("Hold'em: the betting controls are named buttons and a keyboard slider", async ({ page, request }) => {
    await sitDown(page, request, 'holdem');
    await waitForMyTurn(page);
    await expect(assertive(page)).toContainText('Your turn', { timeout: 10_000 });

    // The hole cards are spoken; the board's are counted until they are dealt.
    expect(await hand(page).getByRole('option').first().getAttribute('aria-label')).toMatch(SPOKEN);

    // A raise amount is a slider the arrows move.
    const slider = page.getByRole('region', { name: 'Actions' }).getByRole('slider');
    if (await slider.count()) {
      const was = Number(await slider.first().getAttribute('aria-valuenow'));
      await slider.first().focus();
      await page.keyboard.press('End');
      await expect.poll(async () => Number(await slider.first().getAttribute('aria-valuenow'))).toBeGreaterThanOrEqual(was);
      await page.keyboard.press('Home');
    }

    // Check or call from the keyboard; the turn passes. A bot may have folded
    // the hand already, and then the one thing on offer is the next deal —
    // pressed from the keyboard too, and the betting tried again.
    const go = offer(page, /^(Check|Call|Fold)/);
    let called = false;
    for (let tries = 0; tries < 6 && !called; tries++) {
      await waitForMyTurn(page);
      if (await isLive(go)) {
        // Check or call, before fold.
        const seat = page.getByRole('group', { name: /\(you\)/ });
        const before = await seat.getAttribute('aria-label');
        const pick = (await isLive(offer(page, /^(Check|Call)/))) ? offer(page, /^(Check|Call)/) : go;
        await pick.and(page.locator(':not([aria-disabled="true"])')).first().focus();
        await page.keyboard.press('Enter');
        // The seat's own sentence moves on: the stack, the bet, whose turn.
        await expect.poll(() => seat.getAttribute('aria-label'), { timeout: 15_000 }).not.toBe(before);
        called = true;
        break;
      }
      const next = offer(page, /^(Start the next round|Continue|Next)/);
      if (await isLive(next)) {
        await next.first().focus();
        await page.keyboard.press('Enter');
      }
      await page.waitForTimeout(1000);
    }
    expect(called, 'a check or a call came up').toBe(true);

    // R speaks the table.
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('r');
    await expect(assertive(page)).not.toBeEmpty();
  });

  test('Klondike: turn the stock with D, and lift a card by its name', async ({ page, request }) => {
    await sitDown(page, request, 'klondike');
    await waitForMyTurn(page);

    const table = page.getByRole('region', { name: 'Table' });
    // The columns are named groups: their kind, their cards, how many are face down.
    await expect(table.getByRole('group', { name: /^Column: .+face down/ }).first()).toBeVisible();

    // D turns the stock over onto the waste.
    const waste = () =>
      table
        .getByRole('group', { name: /top card|empty/ })
        .evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')).join(' | '));
    const was = await waste();
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('d');
    await expect.poll(waste, { timeout: 10_000 }).not.toBe(was);

    // A card that can be moved is a button named by the card; Enter moves it
    // (or picks it up, when it has more than one place to go).
    const names = await table
      .getByRole('button')
      .evaluateAll((els) => els.map((e) => e.getAttribute('aria-label') ?? '').filter((n) => /^(Ace|Two|Three|Four|Five|Six|Seven|Eight|Nine|Ten|Jack|Queen|King) of /.test(n)));
    if (names.length) {
      const button = table.getByRole('button', { name: names[0]!, exact: true }).first();
      const snapshot = await table.ariaSnapshot();
      await button.focus();
      await page.keyboard.press('Enter');
      await expect.poll(() => table.ariaSnapshot(), { timeout: 10_000 }).not.toBe(snapshot);
    }

    // R reads the table.
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('r');
    await expect(assertive(page)).toContainText(/Stock|Waste/);
  });
});
