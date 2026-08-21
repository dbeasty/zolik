import { expect, test } from '@playwright/test';

import { dragLocatorTo, flickUp, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

test.describe('discard', () => {
  test('dragging a hand card onto the discard pile discards it', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['5H', '6H', '7H', '8H', '9C', '9D', '9S', 'TC', 'KD'],
      phase: 'discard',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await dragLocatorTo(page, page.getByTestId('hand-card-0'), page.getByTestId('discard-zone'));

    // hand-card-0 was 5H — confirm the hand shrank and 5H is now on top of
    // the discard pile.
    await expect(page.getByText('Your hand (8)')).toBeVisible();
    await expect(page.getByTestId('discard-top-card')).toContainText('5');
  });

  test('a quick upward flick discards without needing to reach the pile', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['5H', '6H', '7H', '8H', '9C', '9D', '9S', 'TC', 'KD'],
      phase: 'discard',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await flickUp(page, page.getByTestId('hand-card-0'));

    await expect(page.getByText('Your hand (8)')).toBeVisible();
    await expect(page.getByTestId('discard-top-card')).toContainText('5');
  });
});
