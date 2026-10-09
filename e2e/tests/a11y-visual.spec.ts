import { expect, test, type Page } from '@playwright/test';

import { playAgainstBots, openGame } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * The visual accessibility settings (docs/accessibility-plan.md, Phase 5),
 * asserted on what a player sees at a live table rather than on the
 * preference being stored:
 *
 *  - the four-colour deck paints a diamond blue and a club green;
 *  - the high-contrast board goes on when asked for in Settings, and when
 *    left on Automatic on a browser that asks for more contrast or forces
 *    its own colours — and stays off otherwise;
 *  - Large and Extra large cards are measurably larger cards;
 *  - "Reduce motion: On" stills the board on a browser that has *not* asked
 *    for less motion: no card is ever put in the air.
 *
 * Preferences are seeded the way the app stores them — one JSON blob under
 * `zolik_a11y_prefs` — before the first page loads, so the very first board
 * is drawn with them.
 */

type Prefs = Partial<{
  fourColour: boolean;
  contrast: 'auto' | 'on' | 'off';
  cardSize: 'normal' | 'large' | 'xlarge';
  motion: 'auto' | 'on' | 'off';
}>;

async function seed(page: Page, prefs: Prefs, skin?: string) {
  await page.addInitScript(
    ({ prefs, skin }) => {
      // Once per tab: a later navigation must see what the player changed
      // since, not the seed again.
      if (window.sessionStorage.getItem('a11y-visual-seeded')) return;
      window.sessionStorage.setItem('a11y-visual-seeded', '1');
      window.localStorage.setItem('zolik_a11y_prefs', JSON.stringify(prefs));
      if (skin) window.localStorage.setItem('zolik_skin', skin);
    },
    { prefs, skin },
  );
}

async function dealZolik(page: Page) {
  await openGame(page, 'zolik');
  await playAgainstBots(page, 'zolik');
  await expect(page.locator('[data-testid^="card-hand:"]').first()).toBeVisible({ timeout: 30_000 });
}

const fresh = () => `vis-${Math.random().toString(36).slice(2, 8)}`;

/** The colour the first line of type on each hand card is printed in, by card code. */
async function inkByCode(page: Page): Promise<Record<string, string>> {
  return page.evaluate(() => {
    const out: Record<string, string> = {};
    for (const card of document.querySelectorAll('[data-testid^="card-hand:"]')) {
      const code = card.closest('[aria-label]')?.getAttribute('aria-label') ?? '';
      const text = card.querySelector('div[dir="auto"]');
      if (code && text) out[code] = getComputedStyle(text).color;
    }
    return out;
  });
}

const suitOf = (code: string) => (code.startsWith('JOKER') ? '' : code[code.length - 1]!);

test.describe('four-colour deck', () => {
  test('diamonds are blue and clubs green; hearts stay red and spades black', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, fresh());
    // Classic: the plain face, whose index is type — its colour is the card's.
    await seed(page, { fourColour: true }, 'classic');
    await dealZolik(page);

    const inks = await inkByCode(page);
    const bySuit: Record<string, Set<string>> = {};
    for (const [code, ink] of Object.entries(inks)) (bySuit[suitOf(code)] ??= new Set()).add(ink);
    // A thirteen-card hand from two decks holds a diamond or a club in all
    // but a vanishing fraction of deals; hold the test to at least one.
    expect(Object.keys(bySuit).some((s) => s === 'D' || s === 'C'), JSON.stringify(inks)).toBe(true);
    if (bySuit.D) expect([...bySuit.D]).toEqual(['rgb(29, 78, 216)']); // classic fourColour.diamonds #1d4ed8
    if (bySuit.C) expect([...bySuit.C]).toEqual(['rgb(21, 128, 61)']); // classic fourColour.clubs #15803d
    if (bySuit.H) expect([...bySuit.H]).toEqual(['rgb(220, 38, 38)']); // classic card.red
    if (bySuit.S) expect([...bySuit.S]).toEqual(['rgb(30, 41, 59)']); // classic card.ink
  });

  test('is off by default: diamonds share the hearts red', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, {}, 'classic');
    await dealZolik(page);
    const inks = await inkByCode(page);
    for (const [code, ink] of Object.entries(inks)) {
      const s = suitOf(code);
      if (s === 'D' || s === 'H') expect(ink, code).toBe('rgb(220, 38, 38)');
      if (s === 'C' || s === 'S') expect(ink, code).toBe('rgb(30, 41, 59)');
    }
  });
});

