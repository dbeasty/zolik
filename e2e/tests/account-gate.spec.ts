import { expect, test } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';

/**
 * The score table and the stats screen are kept against an account, so they
 * are offered only to one.
 *
 * Both halves of that are tested, because only having the first is the bug
 * this guards: the menu draws the entries disabled, *and* the screens behind
 * them refuse on their own. A gate that exists only on the button in front
 * of it is walked around by typing the address — and by anyone who had the
 * screen open when they signed out.
 *
 * A guest is deliberately not enough. What these screens keep is kept with
 * an account, and a guest has nowhere for it to be kept.
 */

const suffix = () => Math.random().toString(36).slice(2, 8);

test.describe('the score table and stats need an account', () => {
  test('a guest is shown them disabled, and cannot reach them by address either', async ({
    page,
    request,
  }) => {
    await loginAsFreshGuest(page, request, `e2e-gate-${suffix()}`);

    await page.goto('/more');
    for (const id of ['more-score-table', 'more-stats']) {
      const entry = page.getByTestId(id);
      // Shown rather than hidden: a player who cannot find a feature
      // concludes it does not exist.
      await expect(entry).toBeVisible();
      await expect(entry).toHaveAttribute('aria-disabled', 'true');
      // And told why, in the same place.
      await expect(page.getByTestId(`${id}-hint`)).toHaveText('(sign in to use)');
    }

    // The screens themselves, reached the way the disabled button cannot
    // take you.
    for (const path of ['/stats', '/scoring']) {
      await page.goto(path);
      await expect(page.getByTestId('sign-in-required')).toBeVisible({ timeout: 10_000 });
    }
  });

  test('a registered account is let through to both', async ({ page }) => {
    const username = `e2e-gateok-${suffix()}`;

    // Registered through the UI rather than seeded, because the thing under
    // test is exactly the `isGuest` flag the real sign-in path produces.
    await page.goto('/auth/register');
    await page.getByPlaceholder('Username').fill(username);
    await page.getByPlaceholder('Password').fill('correct horse battery staple');
    await page.getByText('Register', { exact: true }).click();
    await expect(page).toHaveURL('/', { timeout: 10_000 });

    await page.goto('/more');
    const stats = page.getByTestId('more-stats');
    await expect(stats).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('more-stats-hint')).toHaveCount(0);

    await stats.click();
    await expect(page).toHaveURL(/\/stats/, { timeout: 10_000 });
    await expect(page.getByTestId('sign-in-required')).toHaveCount(0);
    // The screen's own content, not just the absence of the gate.
    await expect(page.getByText('Your stats', { exact: true })).toBeVisible({ timeout: 10_000 });

    await page.goto('/scoring');
    await expect(page.getByTestId('sign-in-required')).toHaveCount(0);
    await expect(page.getByText('New session', { exact: true })).toBeVisible({ timeout: 10_000 });
  });
});
