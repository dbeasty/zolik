import { expect, type Page } from '@playwright/test';

/** Blocks until an offer control is live — i.e. it is this viewer's turn. */
export async function waitForOfferEnabled(page: Page, testId: string, timeout = 30_000) {
  const offer = page.getByTestId(testId);
  await expect(offer).toBeVisible({ timeout });
  await expect(offer).not.toHaveAttribute('aria-disabled', 'true', { timeout });
}
