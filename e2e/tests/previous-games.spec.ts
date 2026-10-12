import { expect, test } from '@playwright/test';

import { openGame } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

// A game's "Previous games" button opens the stored-games list on the tables
// still in progress: those are the ones a player comes back to pick up.
// Finished is the other tab, one tap away.
test('Previous games opens on the games in progress', async ({ page, request }) => {
  await loginAsFreshGuest(page, request, 'Previous');
  await openGame(page, 'prsi');

  await page.getByTestId('previous-games-prsi').click();

  await expect(page).toHaveURL(/\/lobby\/mine\?moduleId=prsi$/);
  await expect(page.getByTestId('mine-tab-unfinished')).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByTestId('mine-tab-finished')).toHaveAttribute('aria-selected', 'false');
  await expect(page.getByText('Nothing in progress right now')).toBeVisible();
});
