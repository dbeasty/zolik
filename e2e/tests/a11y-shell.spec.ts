import { expect, test, type Page } from '@playwright/test';

import { loginAsFreshGuest, seedIntroSeen } from '../helpers/login';

/**
 * The app shell as a keyboard and a screen reader meet it
 * (docs/accessibility-plan.md, Phase 1).
 *
 * Every locator here is by role and accessible name, not by testID, so each
 * assertion is also an assertion that the name exists and says the right
 * thing — a control a screen reader cannot name is a control these tests
 * cannot find. Keys are pressed, never clicked, wherever the point is that a
 * keyboard can do it.
 */

/** Presses Tab until `target` has focus, failing after `max` presses. */
async function tabTo(page: Page, target: ReturnType<Page['getByRole']>, max = 40) {
  for (let i = 0; i < max; i++) {
    await page.keyboard.press('Tab');
    if (await target.evaluate((el) => el === document.activeElement).catch(() => false)) return;
  }
  throw new Error(`Tab never reached ${target}`);
}

test.describe('the app shell with a keyboard and a screen reader', () => {
  test.beforeEach(async ({ page, request }) => {
    await loginAsFreshGuest(page, request, `a11y-${Math.random().toString(36).slice(2, 8)}`);
  });

  test('each screen names itself in the browser tab', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle('Jokerless');
    await page.goto('/settings');
    await expect(page).toHaveTitle('Settings · Jokerless');
    await page.goto('/rules?moduleId=zolik');
    await expect(page).toHaveTitle('Rules · Jokerless');
    await page.goto('/legal/accessibility');
    await expect(page).toHaveTitle('Accessibility · Jokerless');
  });

  test('Tab reaches the account menu, which opens as a named dialog and closes on Escape', async ({ page }) => {
    await page.goto('/settings');
    await expect(page.getByRole('heading', { name: 'Accessibility', level: 3 })).toBeVisible();

    const face = page.getByRole('button', { name: /^Account menu: / });
    await tabTo(page, face);
    // The name carries who and whether signed in — the dot on the face.
    await expect(face).toHaveAccessibleName(/Guest/);
    await expect(face).toHaveAttribute('aria-expanded', 'false');

    // Keyboard focus shows the tooltip; Escape puts it away (WCAG 1.4.13).
    const tip = page.getByTestId('tooltip');
    await expect(tip).toHaveText('Your account, settings and games');
    await page.keyboard.press('Escape');
    await expect(tip).toHaveCount(0);

    await page.keyboard.press('Enter');
    const menu = page.getByRole('dialog', { name: 'Account menu' });
    await expect(menu).toBeVisible();
    // Focus moved into the menu, onto its first item — not left on the page
    // behind it, and not on the backdrop.
    await expect(menu.getByRole('button', { name: 'My games' })).toBeFocused();
    await expect(menu.getByRole('button', { name: 'Settings' })).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(menu).toHaveCount(0);
    // And back where it came from.
    await expect(face).toBeFocused();
  });

  test('the accessibility settings are radios and switches a keyboard can work, and they stick', async ({
    page,
  }) => {
    await page.goto('/settings');
    const size = page.getByRole('radiogroup', { name: 'Card size' });
    await expect(size).toBeVisible();
    const normal = size.getByRole('radio', { name: 'Normal' });
    const large = size.getByRole('radio', { name: 'Large', exact: true });
    await expect(normal).toHaveAttribute('aria-checked', 'true');

    await tabTo(page, normal, 120);
    // Arrow keys move along the group and pick, as in any radio group.
    await page.keyboard.press('ArrowRight');
    await expect(large).toBeFocused();
    await expect(large).toHaveAttribute('aria-checked', 'true');
    await expect(normal).toHaveAttribute('aria-checked', 'false');

    // Space turns a switch off.
    const tooltips = page.getByRole('switch', { name: 'Tooltips' });
    await expect(tooltips).toBeChecked();
    await tooltips.focus();
    await page.keyboard.press('Space');
    await expect(tooltips).not.toBeChecked();

    await page.reload();
    await expect(page.getByRole('radiogroup', { name: 'Card size' }).getByRole('radio', { name: 'Large', exact: true })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    await expect(page.getByRole('switch', { name: 'Tooltips' })).not.toBeChecked();
  });

  test('every face in the avatar picker says whether it is the one chosen', async ({ page }) => {
    await page.goto('/settings');
    const faces = page.getByRole('radiogroup', { name: 'Your face at the table' });
    const radios = faces.getByRole('radio');
    expect(await radios.count()).toBeGreaterThan(1);
    for (const radio of await radios.all()) {
      await expect(radio).toHaveAttribute('aria-checked', /^(true|false)$/);
    }
    await radios.nth(1).focus();
    await page.keyboard.press('Enter');
    await expect(radios.nth(1)).toHaveAttribute('aria-checked', 'true');
  });

  test('the rules are headings a screen reader can jump between', async ({ page }) => {
    await page.goto('/rules?moduleId=zolik');
    await expect(page.getByRole('heading', { name: 'Rules', level: 2 })).toBeVisible();
    expect(await page.getByRole('heading', { level: 3 }).count()).toBeGreaterThan(2);
  });

  test('a join code left empty is refused out loud and marked on the box', async ({ page }) => {
    await page.goto('/lobby/join');
    const box = page.getByRole('textbox', { name: 'Join code or invite link' });
    await expect(box).toBeVisible();
    await box.focus();
    await page.keyboard.press('Enter');
    await expect(box).toHaveAttribute('aria-invalid', 'true');
    await expect(box).toHaveAccessibleDescription(/Enter a join code/);
    // An alert, so a screen reader speaks it the moment it appears.
    await expect(page.getByRole('alert').filter({ hasText: 'Enter a join code' })).toBeVisible();
  });

  test('the accessibility statement is reachable from About by keyboard', async ({ page }) => {
    await page.goto('/about');
    const link = page.getByRole('link', { name: /Accessibility statement/ });
    await tabTo(page, link);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/legal\/accessibility$/);
    await expect(page.getByRole('heading', { name: 'What works today' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Known limitations' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Report a problem' })).toBeVisible();
    await expect(page.getByText(/aims to meet the Web Content Accessibility Guidelines/)).toBeVisible();
  });
});

test.describe('signed out', () => {
  test('the notices links include the accessibility statement', async ({ page }) => {
    await seedIntroSeen(page);
    await page.goto('/');
    const notices = page.getByRole('navigation', { name: 'Notices' }).first();
    await notices.getByRole('link', { name: 'Accessibility' }).click();
    await expect(page.getByTestId('a11y-statement')).toBeVisible();
  });

  test('sign-in fields are labelled, not just placeheld', async ({ page }) => {
    await seedIntroSeen(page);
    await page.goto('/auth/username-login');
    await expect(page.getByRole('textbox', { name: 'Username' })).toBeVisible();
    // A password box has no textbox role; its label still names it.
    await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
    await page.goto('/auth/guest');
    await expect(page.getByRole('textbox', { name: 'Display name' })).toBeVisible();
  });
});
