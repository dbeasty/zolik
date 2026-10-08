import { expect, test } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';

/**
 * The first-run intro is the game picker for somebody who has never been here:
 * every game is a button to that game's own setup, and "Join a table" is the
 * way in for a code heard out loud rather than a link followed.
 *
 * No session and no `seedIntroSeen` for the signed-out cases — a new visitor
 * is exactly who this screen is for.
 */
test.describe('the first-run intro', () => {
  test('lists every hosted game as a button', async ({ page }) => {
    await page.goto('/intro');
    for (const id of ['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy', 'rummytiles', 'blackjack', 'lastcard', 'ferbl', 'okobere', 'sedma', 'snaps']) {
      await expect(page.getByTestId(`game-${id}`)).toBeVisible({ timeout: 30_000 });
    }
    await expect(page.getByTestId('intro-join')).toBeVisible();
  });

  test('a game button signs a visitor in as a guest, then opens that game alone', async ({
    page,
  }) => {
    await page.goto('/intro');
    await page.getByTestId('game-canasta').click();

    await expect(page).toHaveURL(/\/auth\/guest/, { timeout: 10_000 });
    await page.getByText('Continue', { exact: true }).click();

    await expect(page).toHaveURL(/\/lobby\/games\?moduleId=canasta/, { timeout: 15_000 });
    await expect(page.getByTestId('module-canasta')).toBeVisible();
    await expect(page.getByTestId('play-bots-canasta')).toBeVisible();
    // One game, not the picker: no other game's page is showing.
    await expect(page.getByTestId('module-holdem')).toHaveCount(0);
  });

  test('"Join a table" signs a visitor in as a guest, then asks for the code', async ({ page }) => {
    await page.goto('/intro');
    await page.getByTestId('intro-join').click();

    await expect(page).toHaveURL(/\/auth\/guest/, { timeout: 10_000 });
    await page.getByText('Continue', { exact: true }).click();

    await expect(page).toHaveURL(/\/lobby\/join$/, { timeout: 15_000 });
    await expect(page.getByPlaceholder('Join code or invite link')).toBeVisible();
  });

  test('signed in, both go straight there', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, 'IntroHost');

    await page.goto('/intro');
    await page.getByTestId('game-prsi').click();
    await expect(page).toHaveURL(/\/lobby\/games\?moduleId=prsi/, { timeout: 15_000 });
    await expect(page.getByTestId('play-bots-prsi')).toBeVisible();

    await page.goto('/intro');
    await page.getByTestId('intro-join').click();
    await expect(page).toHaveURL(/\/lobby\/join$/, { timeout: 15_000 });
  });
});
