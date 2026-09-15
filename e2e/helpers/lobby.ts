import { expect, type Page } from '@playwright/test';

/**
 * Open a game card's setup on the picker.
 *
 * Cards on `/lobby/games` arrive closed — a name, the table size and a digest
 * of what the game is set to — so the variations, rule facts, options and bot
 * counts are not in the DOM until "Nastavení" is pressed. The controls behind
 * it are the same controls they always were, so a spec touches them exactly as
 * it used to; it just has to ask for them first, as a player does.
 *
 * Idempotent: a card already open is left open.
 */
export async function openGameSetup(page: Page, moduleId: string) {
  const toggle = page.getByTestId(`setup-toggle-${moduleId}`);
  await expect(toggle).toBeVisible({ timeout: 30_000 });
  if ((await toggle.getAttribute('aria-expanded')) === 'true') return;
  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
}
