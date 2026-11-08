import { type APIRequestContext, expect, type Page, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { openGame, openGameSetup } from '../helpers/lobby';
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

/** How many people and bots the table on screen really seated. */
async function seatsAtThisTable(page: Page, request: APIRequestContext): Promise<number> {
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  const matchId = new URL(page.url()).pathname.split('/').pop();
  const res = await request.get(`${API_BASE}/matches/${matchId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return ((await res.json()) as { players: unknown[] }).players.length;
}

test('Play now seats the game’s usual table, not its smallest', async ({ page, request }) => {
  await loginAsFreshGuest(page, request, `e2e-usual-${Math.random().toString(36).slice(2, 8)}`);
  await openGame(page, 'canasta');
  await page.getByTestId('play-bots-canasta').click();
  expect(await seatsAtThisTable(page, request)).toBe(4);
});

test('A bot count the player picked outlasts the default', async ({ page, request }) => {
  test.setTimeout(120_000);
  await loginAsFreshGuest(page, request, `e2e-picked-${Math.random().toString(36).slice(2, 8)}`);
  await openGame(page, 'canasta');
  await openGameSetup(page, 'canasta');
  // The default is what the picker shows before anything is touched.
  await expect(page.getByTestId('bots-canasta-3')).toBeChecked();
  await page.getByTestId('bots-canasta-1').click();
  await page.getByTestId('deal-me-in-canasta').click();
  expect(await seatsAtThisTable(page, request)).toBe(2);

  // And Play now, which asks nothing, deals what was picked last time.
  await openGame(page, 'canasta');
  await page.getByTestId('play-bots-canasta').click();
  expect(await seatsAtThisTable(page, request)).toBe(2);
});