test.describe('high contrast', () => {
  /** What the board says it is wearing, and the card stock it is drawn on. */
  async function look(page: Page) {
    const toggle = page.getByTestId('skin-toggle');
    await expect(toggle).toBeVisible({ timeout: 30_000 });
    return {
      skin: ((await toggle.textContent()) ?? '').replace('◈', '').trim(),
      stock: await page
        .locator('[data-testid^="card-hand:"]')
        .first()
        .evaluate((ring) => {
          // The card's own box: the first child of the ring with a background.
          for (const el of ring.querySelectorAll('div')) {
            const bg = getComputedStyle(el).backgroundColor;
            if (bg && bg !== 'rgba(0, 0, 0, 0)') return bg;
          }
          return '';
        }),
    };
  }

  test('goes on when asked for in Settings', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { contrast: 'on' }, 'casino');
    await dealZolik(page);
    await expect.poll(async () => (await look(page)).skin).toBe('High contrast');
    expect((await look(page)).stock).toBe('rgb(255, 255, 255)');
  });

  test('follows a browser that asks for more contrast when left on Automatic', async ({ page, request }) => {
    await page.emulateMedia({ contrast: 'more' });
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { contrast: 'auto' }, 'casino');
    await dealZolik(page);
    await expect.poll(async () => (await look(page)).skin).toBe('High contrast');
  });

  test('follows forced colours when left on Automatic', async ({ page, request }) => {
    await page.emulateMedia({ forcedColors: 'active' });
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { contrast: 'auto' }, 'casino');
    await dealZolik(page);
    await expect.poll(async () => (await look(page)).skin).toBe('High contrast');
  });

  test('stays off when turned off, whatever the browser asks', async ({ page, request }) => {
    await page.emulateMedia({ contrast: 'more' });
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { contrast: 'off' }, 'casino');
    await dealZolik(page);
    await page.waitForTimeout(500);
    expect((await look(page)).skin).toBe('Casino');
  });

  test('gives the chosen look back when it is turned off', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { contrast: 'on' }, 'casino');
    await dealZolik(page);
    await expect.poll(async () => (await look(page)).skin).toBe('High contrast');
    // Turn it off the way a player does, from Settings, and come back.
    const url = page.url();
    await page.goto('/settings');
    await page.getByTestId('a11y-contrast-off').click();
    await page.goto(url);
    await expect.poll(async () => (await look(page)).skin).toBe('Casino');
  });
});

test.describe('card size', () => {
  async function handCardHeight(page: Page, cardSize: 'normal' | 'large' | 'xlarge', request: Parameters<typeof loginAsFreshGuest>[1]) {
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { cardSize }, 'classic');
    await dealZolik(page);
    const box = await page.locator('[data-testid^="card-hand:"]').first().boundingBox();
    expect(box).not.toBeNull();
    return box!.height;
  }

  test('Large and Extra large draw larger cards than Normal', async ({ browser, request }) => {
    const heights: Record<string, number> = {};
    for (const size of ['normal', 'large', 'xlarge'] as const) {
      const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, reducedMotion: 'reduce' });
      const page = await context.newPage();
      heights[size] = await handCardHeight(page, size, request);
      await context.close();
    }
    expect(heights.large! / heights.normal!, JSON.stringify(heights)).toBeGreaterThan(1.1);
    expect(heights.xlarge! / heights.normal!, JSON.stringify(heights)).toBeGreaterThan(1.22);
  });
});

test.describe('reduced motion, asked for in the app', () => {
  // A browser that has *not* asked for stillness — so whatever stillness
  // there is comes from the in-app setting alone.
  test.use({ contextOptions: { reducedMotion: 'no-preference' } });

  /**
   * Plays a while and reports whether a card was ever put in the air: a
   * MutationObserver installed before the app loads notes the flight layer
   * the moment it exists, however briefly.
   */
  async function everFlew(page: Page, motion: 'on' | 'off', request: Parameters<typeof loginAsFreshGuest>[1]) {
    await page.addInitScript(() => {
      (window as unknown as { __flew: boolean }).__flew = false;
      new MutationObserver(() => {
        if (document.querySelector('[data-testid="flight-layer"]')) (window as unknown as { __flew: boolean }).__flew = true;
      }).observe(document, { childList: true, subtree: true });
    });
    await loginAsFreshGuest(page, request, fresh());
    await seed(page, { motion });
    await openGame(page, 'prsi');
    await playAgainstBots(page, 'prsi');
    await expect(page.locator('[data-testid^="card-hand:"]').first()).toBeVisible({ timeout: 30_000 });
    // Long enough for the bots to have played several cards between them.
    const deadline = Date.now() + 12_000;
    while (Date.now() < deadline) {
      if (await page.evaluate(() => (window as unknown as { __flew: boolean }).__flew)) return true;
      await page.waitForTimeout(250);
    }
    return page.evaluate(() => (window as unknown as { __flew: boolean }).__flew);
  }

  test('with motion left on, cards do fly (so the check below can see one)', async ({ page, request }) => {
    test.setTimeout(60_000);
    expect(await everFlew(page, 'off', request)).toBe(true);
  });

  test('with Reduce motion on, no card is ever put in the air', async ({ page, request }) => {
    test.setTimeout(60_000);
    expect(await everFlew(page, 'on', request)).toBe(false);
  });
});
