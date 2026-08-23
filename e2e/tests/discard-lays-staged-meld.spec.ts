import { expect, test } from '@playwright/test';

import { dragLocatorTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

/**
 * A discard ends the turn, so a meld still sitting in the staging area when
 * one happens was never going to be laid. Dropping the discard lays it
 * first — and if the server refuses it, the discard doesn't happen either:
 * a rejected meld must cost the player nothing, not their whole turn.
 */

async function stage(page: import('@playwright/test').Page, times: number) {
  const staging = page.getByTestId('staging-zone');
  for (let i = 0; i < times; i++) {
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await page.waitForTimeout(200);
  }
}

test.describe('discarding with a meld staged', () => {
  test('lays the staged meld, then discards', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['KS', 'KD', 'KH', '4C', '7D', '9S'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Three kings staged but never laid — then 4C goes on the discard pile.
    await stage(page, 3);
    // Enabled, and visibly so — the lighter blue, not the dimmed resting one.
    await expect(page.getByTestId('lay-all-button')).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('lay-all-button')).toHaveCSS('opacity', '1');
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), page.getByTestId('discard-zone'));

    // The kings landed on the table (rather than being swept back into the
    // hand), and the 4C landed on the pile.
    await expect(page.getByText('TABLE MELDS')).toBeVisible();
    await expect(page.getByTestId('discard-top-card')).toContainText('4');
    await expect(page.getByText('Your hand (2)')).toBeVisible();
  });

  test('refuses the discard and keeps the staging when the meld is illegal', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['KS', '4C', '7D', '9S', 'TH', 'JC'],
      phase: 'meld',
      // Pinned so "the pile did not change" is an assertion about the
      // discard that never happened, not about whatever the deal dealt.
      discardPile: ['8C'],
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // KS + 4C + 7D is neither a set nor a run.
    await stage(page, 3);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), page.getByTestId('discard-zone'));

    await expect(page.getByText(/nothing was discarded/i)).toBeVisible();
    await expect(page.getByText('TABLE MELDS')).toHaveCount(0);
    // Still staged, still holding all six cards (the count covers staged
    // ones too), and the pile still shows the card it was seeded with.
    await expect(page.getByTestId('staged-card-0-0')).toBeVisible();
    await expect(page.getByTestId('staged-card-0-2')).toBeVisible();
    await expect(page.getByText('Your hand (6)')).toBeVisible();
    await expect(page.getByTestId('discard-top-card')).toContainText('8');
  });

  test('discards normally when nothing is staged', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['KS', '4C', '7D', '9S'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await dragLocatorTo(page, page.getByTestId('hand-card-0'), page.getByTestId('discard-zone'));

    await expect(page.getByTestId('discard-top-card')).toContainText('K');
    await expect(page.getByText('Your hand (3)')).toBeVisible();
  });
});
