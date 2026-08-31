import { expect, test } from '@playwright/test';

/**
 * The notices are only worth anything if a player can actually reach them, so
 * every assertion here is about what is on the screen after a real click —
 * never about the URL. A link that navigates to a route rendering nothing
 * would satisfy a routing test and fail the only reader who ever follows it.
 */
test.describe('the legal notices are reachable and readable', () => {
  test('the footer reaches the terms, and the terms say the game is free of charge and of warranty', async ({
    page,
  }) => {
    await page.goto('/');

    await page.getByTestId('legal-link-terms').click();

    const doc = page.getByTestId('legal-terms');
    await expect(doc).toBeVisible();
    // The two clauses this document exists for: no warranty, and no liability.
    await expect(doc).toContainText('The game is provided as is');
    await expect(doc).toContainText('Liability');
    // And the clause a card game specifically needs, given it deals poker.
    await expect(doc).toContainText('No real money');
    await expect(page.getByTestId('legal-terms-version')).toContainText('Version');
  });

  test('the footer reaches the privacy notice, and it claims no tracking', async ({ page }) => {
    await page.goto('/');

    await page.getByTestId('legal-link-privacy').click();

    const doc = page.getByTestId('legal-privacy');
    await expect(doc).toBeVisible();
    await expect(doc).toContainText('No analytics, no ads, no tracking');
    await expect(doc).toContainText('What we store, and why');
    // The claim that is checked against the code — see `privacy.en.ts`. If a
    // sign-in code's retention changes and this sentence does not, the notice
    // has become untrue, and that is worth a red test.
    await expect(doc).toContainText('ten minutes after it is sent');
  });

  test('a guest is told what they are agreeing to before they can agree to it', async ({ page }) => {
    await page.goto('/');
    await page.getByText('Continue as guest').click();

    const notice = page.getByTestId('legal-notice');
    await expect(notice).toBeVisible();
    await expect(notice).toContainText('By playing you agree to the');

    // The link inside the sentence goes to the document, from the screen a
    // guest actually starts on.
    await page.getByTestId('legal-notice-terms').click();
    await expect(page.getByTestId('legal-terms')).toBeVisible();
  });

  test('the draft banner is shown while the operator is unnamed', async ({ page }) => {
    // Guarding the honest failure mode rather than the happy one: as long as
    // `OPERATOR` is placeholders, every reader must be told the document is
    // not in force. When the operator is filled in this test is what tells
    // you to delete it.
    await page.goto('/legal/terms');
    await expect(page.getByTestId('legal-terms-draft')).toBeVisible();
  });
});
