import { expect, test } from '@playwright/test';

import { seedIntroSeen } from '../helpers/login';

/**
 * Refusals on the register screen are worded in the player's language: a taken
 * username, an empty form and a password nobody could hash all read as
 * sentences, not as the server's English phrase.
 */

const suffix = () => Math.random().toString(36).slice(2, 8);

test.beforeEach(async ({ page }) => {
  await seedIntroSeen(page);
});

async function register(page: import('@playwright/test').Page, username: string, password: string) {
  await page.goto('/auth/register');
  await page.getByPlaceholder('Username').fill(username);
  await page.getByPlaceholder('Password').fill(password);
  await page.getByRole('button', { name: 'Register' }).click();
}

test.describe('register refusals', () => {
  test('a username that is taken says so in words', async ({ page }) => {
    const name = `dupe${suffix()}`;
    await register(page, name, 'correct-horse-battery');
    await expect(page).toHaveURL(/\/($|\?)/);
    await page.context().clearCookies();
    await page.evaluate(() => window.localStorage.clear());
    await seedIntroSeen(page);
    await register(page, name, 'another-password-1');
    await expect(page.getByText('That username or email is already taken.')).toBeVisible();
    await expect(page.getByText(/duplicate key|E11000|kdb/i)).toHaveCount(0);
  });

  test('a password too long to hash says so in words', async ({ page }) => {
    await register(page, `long${suffix()}`, 'x'.repeat(2000));
    await expect(page.getByText('That password is too long.')).toBeVisible();
    await expect(page.getByText('password too long')).toHaveCount(0);
  });

  test('an empty password says what is missing', async ({ page }) => {
    await register(page, `empty${suffix()}`, '');
    await expect(page.getByText('Enter a username and a password.')).toBeVisible();
  });
});
