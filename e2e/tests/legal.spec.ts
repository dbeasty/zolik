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

  test('the terms offer the source, which is what the AGPL asks of a network deployment', async ({
    page,
  }) => {
    await page.goto('/');

    // The footer's third link is the one-click form of the offer. It is not
    // clicked here: it leaves for GitHub, and a suite that reaches the public
    // internet to prove a licence notice exists fails for reasons that have
    // nothing to do with the licence notice.
    await expect(page.getByTestId('legal-link-source')).toBeVisible();

    await page.getByTestId('legal-link-terms').click();

    const doc = page.getByTestId('legal-terms');
    // Named licence and a reachable address, on screen, in the document the
    // player is shown — section 13 is satisfied by what a network user can
    // read, not by the LICENSE file sitting in the repository.
    await expect(doc).toContainText('GNU Affero General Public License');
    await expect(doc).toContainText('https://github.com/dbeasty/zolik');
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
    // Guarding the honest failure mode rather than the happy one: a build
    // given no operator must tell every reader the document is not in force.
    // A dev-stack build is given none — `ZOLIK_OPERATOR*` is set only by
    // scripts/deploy.sh — so this is the state the suite runs against. Once a
    // deploy supplies all three, this test is what tells you to delete it.
    await page.goto('/legal/terms');
    await expect(page.getByTestId('legal-terms-draft')).toBeVisible();
  });
});
