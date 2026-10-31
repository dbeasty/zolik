import { expect, test } from '@playwright/test';

import { openGame } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * Play now is the quickest way into a game: one press from the game's page to
 * cards in hand, with bots in the other seats and no settings screen between.
 */
test('Play now deals a game against bots straight away', async ({ page, request }) => {
  await loginAsFreshGuest(page, request, `e2e-playnow-${Math.random().toString(36).slice(2, 8)}`);
  await openGame(page, 'prsi');
  await page.getByTestId('play-bots-prsi').click();
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId('deal-me-in-prsi')).toHaveCount(0);
});
