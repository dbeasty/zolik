import { expect, test } from '@playwright/test';

import { waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

/**
 * The browser end of Phase 2.2 (docs/extensibility-plan.md).
 *
 * The claim being tested is narrow and important: the *server* never sends a
 * rendered sentence, so a whole second language is a client-only change. If
 * any English survives a switch to Czech in the places the engine drives, some
 * server response is carrying prose it should not be.
 */

async function openRules(page: import('@playwright/test').Page) {
  await page.getByText(/rules/i).first().click();
  await expect(page.getByTestId('locale-cs')).toBeVisible();
}

test.describe('locale', () => {
  test('switching to Czech translates the engine-driven text', async ({ page, request }) => {
    const game = await seedGame(
      request,
      { hand: ['KS', 'KD', 'KH', '4C'], phase: 'draw', discardPile: ['9H'] },
      { rulesProfile: 'continental' },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // The discard pile is locked under continental in round 1, and the reason
    // shown is the engine's DISCARD_LOCKED code rendered by the bundle.
    await expect(page.getByText('The discard pile is locked for now')).toBeVisible();

    await openRules(page);
    await page.getByTestId('locale-cs').click();

    // Same code, same server response, Czech words.
    await expect(page.getByText('Odhazovací balíček je zatím zamčený')).toBeVisible();
    await expect(page.getByText('The discard pile is locked for now')).toHaveCount(0);
  });

  test('the rules panel and contract translate, including the count phrase', async ({
    page,
    request,
  }) => {
    // Continental's first deal asks for two sets. English pluralises with an
    // "s"; Czech inflects the noun ("Dvě skupiny"). Getting this right is the
    // reason the bundle owns whole phrases per count rather than gluing a
    // number to a noun.
    const game = await seedGame(
      request,
      { hand: ['KS', 'KD', 'KH', '4C'], phase: 'meld' },
      { rulesProfile: 'continental' },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Before the panel is opened the contract only exists in the header line,
    // so match that whole line rather than the phrase on its own.
    await expect(page.getByText(/Game 1 of 7: Two sets/)).toBeVisible();

    await openRules(page);
    await page.getByTestId('locale-cs').click();

    await expect(page.getByText('Varianta')).toBeVisible();
    await expect(page.getByText('Dvě skupiny', { exact: true })).toBeVisible();
    await expect(page.getByText('12 karet')).toBeVisible();
    // The deal header too — a param-carrying key, not a fixed string.
    await expect(page.getByText(/Hra 1 ze 7/)).toBeVisible();
  });

  test('the choice survives a reload', async ({ page, request }) => {
    const game = await seedGame(
      request,
      { hand: ['KS', 'KD', 'KH', '4C'], phase: 'meld' },
      { rulesProfile: 'continental' },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await openRules(page);
    await page.getByTestId('locale-cs').click();
    await expect(page.getByText('Varianta')).toBeVisible();

    await page.reload();
    await waitForGameLoaded(page);
    // Restored before first paint, so the player never sees English flash by.
    await expect(page.getByText(/Hra 1 ze 7/)).toBeVisible();
  });

  test('switching back to English restores it', async ({ page, request }) => {
    const game = await seedGame(
      request,
      { hand: ['KS', 'KD', 'KH', '4C'], phase: 'meld' },
      { rulesProfile: 'continental' },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await openRules(page);
    await page.getByTestId('locale-cs').click();
    await expect(page.getByText('Varianta')).toBeVisible();

    await page.getByTestId('locale-en').click();
    await expect(page.getByText('Variation')).toBeVisible();
    await expect(page.getByText('Varianta')).toHaveCount(0);
  });
});
