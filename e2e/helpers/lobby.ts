import { expect, type Page } from '@playwright/test';

/**
 * Open one game's setup page — what pressing that game's button on the list
 * (or the intro) leads to. `/lobby/games` itself is only the list of buttons.
 */
export async function openGame(page: Page, moduleId: string) {
  await page.goto(`/lobby/games?moduleId=${encodeURIComponent(moduleId)}`);
  await expect(page.getByTestId(`module-${moduleId}`)).toBeVisible({ timeout: 30_000 });
}

/**
 * Go from a game's page to its settings, the way a player does: "Play against
 * bots" (the default) or "Open a table". Every setting is on that screen, open;
 * the button that starts the game is `deal-me-in-<id>` or `open-table-<id>` —
 * see `playAgainstBots` and `openTableFor`.
 *
 * Idempotent: already on the settings screen, it stays there.
 */
export async function openGameSetup(page: Page, moduleId: string, mode: 'bots' | 'table' = 'bots') {
  const start = page.getByTestId(mode === 'bots' ? `deal-me-in-${moduleId}` : `open-table-${moduleId}`);
  if (await start.isVisible()) return;
  await page.getByTestId(mode === 'bots' ? `play-bots-${moduleId}` : `play-friends-${moduleId}`).click();
  await expect(start).toBeVisible({ timeout: 30_000 });
}

/** From a game's page: its settings as they stand, then deal against bots. */
export async function playAgainstBots(page: Page, moduleId: string) {
  await openGameSetup(page, moduleId, 'bots');
  await page.getByTestId(`deal-me-in-${moduleId}`).click();
}

/** From a game's page: its settings as they stand, then open a table. */
export async function openTableFor(page: Page, moduleId: string) {
  await openGameSetup(page, moduleId, 'table');
  await page.getByTestId(`open-table-${moduleId}`).click();
}
