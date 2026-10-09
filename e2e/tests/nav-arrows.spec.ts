import { expect, test, type Page } from '@playwright/test';

import { WEB_BASE } from '../helpers/env';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * The header's back and forward arrows. Going back always has a way forward:
 * a player who goes back to the main screen has → right there to return to
 * where they were, and following anything new clears it, as a browser does.
 */

const arrow = (page: Page, id: 'nav-back' | 'nav-forward') =>
  page.getByTestId(id).filter({ visible: true }).first();

async function onPath(page: Page, re: RegExp) {
  await page.waitForFunction((src) => new RegExp(src).test(location.pathname), re.source, { timeout: 20_000 });
}

test('back to the main screen, then forward to where the player was', async ({ page, request }) => {
  await loginAsFreshGuest(page, request, 'Arrow Tester');
  await page.goto(`${WEB_BASE}/`);

  // Nowhere to go either way on arrival.
  await expect(arrow(page, 'nav-back')).toHaveAttribute('aria-disabled', 'true');
  await expect(arrow(page, 'nav-forward')).toHaveAttribute('aria-disabled', 'true');

  await page.getByText('Last Card', { exact: true }).first().click();
  await onPath(page, /^\/lobby\/games$/);
  await expect(arrow(page, 'nav-back')).not.toHaveAttribute('aria-disabled', 'true');

  await arrow(page, 'nav-back').click();
  await onPath(page, /^\/$/);
  // Home is where Back stops, even with history still behind it.
  await expect(arrow(page, 'nav-back')).toHaveAttribute('aria-disabled', 'true');
  await expect(arrow(page, 'nav-forward')).not.toHaveAttribute('aria-disabled', 'true');

  await arrow(page, 'nav-forward').click();
  await onPath(page, /^\/lobby\/games$/);
  expect(new URL(page.url()).searchParams.get('moduleId')).toBe('lastcard');
  await expect(arrow(page, 'nav-forward')).toHaveAttribute('aria-disabled', 'true');
});

test('following something new forgets what was ahead', async ({ page, request }) => {
  await loginAsFreshGuest(page, request, 'Arrow Tester');
  await page.goto(`${WEB_BASE}/`);

  await page.getByText('Last Card', { exact: true }).first().click();
  await onPath(page, /^\/lobby\/games$/);
  await arrow(page, 'nav-back').click();
  await onPath(page, /^\/$/);
  await expect(arrow(page, 'nav-forward')).not.toHaveAttribute('aria-disabled', 'true');

  await page.getByText('Canasta', { exact: true }).first().click();
  await onPath(page, /^\/lobby\/games$/);
  await expect(arrow(page, 'nav-forward')).toHaveAttribute('aria-disabled', 'true');
});
